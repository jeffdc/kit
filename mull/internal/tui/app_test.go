package tui

import (
	"path/filepath"
	"sort"
	"testing"

	"mull/internal/model"
	"mull/internal/storage"
)

func TestOpenEditorCmd_BuildsCorrectPath(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewApp(store)
	m := &model.Matter{
		ID:       "ab3f",
		Filename: "ab3f-test-matter.md",
	}

	path := app.matterFilePath(m)
	expected := filepath.Join(dir, ".mull", "matters", "ab3f-test-matter.md")
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}
}

func TestOpenEditorCmd_NilMatter(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	app := NewApp(store)
	path := app.matterFilePath(nil)
	if path != "" {
		t.Errorf("expected empty path for nil matter, got %q", path)
	}
}

func testMatters() []*model.Matter {
	return []*model.Matter{
		{ID: "bb01", Title: "Zeta feature", Status: "planned", Created: "2026-02-15", Updated: "2026-03-01"},
		{ID: "aa02", Title: "Alpha bugfix", Status: "raw", Created: "2026-03-01", Updated: "2026-02-20"},
		{ID: "cc03", Title: "Mid refactor", Status: "active", Created: "2026-01-10", Updated: "2026-03-03"},
		{ID: "dd04", Title: "Done thing", Status: "done", Created: "2026-02-01", Updated: "2026-02-01"},
	}
}

func TestSortByTitle(t *testing.T) {
	matters := testMatters()
	sort.Slice(matters, sortFunc(sortTitle, matters))

	want := []string{"Alpha bugfix", "Done thing", "Mid refactor", "Zeta feature"}
	for i, m := range matters {
		if m.Title != want[i] {
			t.Errorf("index %d: got %q, want %q", i, m.Title, want[i])
		}
	}
}

func TestSortByCreated(t *testing.T) {
	matters := testMatters()
	sort.Slice(matters, sortFunc(sortCreated, matters))

	// Newest first
	want := []string{"aa02", "bb01", "dd04", "cc03"}
	for i, m := range matters {
		if m.ID != want[i] {
			t.Errorf("index %d: got %q, want %q", i, m.ID, want[i])
		}
	}
}

func TestSortByUpdated(t *testing.T) {
	matters := testMatters()
	sort.Slice(matters, sortFunc(sortUpdated, matters))

	// Newest first
	want := []string{"cc03", "bb01", "aa02", "dd04"}
	for i, m := range matters {
		if m.ID != want[i] {
			t.Errorf("index %d: got %q, want %q", i, m.ID, want[i])
		}
	}
}

func TestSortByStatus(t *testing.T) {
	matters := testMatters()
	sort.Slice(matters, sortFunc(sortStatus, matters))

	// Lifecycle order: raw, refined, planned, active, done, dropped
	want := []string{"aa02", "bb01", "cc03", "dd04"}
	for i, m := range matters {
		if m.ID != want[i] {
			t.Errorf("index %d: got %q (status=%s), want %q", i, m.ID, m.Status, want[i])
		}
	}
}

func TestCycleSort(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	app := NewApp(store)

	if app.sortMode != sortTitle {
		t.Fatalf("default sort should be sortTitle, got %d", app.sortMode)
	}

	app.cycleSort()
	if app.sortMode != sortCreated {
		t.Fatalf("after first cycle should be sortCreated, got %d", app.sortMode)
	}

	app.cycleSort()
	if app.sortMode != sortUpdated {
		t.Fatalf("after second cycle should be sortUpdated, got %d", app.sortMode)
	}

	app.cycleSort()
	if app.sortMode != sortStatus {
		t.Fatalf("after third cycle should be sortStatus, got %d", app.sortMode)
	}

	app.cycleSort()
	if app.sortMode != sortTitle {
		t.Fatalf("after fourth cycle should wrap to sortTitle, got %d", app.sortMode)
	}
}
