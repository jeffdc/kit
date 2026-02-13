package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return s
}

func TestNew(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Verify .mull/matters/ was created
	info, err := os.Stat(filepath.Join(dir, ".mull", "matters"))
	if err != nil {
		t.Fatalf("matters dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("matters is not a directory")
	}
	_ = s
}

func TestCreateMatter(t *testing.T) {
	s := setupTestStore(t)

	m, err := s.CreateMatter("Add an RSS feed", nil)
	if err != nil {
		t.Fatalf("CreateMatter() error: %v", err)
	}

	if len(m.ID) != 4 {
		t.Errorf("ID length = %d, want 4", len(m.ID))
	}
	if m.Title != "Add an RSS feed" {
		t.Errorf("Title = %q, want %q", m.Title, "Add an RSS feed")
	}
	if m.Status != "raw" {
		t.Errorf("Status = %q, want %q", m.Status, "raw")
	}
	if m.Created == "" {
		t.Error("Created is empty")
	}

	// Verify file exists
	path := filepath.Join(s.mattersDir, m.Filename)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("matter file not created: %v", err)
	}
}

func TestCreateMatterWithMeta(t *testing.T) {
	s := setupTestStore(t)

	meta := map[string]any{
		"status": "refined",
		"effort": "small",
		"tags":   "content,low-effort",
	}
	m, err := s.CreateMatter("Dark mode", meta)
	if err != nil {
		t.Fatalf("CreateMatter() error: %v", err)
	}

	if m.Status != "refined" {
		t.Errorf("Status = %q, want %q", m.Status, "refined")
	}
	if m.Effort != "small" {
		t.Errorf("Effort = %q, want %q", m.Effort, "small")
	}
	if len(m.Tags) != 2 || m.Tags[0] != "content" || m.Tags[1] != "low-effort" {
		t.Errorf("Tags = %v, want [content low-effort]", m.Tags)
	}
}

func TestGetMatter(t *testing.T) {
	s := setupTestStore(t)

	created, err := s.CreateMatter("Test matter", nil)
	if err != nil {
		t.Fatalf("CreateMatter() error: %v", err)
	}

	got, err := s.GetMatter(created.ID)
	if err != nil {
		t.Fatalf("GetMatter() error: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if got.Title != "Test matter" {
		t.Errorf("Title = %q, want %q", got.Title, "Test matter")
	}
	if got.Status != "raw" {
		t.Errorf("Status = %q, want %q", got.Status, "raw")
	}
}

func TestGetMatterNotFound(t *testing.T) {
	s := setupTestStore(t)

	_, err := s.GetMatter("zzzz")
	if err == nil {
		t.Fatal("expected error for non-existent matter")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

func TestListMatters(t *testing.T) {
	s := setupTestStore(t)

	s.CreateMatter("First", map[string]any{"status": "raw"})
	s.CreateMatter("Second", map[string]any{"status": "refined"})
	s.CreateMatter("Third", map[string]any{"status": "raw"})

	// List all
	all, err := s.ListMatters(nil)
	if err != nil {
		t.Fatalf("ListMatters() error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}

	// Filter by status
	raw, err := s.ListMatters(map[string]string{"status": "raw"})
	if err != nil {
		t.Fatalf("ListMatters(status=raw) error: %v", err)
	}
	if len(raw) != 2 {
		t.Errorf("len = %d, want 2", len(raw))
	}
}

func TestListMattersWithTagFilter(t *testing.T) {
	s := setupTestStore(t)

	s.CreateMatter("Tagged", map[string]any{"tags": "content,design"})
	s.CreateMatter("Untagged", nil)

	tagged, err := s.ListMatters(map[string]string{"tag": "content"})
	if err != nil {
		t.Fatalf("ListMatters(tag=content) error: %v", err)
	}
	if len(tagged) != 1 {
		t.Errorf("len = %d, want 1", len(tagged))
	}
}

func TestSearchMatters(t *testing.T) {
	s := setupTestStore(t)

	s.CreateMatter("Add RSS feed", nil)
	s.CreateMatter("Dark mode support", nil)

	results, err := s.SearchMatters("rss")
	if err != nil {
		t.Fatalf("SearchMatters() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len = %d, want 1", len(results))
	}
	if results[0].Title != "Add RSS feed" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Add RSS feed")
	}
}

func TestUpdateMatter(t *testing.T) {
	s := setupTestStore(t)

	m, _ := s.CreateMatter("Test", nil)

	updated, err := s.UpdateMatter(m.ID, "status", "refined")
	if err != nil {
		t.Fatalf("UpdateMatter() error: %v", err)
	}
	if updated.Status != "refined" {
		t.Errorf("Status = %q, want %q", updated.Status, "refined")
	}

	// Verify persistence
	got, _ := s.GetMatter(m.ID)
	if got.Status != "refined" {
		t.Errorf("persisted Status = %q, want %q", got.Status, "refined")
	}
}

func TestAppendBody(t *testing.T) {
	s := setupTestStore(t)

	m, _ := s.CreateMatter("Test", nil)

	m, err := s.AppendBody(m.ID, "First paragraph.")
	if err != nil {
		t.Fatalf("AppendBody() error: %v", err)
	}
	if m.Body != "First paragraph." {
		t.Errorf("Body = %q, want %q", m.Body, "First paragraph.")
	}

	m, err = s.AppendBody(m.ID, "Second paragraph.")
	if err != nil {
		t.Fatalf("AppendBody() error: %v", err)
	}
	if !strings.Contains(m.Body, "First paragraph.") || !strings.Contains(m.Body, "Second paragraph.") {
		t.Errorf("Body = %q, want both paragraphs", m.Body)
	}
}

func TestDeleteMatter(t *testing.T) {
	s := setupTestStore(t)

	m, _ := s.CreateMatter("To delete", nil)

	err := s.DeleteMatter(m.ID)
	if err != nil {
		t.Fatalf("DeleteMatter() error: %v", err)
	}

	_, err = s.GetMatter(m.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Add an RSS feed", "add-an-rss-feed"},
		{"Dark mode", "dark-mode"},
		{"Hello   World!!", "hello-world"},
		{"café latte", "café-latte"},
	}

	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRoundtripExtraFields(t *testing.T) {
	s := setupTestStore(t)

	meta := map[string]any{
		"priority": "high",
	}
	m, err := s.CreateMatter("With extras", meta)
	if err != nil {
		t.Fatalf("CreateMatter() error: %v", err)
	}

	got, err := s.GetMatter(m.ID)
	if err != nil {
		t.Fatalf("GetMatter() error: %v", err)
	}
	if got.Extra["priority"] != "high" {
		t.Errorf("Extra[priority] = %v, want %q", got.Extra["priority"], "high")
	}
}
