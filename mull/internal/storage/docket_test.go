package storage

import (
	"os"
	"testing"

	"mull/internal/model"
)

func TestLoadDocketEmpty(t *testing.T) {
	s := setupTestStore(t)

	entries, err := s.LoadDocket()
	if err != nil {
		t.Fatalf("LoadDocket() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len = %d, want 0", len(entries))
	}
}

func TestSaveAndLoadDocket(t *testing.T) {
	s := setupTestStore(t)

	want := []model.DocketEntry{
		{ID: "a4c8", Note: "do this first"},
		{ID: "ab3f"},
		{ID: "c7d1", Note: "stretch goal"},
	}

	if err := s.SaveDocket(want); err != nil {
		t.Fatalf("SaveDocket() error: %v", err)
	}

	got, err := s.LoadDocket()
	if err != nil {
		t.Fatalf("LoadDocket() error: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Errorf("[%d] ID = %q, want %q", i, got[i].ID, want[i].ID)
		}
		if got[i].Note != want[i].Note {
			t.Errorf("[%d] Note = %q, want %q", i, got[i].Note, want[i].Note)
		}
	}

	// Verify the file exists on disk
	if _, err := os.Stat(s.docketPath()); err != nil {
		t.Fatalf("docket.yml not created: %v", err)
	}
}

func TestDocketAdd(t *testing.T) {
	s := setupTestStore(t)

	if err := s.DocketAdd("aaaa", "", "first"); err != nil {
		t.Fatalf("DocketAdd() error: %v", err)
	}
	if err := s.DocketAdd("bbbb", "", "second"); err != nil {
		t.Fatalf("DocketAdd() error: %v", err)
	}

	entries, _ := s.LoadDocket()
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].ID != "aaaa" {
		t.Errorf("[0] ID = %q, want %q", entries[0].ID, "aaaa")
	}
	if entries[1].ID != "bbbb" {
		t.Errorf("[1] ID = %q, want %q", entries[1].ID, "bbbb")
	}
}

func TestDocketAddDuplicate(t *testing.T) {
	s := setupTestStore(t)

	s.DocketAdd("aaaa", "", "")
	err := s.DocketAdd("aaaa", "", "")
	if err == nil {
		t.Fatal("expected error for duplicate add")
	}
}

func TestDocketAddAfter(t *testing.T) {
	s := setupTestStore(t)

	s.DocketAdd("aaaa", "", "")
	s.DocketAdd("cccc", "", "")

	// Insert bbbb after aaaa
	if err := s.DocketAdd("bbbb", "aaaa", "middle"); err != nil {
		t.Fatalf("DocketAdd(after) error: %v", err)
	}

	entries, _ := s.LoadDocket()
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	ids := []string{entries[0].ID, entries[1].ID, entries[2].ID}
	want := []string{"aaaa", "bbbb", "cccc"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("[%d] ID = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestDocketAddAfterNotFound(t *testing.T) {
	s := setupTestStore(t)

	err := s.DocketAdd("aaaa", "zzzz", "")
	if err == nil {
		t.Fatal("expected error for after-id not found")
	}
}

func TestDocketRemove(t *testing.T) {
	s := setupTestStore(t)

	s.DocketAdd("aaaa", "", "")
	s.DocketAdd("bbbb", "", "")
	s.DocketAdd("cccc", "", "")

	if err := s.DocketRemove("bbbb"); err != nil {
		t.Fatalf("DocketRemove() error: %v", err)
	}

	entries, _ := s.LoadDocket()
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].ID != "aaaa" || entries[1].ID != "cccc" {
		t.Errorf("entries = %v, want [aaaa cccc]", entries)
	}
}

func TestDocketRemoveNotFound(t *testing.T) {
	s := setupTestStore(t)

	err := s.DocketRemove("zzzz")
	if err == nil {
		t.Fatal("expected error for removing non-existent entry")
	}
}

func TestDocketMove(t *testing.T) {
	s := setupTestStore(t)

	s.DocketAdd("aaaa", "", "")
	s.DocketAdd("bbbb", "", "")
	s.DocketAdd("cccc", "", "")

	// Move cccc to after aaaa (before bbbb)
	if err := s.DocketMove("cccc", "aaaa"); err != nil {
		t.Fatalf("DocketMove() error: %v", err)
	}

	entries, _ := s.LoadDocket()
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	ids := []string{entries[0].ID, entries[1].ID, entries[2].ID}
	want := []string{"aaaa", "cccc", "bbbb"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("[%d] ID = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestDocketMoveNotFound(t *testing.T) {
	s := setupTestStore(t)

	s.DocketAdd("aaaa", "", "")

	err := s.DocketMove("zzzz", "aaaa")
	if err == nil {
		t.Fatal("expected error for moving non-existent entry")
	}
}

func TestDocketMoveAfterNotFound(t *testing.T) {
	s := setupTestStore(t)

	s.DocketAdd("aaaa", "", "")

	err := s.DocketMove("aaaa", "zzzz")
	if err == nil {
		t.Fatal("expected error for after-id not found")
	}
}
