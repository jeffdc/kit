package obsidian

import (
	"flint/internal/git"
	"strings"
	"testing"
)

func TestFormatPark_includesAllSections(t *testing.T) {
	ctx := &git.RepoContext{
		Branch:        "feat/caching",
		DirtyFiles:    []string{"?? cache.go", "M  main.go"},
		RecentCommits: []string{"abc1234 add cache layer", "def5678 initial commit"},
	}
	notes := "Was investigating whether the cache invalidation is racy. Think the mutex needs to cover the whole read-modify-write."

	result := FormatPark("kit", ctx, notes)

	if !strings.Contains(result, "#kit") {
		t.Error("missing project tag")
	}
	if !strings.Contains(result, "#park") {
		t.Error("missing park tag")
	}
	if !strings.Contains(result, "feat/caching") {
		t.Error("missing branch")
	}
	if !strings.Contains(result, "cache.go") {
		t.Error("missing dirty files")
	}
	if !strings.Contains(result, "add cache layer") {
		t.Error("missing recent commits")
	}
	if !strings.Contains(result, notes) {
		t.Error("missing user notes")
	}
}

func TestFormatPark_appendsViaClient(t *testing.T) {
	r := &fakeRunner{}
	client := New(r)
	ctx := &git.RepoContext{
		Branch:     "main",
		DirtyFiles: []string{"?? new.go"},
	}

	err := client.Park("myproject", ctx, "leaving for lunch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	if r.calls[0][0] != "daily:append" {
		t.Errorf("expected daily:append, got %q", r.calls[0][0])
	}
}
