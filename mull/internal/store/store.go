// Package store keeps mull's records on a dedicated git branch that is never
// checked out. Writes go through git plumbing and a compare-and-swap on the
// branch ref, so they are atomic, never touch a working tree, and are shared
// by every worktree of the repository.
package store

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mull/internal/git"
)

// Ref is the branch mull's data lives on. It is a constant on purpose:
// nothing in mull reads or writes any other ref.
const Ref = "refs/heads/mull"

// maxAttempts bounds compare-and-swap retries when writers race.
const maxAttempts = 20

// ErrNotInitialized means the mull branch does not exist yet.
var ErrNotInitialized = errors.New("mull is not initialized in this repository (run: mull init)")

// ErrBusy means writers kept racing and the update gave up.
var ErrBusy = errors.New("could not write to the mull branch: too many concurrent writers; try again")

// ErrExists means init was run where the mull branch already exists.
var ErrExists = errors.New("mull is already initialized in this repository")

// Store reads and writes the mull branch of one repository.
type Store struct {
	git *git.Repo
}

// New returns a store for the repository r.
func New(r *git.Repo) *Store { return &Store{git: r} }

// Snapshot is the content of the mull branch at one commit. Once loaded it
// isn't changed, so many readers can share it.
type Snapshot struct {
	Commit string
	Files  map[string][]byte
	// Blobs are the files' blob IDs, when the snapshot was read from git (a
	// file's content is the same wherever its blob ID is).
	Blobs map[string]string

	sortOnce sync.Once
	sorted   []string // the paths of Files, sorted; see paths
}

// paths returns the snapshot's paths, sorted.
func (s *Snapshot) paths() []string {
	s.sortOnce.Do(func() {
		s.sorted = make([]string, 0, len(s.Files))
		for p := range s.Files {
			s.sorted = append(s.sorted, p)
		}
		sort.Strings(s.sorted)
	})
	return s.sorted
}

// Initialized reports whether the mull branch exists.
func (s *Store) Initialized() (bool, error) {
	c, err := s.git.RefCommit(Ref)
	return c != "", err
}

// Load reads the whole mull branch.
func (s *Store) Load() (*Snapshot, error) {
	commit, err := s.Head()
	if err != nil {
		return nil, err
	}
	return s.loadCommit(commit)
}

// Head returns the commit the mull branch points at.
func (s *Store) Head() (string, error) {
	commit, err := s.git.RefCommit(Ref)
	if err != nil {
		return "", err
	}
	if commit == "" {
		return "", ErrNotInitialized
	}
	return commit, nil
}

// ReadOnly loads the mull branch at commit into a transaction that refuses
// writes, for readers that keep it (a cache shared between requests).
// commit must be a commit in this repository. Given prev, another read-only
// transaction of this repository, it reads only the files that differ from
// prev's.
func (s *Store) ReadOnly(commit string, prev *Tx) (*Tx, error) {
	var snap *Snapshot
	var err error
	if prev != nil && prev.snap.Blobs != nil {
		snap, err = s.loadFrom(prev.snap, commit)
	} else {
		snap, err = s.loadCommit(commit)
	}
	if err != nil {
		return nil, err
	}
	tx := newTx(snap)
	tx.frozen = true
	return tx, nil
}

func (s *Store) loadCommit(commit string) (*Snapshot, error) {
	listing, err := s.git.Run(nil, nil, "ls-tree", "-r", "-z", commit)
	if err != nil {
		return nil, err
	}
	type entry struct{ sha, path string }
	var entries []entry
	for _, rec := range bytes.Split(listing, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		// "<mode> <type> <sha>\t<path>"
		meta, path, ok := strings.Cut(string(rec), "\t")
		if !ok {
			return nil, fmt.Errorf("unexpected ls-tree line %q", rec)
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		entries = append(entries, entry{sha: fields[2], path: path})
	}
	snap := &Snapshot{Commit: commit, Files: make(map[string][]byte, len(entries)), Blobs: make(map[string]string, len(entries))}
	shas := make([]string, len(entries))
	for i, e := range entries {
		shas[i] = e.sha
	}
	blobs, err := s.readBlobs(shas)
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		snap.Files[e.path], snap.Blobs[e.path] = blobs[i], e.sha
	}
	return snap, nil
}

// readBlobs reads blobs by ID, in one git call.
func (s *Store) readBlobs(shas []string) ([][]byte, error) {
	if len(shas) == 0 {
		return nil, nil
	}
	var req bytes.Buffer
	for _, sha := range shas {
		req.WriteString(sha)
		req.WriteByte('\n')
	}
	out, err := s.git.Run(nil, &req, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(bytes.NewReader(out))
	blobs := make([][]byte, len(shas))
	for i, sha := range shas {
		header, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading cat-file output: %w", err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected cat-file header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("unexpected cat-file size %q", header)
		}
		blobs[i] = make([]byte, size)
		if _, err := io.ReadFull(rd, blobs[i]); err != nil {
			return nil, fmt.Errorf("reading blob %s: %w", sha, err)
		}
		if _, err := rd.ReadByte(); err != nil { // trailing newline
			return nil, fmt.Errorf("reading blob %s: %w", sha, err)
		}
	}
	return blobs, nil
}

// loadFrom reads the mull branch at commit starting from prev, a snapshot
// read from git: only the files that differ between the two are read.
func (s *Store) loadFrom(prev *Snapshot, commit string) (*Snapshot, error) {
	out, err := s.git.Run(nil, nil, "diff-tree", "-r", "-z", "--no-renames", "--no-commit-id", prev.Commit, commit)
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{Commit: commit, Files: make(map[string][]byte, len(prev.Files)), Blobs: make(map[string]string, len(prev.Blobs))}
	for p, b := range prev.Files {
		snap.Files[p], snap.Blobs[p] = b, prev.Blobs[p]
	}
	var paths, shas []string
	recs := bytes.Split(out, []byte{0})
	for i := 0; i+1 < len(recs); i += 2 {
		// ":<old mode> <new mode> <old sha> <new sha> <status>" then the path
		fields := strings.Fields(string(recs[i]))
		if len(fields) != 5 {
			return nil, fmt.Errorf("unexpected diff-tree record %q", recs[i])
		}
		path := string(recs[i+1])
		if fields[4] == "D" {
			delete(snap.Files, path)
			delete(snap.Blobs, path)
			continue
		}
		if strings.HasPrefix(fields[1], "04") || strings.HasPrefix(fields[1], "16") { // not a blob
			continue
		}
		paths, shas = append(paths, path), append(shas, fields[3])
	}
	blobs, err := s.readBlobs(shas)
	if err != nil {
		return nil, err
	}
	for i, p := range paths {
		snap.Files[p], snap.Blobs[p] = blobs[i], shas[i]
	}
	return snap, nil
}

// Version is one version of a file on the mull branch: its content after a
// commit that changed it. Removed means the commit deleted it.
type Version struct {
	Commit  string
	At      time.Time // the commit's time
	Data    []byte
	Removed bool
}

// History returns every version of path on the mull branch, newest first:
// one per commit that changed it.
func (s *Store) History(path string) ([]Version, error) {
	commit, err := s.git.RefCommit(Ref)
	if err != nil {
		return nil, err
	}
	if commit == "" {
		return nil, ErrNotInitialized
	}
	out, err := s.git.Output(nil, nil, "log", "--format=%H %ct", "--no-renames", commit, "--", path)
	if err != nil {
		return nil, err
	}
	var versions []Version
	var req bytes.Buffer
	for _, line := range strings.Split(out, "\n") {
		sha, ts, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		secs, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected git log line %q", line)
		}
		versions = append(versions, Version{Commit: sha, At: time.Unix(secs, 0).UTC()})
		fmt.Fprintf(&req, "%s:%s\n", sha, path)
	}
	if len(versions) == 0 {
		return versions, nil
	}
	data, err := s.git.Run(nil, &req, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(bytes.NewReader(data))
	for i := range versions {
		header, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading cat-file output: %w", err)
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" { // the commit removed it
			versions[i].Removed = true
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected cat-file header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("unexpected cat-file size %q", header)
		}
		versions[i].Data = make([]byte, size)
		if _, err := io.ReadFull(rd, versions[i].Data); err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		if _, err := rd.ReadByte(); err != nil { // trailing newline
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
	}
	return versions, nil
}

// Tx is a set of changes against one snapshot.
type Tx struct {
	snap   *Snapshot
	puts   map[string][]byte
	dels   map[string]bool
	frozen bool // ReadOnly: shared between readers, so writing is a bug
}

// Get returns a file's content as seen by this transaction.
func (t *Tx) Get(path string) ([]byte, bool) {
	if t.dels[path] {
		return nil, false
	}
	if b, ok := t.puts[path]; ok {
		return b, true
	}
	b, ok := t.snap.Files[path]
	return b, ok
}

// Put writes a file.
func (t *Tx) Put(path string, data []byte) {
	t.mustWrite()
	delete(t.dels, path)
	t.puts[path] = data
}

// Delete removes a file.
func (t *Tx) Delete(path string) {
	t.mustWrite()
	delete(t.puts, path)
	t.dels[path] = true
}

func (t *Tx) mustWrite() {
	if t.frozen {
		panic("store: write to a read-only transaction")
	}
}

// List returns the sorted paths under prefix as seen by this transaction.
func (t *Tx) List(prefix string) []string {
	if len(t.puts) == 0 && len(t.dels) == 0 {
		// Nothing written: the snapshot's sorted paths, by binary search.
		paths := t.snap.paths()
		lo := sort.SearchStrings(paths, prefix)
		hi := lo
		for hi < len(paths) && strings.HasPrefix(paths[hi], prefix) {
			hi++
		}
		return slices.Clone(paths[lo:hi])
	}
	seen := map[string]bool{}
	for p := range t.snap.Files {
		if strings.HasPrefix(p, prefix) && !t.dels[p] {
			seen[p] = true
		}
	}
	for p := range t.puts {
		if strings.HasPrefix(p, prefix) {
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Blob returns the blob ID path had in the snapshot this transaction read,
// or "" when it had none (or the snapshot wasn't read from git). It ignores
// the transaction's own writes.
func (t *Tx) Blob(path string) string { return t.snap.Blobs[path] }

// Commit is the mull branch commit this transaction read.
func (t *Tx) Commit() string { return t.snap.Commit }

func (t *Tx) changed() bool { return len(t.puts) > 0 || len(t.dels) > 0 }

// Written reports whether this transaction writes or deletes path.
func (t *Tx) Written(path string) bool {
	_, put := t.puts[path]
	return put || t.dels[path]
}

// View runs fn against the current snapshot without writing.
func (s *Store) View(fn func(tx *Tx) error) error {
	snap, err := s.Load()
	if err != nil {
		return err
	}
	return fn(newTx(snap))
}

// ViewAt runs fn against the mull branch as it was at commit, without
// writing. commit must be a commit in this repository.
func (s *Store) ViewAt(commit string, fn func(tx *Tx) error) error {
	snap, err := s.loadCommit(commit)
	if err != nil {
		return err
	}
	return fn(newTx(snap))
}

func newTx(snap *Snapshot) *Tx {
	return &Tx{snap: snap, puts: map[string][]byte{}, dels: map[string]bool{}}
}

// Init creates the mull branch with the given files. It fails if the branch exists.
func (s *Store) Init(msg string, files map[string][]byte) error {
	commit, err := s.git.RefCommit(Ref)
	if err != nil {
		return err
	}
	if commit != "" {
		return ErrExists
	}
	tx := newTx(&Snapshot{Files: map[string][]byte{}})
	for p, b := range files {
		tx.Put(p, b)
	}
	newCommit, err := s.writeCommit("", tx, msg)
	if err != nil {
		return err
	}
	// An empty old value makes update-ref fail if the ref appeared meanwhile.
	if _, err := s.git.Run(nil, nil, "update-ref", Ref, newCommit, ""); err != nil {
		if c, _ := s.git.RefCommit(Ref); c != "" {
			return ErrExists
		}
		return err
	}
	return nil
}

// ErrMoved means the mull branch moved during a compare-and-swap.
var ErrMoved = errors.New("the mull branch moved during the update")

// SetRef moves the mull branch from old to newCommit with a compare-and-swap.
// old "" means the branch must not exist yet. Used by sync to fast-forward
// or to adopt a merge.
func (s *Store) SetRef(newCommit, old string) error {
	if _, err := s.git.Run(nil, nil, "update-ref", Ref, newCommit, old); err != nil {
		current, rerr := s.git.RefCommit(Ref)
		if rerr == nil && current != old {
			return ErrMoved
		}
		return err
	}
	return nil
}

// Update applies fn to the latest snapshot and commits the result to the
// mull branch with a compare-and-swap. If another writer moved the branch
// first, fn runs again on the new snapshot, so fn must decide only from what
// the transaction shows it. fn making no changes writes nothing.
func (s *Store) Update(msg string, fn func(tx *Tx) error) error {
	_, err := s.Write(msg, fn)
	return err
}

// Write is Update, returning the commit it wrote ("" when fn changed
// nothing).
func (s *Store) Write(msg string, fn func(tx *Tx) error) (string, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		snap, err := s.Load()
		if err != nil {
			return "", err
		}
		tx := newTx(snap)
		if err := fn(tx); err != nil {
			return "", err
		}
		if !tx.changed() {
			return "", nil
		}
		newCommit, err := s.writeCommit(snap.Commit, tx, msg)
		if err != nil {
			return "", err
		}
		_, err = s.git.Run(nil, nil, "update-ref", Ref, newCommit, snap.Commit)
		if err == nil {
			return newCommit, nil
		}
		current, rerr := s.git.RefCommit(Ref)
		if rerr != nil {
			return "", rerr
		}
		if current == snap.Commit {
			return "", err // failed for a reason other than a race
		}
	}
	return "", ErrBusy
}

// writeCommit builds a tree from parent plus tx's changes in a private index
// and commits it. It never touches the working tree or the real index.
func (s *Store) writeCommit(parent string, tx *Tx, msg string) (string, error) {
	dir, err := os.MkdirTemp("", "mull-index-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}

	if parent != "" {
		if _, err := s.git.Run(env, nil, "read-tree", parent); err != nil {
			return "", err
		}
	}
	var info bytes.Buffer
	paths := make([]string, 0, len(tx.puts))
	for p := range tx.puts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		sha, err := s.git.Output(nil, bytes.NewReader(tx.puts[p]), "hash-object", "-w", "--stdin")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&info, "100644 %s\t%s\n", sha, p)
	}
	for p := range tx.dels {
		fmt.Fprintf(&info, "0 %s\t%s\n", strings.Repeat("0", 40), p)
	}
	if info.Len() > 0 {
		if _, err := s.git.Run(env, &info, "update-index", "--index-info"); err != nil {
			return "", err
		}
	}
	tree, err := s.git.Output(env, nil, "write-tree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", tree, "-m", msg}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	return s.git.Output(nil, nil, args...)
}
