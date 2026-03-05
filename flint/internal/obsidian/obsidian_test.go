package obsidian

import (
	"strings"
	"testing"
)

type fakeRunner struct {
	calls  [][]string
	output string
}

func (f *fakeRunner) Run(args ...string) (string, error) {
	f.calls = append(f.calls, args)
	return f.output, nil
}

func TestAppend_callsDailyAppendWithTaggedContent(t *testing.T) {
	r := &fakeRunner{}
	client := New(r)

	err := client.Append("kit", "cache might have a race condition")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}

	args := r.calls[0]
	if args[0] != "daily:append" {
		t.Errorf("expected daily:append, got %q", args[0])
	}

	contentArg := args[1]
	if !strings.HasPrefix(contentArg, "content=") {
		t.Fatalf("expected content= prefix, got %q", contentArg)
	}
	content := strings.TrimPrefix(contentArg, "content=")
	if !strings.Contains(content, "#kit") {
		t.Errorf("expected #kit tag in content, got %q", content)
	}
	if !strings.Contains(content, "cache might have a race condition") {
		t.Errorf("expected thought in content, got %q", content)
	}
}

func TestSearch_callsSearchWithProjectTag(t *testing.T) {
	r := &fakeRunner{output: "2026-03-05.md\nsome-note.md"}
	client := New(r)

	results, err := client.Search("kit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}

	args := r.calls[0]
	if args[0] != "search" {
		t.Errorf("expected search, got %q", args[0])
	}
	if !strings.Contains(args[1], "#kit") {
		t.Errorf("expected #kit in query, got %q", args[1])
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
