package tui

import (
	"path/filepath"
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
