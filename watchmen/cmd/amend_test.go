package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"watchmen/internal/storage"
)

func TestIsTerminal(t *testing.T) {
	// Test with a regular file (not a terminal)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	f, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer f.Close()

	if isTerminal(f) {
		t.Error("Regular file should not be detected as terminal")
	}

	// Note: We can't easily test the positive case (actual TTY) in unit tests
	// as it requires a real terminal device
}

// TestAmendNonInteractiveBehavior tests the storage layer behavior
// that supports non-interactive amend (defaulting to index 1)
func TestAmendNonInteractiveBehavior(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	store, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	project, _ := store.AddProject("Test", 100, "")

	// Create two entries
	store.StartEntry(project.ID, "first")
	store.StopEntry("")

	store.StartEntry(project.ID, "second")
	store.StopEntry("")

	// Index 1 should be the most recent (second entry)
	amended, err := store.AmendEntry(1, "updated second")
	if err != nil {
		t.Fatalf("AmendEntry failed: %v", err)
	}

	// Verify it was the second entry (most recent)
	entries := store.ListEntries("", nil, nil)
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// The second entry should have the updated note
	// entries are in chronological order, so index 1 is second entry
	if entries[1].Note != "updated second" {
		t.Errorf("Expected 'updated second', got %q", entries[1].Note)
	}

	// First entry should still have original note
	if entries[0].Note != "first" {
		t.Errorf("First entry should be unchanged, got %q", entries[0].Note)
	}

	// Verify amended return value matches
	if amended.Note != "updated second" {
		t.Errorf("Amended return value note = %q, want 'updated second'", amended.Note)
	}
}

// TestAmendClearNoteNonInteractive tests clearing a note works correctly
func TestAmendClearNoteNonInteractive(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	store, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	project, _ := store.AddProject("Test", 100, "")

	// Create entry with note
	store.StartEntry(project.ID, "has a note")
	store.StopEntry("")

	// Clear it (empty string)
	amended, err := store.AmendEntry(1, "")
	if err != nil {
		t.Fatalf("AmendEntry failed: %v", err)
	}

	if amended.Note != "" {
		t.Errorf("Note should be empty, got %q", amended.Note)
	}
}

// TestAmendNoCompletedEntries tests the error case
func TestAmendNoCompletedEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	store, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	project, _ := store.AddProject("Test", 100, "")

	// Start but don't stop an entry
	store.StartEntry(project.ID, "running")

	// Try to amend - should fail (no completed entries)
	_, err = store.AmendEntry(1, "test")
	if err == nil {
		t.Error("Expected error when no completed entries exist")
	}
}

// TestAmendIndexOutOfRange tests the error case for invalid index
func TestAmendIndexOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	store, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	project, _ := store.AddProject("Test", 100, "")

	// Create one entry
	store.StartEntry(project.ID, "only one")
	store.StopEntry("")

	// Try to amend index 2 (doesn't exist)
	_, err = store.AmendEntry(2, "test")
	if err == nil {
		t.Error("Expected error for out of range index")
	}

	// Try to amend index 0 (invalid)
	_, err = store.AmendEntry(0, "test")
	if err == nil {
		t.Error("Expected error for index 0")
	}

	// Try negative index
	_, err = store.AmendEntry(-1, "test")
	if err == nil {
		t.Error("Expected error for negative index")
	}
}
