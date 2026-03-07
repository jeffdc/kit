package cmd

import (
	"path/filepath"
	"testing"

	"watchmen/internal/storage"
)

func TestDeleteEntryByIndex(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	s, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	project, _ := s.AddProject("Test", 100, "")

	// Create two completed entries
	s.StartEntry(project.ID, "first entry")
	s.StopEntry("")
	s.StartEntry(project.ID, "second entry")
	s.StopEntry("")

	// Delete most recent (index 1 = "second entry")
	entries := s.ListEntries("", nil, nil)
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Resolve index 1 (most recent completed) to ID
	var completed []string
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Completed {
			completed = append(completed, entries[i].ID)
		}
	}
	if len(completed) < 1 {
		t.Fatal("Expected at least 1 completed entry")
	}

	err = s.DeleteEntry(completed[0])
	if err != nil {
		t.Fatalf("DeleteEntry failed: %v", err)
	}

	// Verify only first entry remains
	remaining := s.ListEntries("", nil, nil)
	if len(remaining) != 1 {
		t.Fatalf("Expected 1 entry after delete, got %d", len(remaining))
	}
	if remaining[0].Note != "first entry" {
		t.Errorf("Expected remaining entry to be 'first entry', got %q", remaining[0].Note)
	}
}

func TestDeleteEntryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	s, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	err = s.DeleteEntry("nonexistent-id")
	if err == nil {
		t.Error("Expected error when deleting nonexistent entry")
	}
}

func TestDeleteCommandNonInteractive(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")

	// Set up root command with test data path
	dataPath = path
	var err error
	store, err = storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	project, _ := store.AddProject("Test", 100, "")
	store.StartEntry(project.ID, "to delete")
	store.StopEntry("")

	// Execute delete command with --yes flag
	rootCmd.SetArgs([]string{"delete", "1", "--yes", "--data", path})
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("delete command failed: %v", err)
	}

	// Reload store and verify entry was deleted
	store, _ = storage.New(path)
	entries := store.ListEntries("", nil, nil)
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after delete, got %d", len(entries))
	}
}
