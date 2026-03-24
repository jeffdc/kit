package storage

import (
	"strings"
	"testing"
	"time"
)

func TestNew_CreatesSessionsDir(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if s.sessionsDir == "" {
		t.Fatal("sessionsDir not set")
	}
}

func TestCreateSession(t *testing.T) {
	s := setupTestStore(t)

	body := "## What changed\n- Added session support\n\n## Decisions\n- Went with file-per-session\n\n## Open questions\n- None"
	sess, err := s.CreateSession([]string{"079d", "ca96"}, body)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	if sess.Date.IsZero() {
		t.Error("Date is zero")
	}
	if len(sess.Matters) != 2 {
		t.Errorf("Matters count = %d, want 2", len(sess.Matters))
	}
	if sess.Body != body {
		t.Errorf("Body = %q, want %q", sess.Body, body)
	}
	if sess.Filename == "" {
		t.Error("Filename is empty")
	}
	// Filename should be timestamp-based
	if !strings.HasSuffix(sess.Filename, ".md") {
		t.Errorf("Filename %q should end with .md", sess.Filename)
	}
}

func TestGetSession(t *testing.T) {
	s := setupTestStore(t)

	body := "## What changed\n- Something"
	created, err := s.CreateSession([]string{"ab12"}, body)
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	got, err := s.GetSession(created.Filename)
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}

	if got.Body != body {
		t.Errorf("Body = %q, want %q", got.Body, body)
	}
	if len(got.Matters) != 1 || got.Matters[0] != "ab12" {
		t.Errorf("Matters = %v, want [ab12]", got.Matters)
	}
}

func TestListSessions_MostRecentFirst(t *testing.T) {
	s := setupTestStore(t)

	s.CreateSessionAt([]string{}, "first", time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC))
	s.CreateSessionAt([]string{}, "third", time.Date(2026, 3, 23, 14, 0, 0, 0, time.UTC))
	s.CreateSessionAt([]string{}, "second", time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC))

	sessions, err := s.ListSessions("")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	if len(sessions) != 3 {
		t.Fatalf("count = %d, want 3", len(sessions))
	}
	// Most recent first
	if sessions[0].Body != "third" {
		t.Errorf("first session body = %q, want 'third'", sessions[0].Body)
	}
	if sessions[2].Body != "first" {
		t.Errorf("last session body = %q, want 'first'", sessions[2].Body)
	}
}

func TestListSessions_FilterByMatter(t *testing.T) {
	s := setupTestStore(t)

	s.CreateSessionAt([]string{"ab12"}, "touches ab12", time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC))
	s.CreateSessionAt([]string{"cd34"}, "touches cd34", time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC))
	s.CreateSessionAt([]string{"ab12", "cd34"}, "touches both", time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC))

	sessions, err := s.ListSessions("ab12")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("count = %d, want 2", len(sessions))
	}
}

func TestSessionContext(t *testing.T) {
	s := setupTestStore(t)

	for i := 0; i < 5; i++ {
		ts := time.Date(2026, 3, 20+i, 10, 0, 0, 0, time.UTC)
		s.CreateSessionAt([]string{}, "session "+string(rune('A'+i)), ts)
	}

	// Default: last 3
	sessions, err := s.SessionContext(3, "")
	if err != nil {
		t.Fatalf("SessionContext() error: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("count = %d, want 3", len(sessions))
	}
	// Most recent first
	if sessions[0].Body != "session E" {
		t.Errorf("first = %q, want 'session E'", sessions[0].Body)
	}
}
