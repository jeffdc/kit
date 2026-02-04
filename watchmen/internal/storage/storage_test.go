package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")
	store, err := New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store, path
}

func TestMigrationFromV1(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")

	// Create a v1 format file
	v1Data := `{
		"version": 1,
		"projects": [
			{
				"id": "proj1",
				"name": "Test Project",
				"hourly_rate": 100,
				"created_at": "2024-01-01T00:00:00Z"
			}
		],
		"entries": [
			{
				"id": "entry1",
				"project_id": "proj1",
				"note": "Test entry",
				"start_time": "2024-01-15T09:00:00Z",
				"end_time": "2024-01-15T11:00:00Z"
			},
			{
				"id": "entry2",
				"project_id": "proj1",
				"note": "Running entry",
				"start_time": "2024-01-15T14:00:00Z"
			}
		]
	}`

	if err := os.WriteFile(path, []byte(v1Data), 0644); err != nil {
		t.Fatalf("Failed to write v1 data: %v", err)
	}

	// Load the store, which should trigger migration
	store, err := New(path)
	if err != nil {
		t.Fatalf("Failed to load store: %v", err)
	}

	// Verify migration
	entries := store.ListEntries("", nil, nil)
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Check first entry (completed)
	e1 := entries[0]
	if len(e1.Segments) != 1 {
		t.Errorf("Entry 1 should have 1 segment, got %d", len(e1.Segments))
	}
	if !e1.Completed {
		t.Error("Entry 1 should be completed")
	}
	if e1.Segments[0].End == nil {
		t.Error("Entry 1 segment should have end time")
	}

	// Check second entry (was running, now has nil end in segment)
	e2 := entries[1]
	if len(e2.Segments) != 1 {
		t.Errorf("Entry 2 should have 1 segment, got %d", len(e2.Segments))
	}
	if e2.Completed {
		t.Error("Entry 2 should not be completed")
	}
	if e2.Segments[0].End != nil {
		t.Error("Entry 2 segment should have nil end time (was running)")
	}

	// Verify file was updated to v2
	data, _ := os.ReadFile(path)
	var check struct {
		Version int `json:"version"`
	}
	json.Unmarshal(data, &check)
	if check.Version != CurrentVersion {
		t.Errorf("File should be version %d, got %d", CurrentVersion, check.Version)
	}
}

func TestMigrationFromV0(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_data.json")

	// Create a file without version (v0)
	v0Data := `{
		"projects": [
			{
				"id": "proj1",
				"name": "Test Project",
				"hourly_rate": 100,
				"created_at": "2024-01-01T00:00:00Z"
			}
		],
		"entries": [
			{
				"id": "entry1",
				"project_id": "proj1",
				"start_time": "2024-01-15T09:00:00Z",
				"end_time": "2024-01-15T11:00:00Z"
			}
		]
	}`

	if err := os.WriteFile(path, []byte(v0Data), 0644); err != nil {
		t.Fatalf("Failed to write v0 data: %v", err)
	}

	// Load should trigger migration
	store, err := New(path)
	if err != nil {
		t.Fatalf("Failed to load store: %v", err)
	}

	entries := store.ListEntries("", nil, nil)
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if len(entries[0].Segments) != 1 {
		t.Errorf("Entry should have 1 segment after migration")
	}
}

func TestPauseEntry(t *testing.T) {
	store, _ := setupTestStore(t)

	// Add a project
	project, _ := store.AddProject("Test", 100, "")

	// Start an entry
	_, err := store.StartEntry(project.ID, "test note")
	if err != nil {
		t.Fatalf("Failed to start entry: %v", err)
	}

	// Verify it's running
	active := store.ActiveEntry()
	if active == nil {
		t.Fatal("Expected active entry")
	}
	if !active.IsRunning() {
		t.Error("Entry should be running")
	}

	// Pause it
	paused, err := store.PauseEntry()
	if err != nil {
		t.Fatalf("Failed to pause entry: %v", err)
	}

	if !paused.IsPaused() {
		t.Error("Entry should be paused")
	}
	if paused.IsRunning() {
		t.Error("Entry should not be running after pause")
	}
	if paused.Segments[0].End == nil {
		t.Error("First segment should have end time after pause")
	}
}

func TestPauseEntryErrors(t *testing.T) {
	store, _ := setupTestStore(t)

	// Try to pause with no active entry
	_, err := store.PauseEntry()
	if err != ErrNoActiveEntry {
		t.Errorf("Expected ErrNoActiveEntry, got %v", err)
	}

	// Add project and start entry
	project, _ := store.AddProject("Test", 100, "")
	store.StartEntry(project.ID, "")

	// Pause it
	store.PauseEntry()

	// Try to pause again
	_, err = store.PauseEntry()
	if err != ErrAlreadyPaused {
		t.Errorf("Expected ErrAlreadyPaused, got %v", err)
	}
}

func TestResumeEntry(t *testing.T) {
	store, _ := setupTestStore(t)

	// Add a project and start/pause an entry
	project, _ := store.AddProject("Test", 100, "")
	store.StartEntry(project.ID, "test note")
	store.PauseEntry()

	// Verify it's paused
	active := store.ActiveEntry()
	if !active.IsPaused() {
		t.Fatal("Entry should be paused before resume")
	}
	initialSegments := len(active.Segments)

	// Resume it
	resumed, err := store.ResumeEntry()
	if err != nil {
		t.Fatalf("Failed to resume entry: %v", err)
	}

	if !resumed.IsRunning() {
		t.Error("Entry should be running after resume")
	}
	if resumed.IsPaused() {
		t.Error("Entry should not be paused after resume")
	}
	if len(resumed.Segments) != initialSegments+1 {
		t.Errorf("Expected %d segments, got %d", initialSegments+1, len(resumed.Segments))
	}

	// Last segment should have no end time
	lastSeg := resumed.Segments[len(resumed.Segments)-1]
	if lastSeg.End != nil {
		t.Error("New segment should have nil end time")
	}
}

func TestResumeEntryErrors(t *testing.T) {
	store, _ := setupTestStore(t)

	// Try to resume with no paused entry
	_, err := store.ResumeEntry()
	if err != ErrNoPausedEntry {
		t.Errorf("Expected ErrNoPausedEntry, got %v", err)
	}

	// Add project and start entry (running, not paused)
	project, _ := store.AddProject("Test", 100, "")
	store.StartEntry(project.ID, "")

	// Try to resume a running entry
	_, err = store.ResumeEntry()
	if err != ErrActiveEntry {
		t.Errorf("Expected ErrActiveEntry, got %v", err)
	}
}

func TestStopPausedEntry(t *testing.T) {
	store, _ := setupTestStore(t)

	// Add a project, start, and pause an entry
	project, _ := store.AddProject("Test", 100, "")
	store.StartEntry(project.ID, "initial note")
	store.PauseEntry()

	// Stop the paused entry
	stopped, err := store.StopEntry("final note")
	if err != nil {
		t.Fatalf("Failed to stop paused entry: %v", err)
	}

	if !stopped.Completed {
		t.Error("Entry should be completed")
	}
	if stopped.IsRunning() || stopped.IsPaused() {
		t.Error("Stopped entry should not be running or paused")
	}
	if stopped.Note != "initial note | final note" {
		t.Errorf("Expected combined note, got %q", stopped.Note)
	}
}

func TestStartEntryBlockedByPaused(t *testing.T) {
	store, _ := setupTestStore(t)

	// Add projects
	project1, _ := store.AddProject("Test1", 100, "")
	project2, _ := store.AddProject("Test2", 100, "")

	// Start and pause on project1
	store.StartEntry(project1.ID, "")
	store.PauseEntry()

	// Try to start on project2 - should fail
	_, err := store.StartEntry(project2.ID, "")
	if err != ErrActiveEntry {
		t.Errorf("Expected ErrActiveEntry when starting with paused entry, got %v", err)
	}
}

func TestPauseResumeDuration(t *testing.T) {
	store, _ := setupTestStore(t)

	project, _ := store.AddProject("Test", 100, "")

	// Start entry
	entry, _ := store.StartEntry(project.ID, "")
	time.Sleep(100 * time.Millisecond)

	// Pause
	entry, _ = store.PauseEntry()
	pausedDuration := entry.Duration()

	// Wait while paused - duration should not increase
	time.Sleep(100 * time.Millisecond)
	entry = store.ActiveEntry()
	afterWaitDuration := entry.Duration()

	// Duration should be the same (within small tolerance)
	diff := afterWaitDuration - pausedDuration
	if diff > 10*time.Millisecond {
		t.Errorf("Duration increased while paused: before=%v, after=%v", pausedDuration, afterWaitDuration)
	}

	// Resume
	entry, _ = store.ResumeEntry()
	time.Sleep(100 * time.Millisecond)

	// Stop
	entry, _ = store.StopEntry("")

	// Total duration should be roughly 200ms (100ms running + 100ms after resume)
	// Not 300ms (which would include the paused time)
	finalDuration := entry.Duration()
	if finalDuration < 150*time.Millisecond || finalDuration > 300*time.Millisecond {
		t.Errorf("Final duration %v seems wrong, expected ~200ms", finalDuration)
	}
}

func TestAmendEntry(t *testing.T) {
	store, _ := setupTestStore(t)

	project, _ := store.AddProject("Test", 100, "")

	// Create three completed entries
	store.StartEntry(project.ID, "first entry")
	time.Sleep(10 * time.Millisecond)
	store.StopEntry("")

	store.StartEntry(project.ID, "second entry")
	time.Sleep(10 * time.Millisecond)
	store.StopEntry("")

	store.StartEntry(project.ID, "third entry")
	time.Sleep(10 * time.Millisecond)
	store.StopEntry("")

	// Amend the most recent entry (index 1)
	amended, err := store.AmendEntry(1, "updated third entry")
	if err != nil {
		t.Fatalf("Failed to amend entry: %v", err)
	}

	if amended.Note != "updated third entry" {
		t.Errorf("Expected note 'updated third entry', got %q", amended.Note)
	}

	// Amend the second entry (index 2)
	amended, err = store.AmendEntry(2, "updated second entry")
	if err != nil {
		t.Fatalf("Failed to amend entry: %v", err)
	}

	if amended.Note != "updated second entry" {
		t.Errorf("Expected note 'updated second entry', got %q", amended.Note)
	}

	// Verify changes persisted
	entries := store.ListEntries("", nil, nil)
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}
}

func TestAmendEntryErrors(t *testing.T) {
	store, _ := setupTestStore(t)

	project, _ := store.AddProject("Test", 100, "")

	// Create one completed entry
	store.StartEntry(project.ID, "test")
	store.StopEntry("")

	// Try to amend with invalid index
	_, err := store.AmendEntry(0, "test")
	if err == nil {
		t.Error("Expected error for index 0")
	}

	_, err = store.AmendEntry(2, "test")
	if err == nil {
		t.Error("Expected error for index 2 (only 1 entry)")
	}

	_, err = store.AmendEntry(-1, "test")
	if err == nil {
		t.Error("Expected error for negative index")
	}
}

func TestAmendRunningEntry(t *testing.T) {
	store, _ := setupTestStore(t)

	project, _ := store.AddProject("Test", 100, "")

	// Create completed entry
	store.StartEntry(project.ID, "completed")
	store.StopEntry("")

	// Start a running entry
	store.StartEntry(project.ID, "running")

	// Try to amend - should only see completed entry
	amended, err := store.AmendEntry(1, "updated")
	if err != nil {
		t.Fatalf("Should be able to amend completed entry: %v", err)
	}

	if amended.Note != "updated" {
		t.Errorf("Expected 'updated', got %q", amended.Note)
	}

	// Running entry should not be amendable
	_, err = store.AmendEntry(2, "test")
	if err == nil {
		t.Error("Should not be able to amend running entry via index")
	}
}

func TestAmendClearNote(t *testing.T) {
	store, _ := setupTestStore(t)

	project, _ := store.AddProject("Test", 100, "")

	// Create entry with note
	store.StartEntry(project.ID, "original note")
	store.StopEntry("")

	// Clear the note
	amended, err := store.AmendEntry(1, "")
	if err != nil {
		t.Fatalf("Failed to clear note: %v", err)
	}

	if amended.Note != "" {
		t.Errorf("Expected empty note, got %q", amended.Note)
	}
}
