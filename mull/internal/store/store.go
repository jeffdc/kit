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
	"sort"
	"strconv"
	"strings"

	"mull/internal/git"
)

// Ref is the branch mull's data lives on. It is a constant on purpose:
// nothing in mull reads or writes any other ref.
const Ref = "refs/heads/mull"

// maxAttempts bounds compare-and-swap retries when writers race.
const maxAttempts = 20

// ErrNotInitialized means the mull branch does not exist yet.
var ErrNotInitialized = errors.New("mull is not initialized in this repository (run: mull init)")

// ErrExists means init was run where the mull branch already exists.
var ErrExists = errors.New("mull is already initialized in this repository")

// Store reads and writes the mull branch of one repository.
type Store struct {
	git *git.Repo
}

// New returns a store for the repository r.
func New(r *git.Repo) *Store { return &Store{git: r} }

// Snapshot is the content of the mull branch at one commit.
type Snapshot struct {
	Commit string
	Files  map[string][]byte
}

// Initialized reports whether the mull branch exists.
func (s *Store) Initialized() (bool, error) {
	c, err := s.git.RefCommit(Ref)
	return c != "", err
}

// Load reads the whole mull branch.
func (s *Store) Load() (*Snapshot, error) {
	commit, err := s.git.RefCommit(Ref)
	if err != nil {
		return nil, err
	}
	if commit == "" {
		return nil, ErrNotInitialized
	}
	return s.loadCommit(commit)
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
	snap := &Snapshot{Commit: commit, Files: make(map[string][]byte, len(entries))}
	if len(entries) == 0 {
		return snap, nil
	}
	var req bytes.Buffer
	for _, e := range entries {
		req.WriteString(e.sha)
		req.WriteByte('\n')
	}
	out, err := s.git.Run(nil, &req, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(bytes.NewReader(out))
	for _, e := range entries {
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
		data := make([]byte, size)
		if _, err := io.ReadFull(rd, data); err != nil {
			return nil, fmt.Errorf("reading blob %s: %w", e.sha, err)
		}
		if _, err := rd.ReadByte(); err != nil { // trailing newline
			return nil, fmt.Errorf("reading blob %s: %w", e.sha, err)
		}
		snap.Files[e.path] = data
	}
	return snap, nil
}

// Tx is a set of changes against one snapshot.
type Tx struct {
	snap *Snapshot
	puts map[string][]byte
	dels map[string]bool
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
	delete(t.dels, path)
	t.puts[path] = data
}

// Delete removes a file.
func (t *Tx) Delete(path string) {
	delete(t.puts, path)
	t.dels[path] = true
}

// List returns the sorted paths under prefix as seen by this transaction.
func (t *Tx) List(prefix string) []string {
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

func (t *Tx) changed() bool { return len(t.puts) > 0 || len(t.dels) > 0 }

// View runs fn against the current snapshot without writing.
func (s *Store) View(fn func(tx *Tx) error) error {
	snap, err := s.Load()
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

// Update applies fn to the latest snapshot and commits the result to the
// mull branch with a compare-and-swap. If another writer moved the branch
// first, fn runs again on the new snapshot, so fn must decide only from what
// the transaction shows it. fn making no changes writes nothing.
func (s *Store) Update(msg string, fn func(tx *Tx) error) error {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		snap, err := s.Load()
		if err != nil {
			return err
		}
		tx := newTx(snap)
		if err := fn(tx); err != nil {
			return err
		}
		if !tx.changed() {
			return nil
		}
		newCommit, err := s.writeCommit(snap.Commit, tx, msg)
		if err != nil {
			return err
		}
		_, err = s.git.Run(nil, nil, "update-ref", Ref, newCommit, snap.Commit)
		if err == nil {
			return nil
		}
		current, rerr := s.git.RefCommit(Ref)
		if rerr != nil {
			return rerr
		}
		if current == snap.Commit {
			return err // failed for a reason other than a race
		}
	}
	return fmt.Errorf("could not write to %s: too many concurrent writers", Ref)
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
