package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"mull/internal/storage"
)

func runRootCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	store = nil
	rootCmd.SetArgs(args)
	execErr := rootCmd.Execute()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}

	if execErr != nil {
		t.Fatalf("rootCmd.Execute() error: %v", execErr)
	}
	return string(out)
}

func TestArchiveCommandArchivesTerminalMatters(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	done, err := s.CreateMatter("Done matter", map[string]any{"status": "done"})
	if err != nil {
		t.Fatalf("CreateMatter(done) error: %v", err)
	}
	dropped, err := s.CreateMatter("Dropped matter", map[string]any{"status": "dropped"})
	if err != nil {
		t.Fatalf("CreateMatter(dropped) error: %v", err)
	}
	active, err := s.CreateMatter("Active matter", map[string]any{"status": "active"})
	if err != nil {
		t.Fatalf("CreateMatter(active) error: %v", err)
	}
	if err := s.LinkMatters(active.ID, "relates", done.ID); err != nil {
		t.Fatalf("LinkMatters() error: %v", err)
	}
	if err := s.DocketAdd(done.ID, "", ""); err != nil {
		t.Fatalf("DocketAdd() error: %v", err)
	}
	if _, err := s.CreateSession([]string{done.ID}, "session stays in place"); err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	out := runRootCommand(t, dir, "archive")

	var got struct {
		Archived []archiveEntry `json:"archived"`
		Count    int            `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput: %s", err, out)
	}

	if got.Count != 2 {
		t.Fatalf("count = %d, want 2", got.Count)
	}
	if len(got.Archived) != 2 {
		t.Fatalf("archived len = %d, want 2", len(got.Archived))
	}

	activeMatter, err := s.GetMatter(active.ID)
	if err != nil {
		t.Fatalf("GetMatter(active) error: %v", err)
	}
	if len(activeMatter.Relates) != 0 {
		t.Errorf("active matter relates = %v, want empty", activeMatter.Relates)
	}

	entries, err := s.LoadDocket()
	if err != nil {
		t.Fatalf("LoadDocket() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("docket len = %d, want 0", len(entries))
	}

	activeMatters, err := s.ListMatters(nil)
	if err != nil {
		t.Fatalf("ListMatters() error: %v", err)
	}
	if len(activeMatters) != 1 {
		t.Fatalf("active matters len = %d, want 1", len(activeMatters))
	}

	archivedFiles, err := os.ReadDir(filepath.Join(dir, ".mull", "archive"))
	if err != nil {
		t.Fatalf("ReadDir(archive) error: %v", err)
	}
	if len(archivedFiles) != 2 {
		t.Fatalf("archive file count = %d, want 2", len(archivedFiles))
	}

	sessionFiles, err := os.ReadDir(filepath.Join(dir, ".mull", "sessions"))
	if err != nil {
		t.Fatalf("ReadDir(sessions) error: %v", err)
	}
	if len(sessionFiles) != 1 {
		t.Fatalf("session file count = %d, want 1", len(sessionFiles))
	}

	if _, err := s.GetMatter(done.ID); err == nil {
		t.Fatal("expected done matter to be archived out of active matters")
	}
	if _, err := s.GetMatter(dropped.ID); err == nil {
		t.Fatal("expected dropped matter to be archived out of active matters")
	}
}

func TestArchiveCommandReportsNoMattersArchived(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := s.CreateMatter("Active matter", map[string]any{"status": "active"}); err != nil {
		t.Fatalf("CreateMatter() error: %v", err)
	}
	if _, err := s.CreateSession([]string{}, "session stays in place"); err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	out := runRootCommand(t, dir, "archive")

	var got struct {
		Archived []archiveEntry `json:"archived"`
		Count    int            `json:"count"`
		Message  string         `json:"message"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\noutput: %s", err, out)
	}

	if got.Count != 0 {
		t.Fatalf("count = %d, want 0", got.Count)
	}
	if len(got.Archived) != 0 {
		t.Fatalf("archived len = %d, want 0", len(got.Archived))
	}
	if got.Message != "no matters archived" {
		t.Errorf("message = %q, want %q", got.Message, "no matters archived")
	}

	sessionFiles, err := os.ReadDir(filepath.Join(dir, ".mull", "sessions"))
	if err != nil {
		t.Fatalf("ReadDir(sessions) error: %v", err)
	}
	if len(sessionFiles) != 1 {
		t.Fatalf("session file count = %d, want 1", len(sessionFiles))
	}
}
