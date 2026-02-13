package storage

import (
	"testing"
)

func TestLinkRelates(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Feature A", nil)
	m2, _ := s.CreateMatter("Feature B", nil)

	if err := s.LinkMatters(m1.ID, "relates", m2.ID); err != nil {
		t.Fatalf("LinkMatters() error: %v", err)
	}

	got1, _ := s.GetMatter(m1.ID)
	got2, _ := s.GetMatter(m2.ID)

	if !containsString(got1.Relates, m2.ID) {
		t.Errorf("m1.Relates = %v, want to contain %s", got1.Relates, m2.ID)
	}
	if !containsString(got2.Relates, m1.ID) {
		t.Errorf("m2.Relates = %v, want to contain %s", got2.Relates, m1.ID)
	}
}

func TestLinkBlocks(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Blocker", nil)
	m2, _ := s.CreateMatter("Blocked", nil)

	if err := s.LinkMatters(m1.ID, "blocks", m2.ID); err != nil {
		t.Fatalf("LinkMatters() error: %v", err)
	}

	got1, _ := s.GetMatter(m1.ID)
	got2, _ := s.GetMatter(m2.ID)

	if !containsString(got1.Blocks, m2.ID) {
		t.Errorf("m1.Blocks = %v, want to contain %s", got1.Blocks, m2.ID)
	}
	if !containsString(got2.Needs, m1.ID) {
		t.Errorf("m2.Needs = %v, want to contain %s", got2.Needs, m1.ID)
	}
}

func TestLinkNeeds(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Dependent", nil)
	m2, _ := s.CreateMatter("Dependency", nil)

	if err := s.LinkMatters(m1.ID, "needs", m2.ID); err != nil {
		t.Fatalf("LinkMatters() error: %v", err)
	}

	got1, _ := s.GetMatter(m1.ID)
	got2, _ := s.GetMatter(m2.ID)

	if !containsString(got1.Needs, m2.ID) {
		t.Errorf("m1.Needs = %v, want to contain %s", got1.Needs, m2.ID)
	}
	if !containsString(got2.Blocks, m1.ID) {
		t.Errorf("m2.Blocks = %v, want to contain %s", got2.Blocks, m1.ID)
	}
}

func TestLinkParent(t *testing.T) {
	s := setupTestStore(t)

	child, _ := s.CreateMatter("Child task", nil)
	parent, _ := s.CreateMatter("Parent epic", nil)

	if err := s.LinkMatters(child.ID, "parent", parent.ID); err != nil {
		t.Fatalf("LinkMatters() error: %v", err)
	}

	got, _ := s.GetMatter(child.ID)
	if got.Parent != parent.ID {
		t.Errorf("Parent = %q, want %q", got.Parent, parent.ID)
	}
}

func TestLinkNoDuplicates(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Feature A", nil)
	m2, _ := s.CreateMatter("Feature B", nil)

	// Link twice
	s.LinkMatters(m1.ID, "relates", m2.ID)
	s.LinkMatters(m1.ID, "relates", m2.ID)

	got1, _ := s.GetMatter(m1.ID)
	got2, _ := s.GetMatter(m2.ID)

	if len(got1.Relates) != 1 {
		t.Errorf("m1.Relates has %d entries, want 1", len(got1.Relates))
	}
	if len(got2.Relates) != 1 {
		t.Errorf("m2.Relates has %d entries, want 1", len(got2.Relates))
	}
}

func TestLinkNonExistentMatter(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Real matter", nil)

	err := s.LinkMatters(m1.ID, "relates", "zzzz")
	if err == nil {
		t.Fatal("expected error when linking to non-existent matter")
	}

	// Verify m1 was not modified
	got, _ := s.GetMatter(m1.ID)
	if len(got.Relates) != 0 {
		t.Errorf("m1.Relates = %v, want empty after failed link", got.Relates)
	}
}

func TestLinkInvalidType(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("A", nil)
	m2, _ := s.CreateMatter("B", nil)

	err := s.LinkMatters(m1.ID, "invalid", m2.ID)
	if err == nil {
		t.Fatal("expected error for invalid relationship type")
	}
}

func TestUnlinkRelates(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Feature A", nil)
	m2, _ := s.CreateMatter("Feature B", nil)

	s.LinkMatters(m1.ID, "relates", m2.ID)

	if err := s.UnlinkMatters(m1.ID, "relates", m2.ID); err != nil {
		t.Fatalf("UnlinkMatters() error: %v", err)
	}

	got1, _ := s.GetMatter(m1.ID)
	got2, _ := s.GetMatter(m2.ID)

	if len(got1.Relates) != 0 {
		t.Errorf("m1.Relates = %v, want empty", got1.Relates)
	}
	if len(got2.Relates) != 0 {
		t.Errorf("m2.Relates = %v, want empty", got2.Relates)
	}
}

func TestUnlinkBlocks(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("Blocker", nil)
	m2, _ := s.CreateMatter("Blocked", nil)

	s.LinkMatters(m1.ID, "blocks", m2.ID)

	if err := s.UnlinkMatters(m1.ID, "blocks", m2.ID); err != nil {
		t.Fatalf("UnlinkMatters() error: %v", err)
	}

	got1, _ := s.GetMatter(m1.ID)
	got2, _ := s.GetMatter(m2.ID)

	if len(got1.Blocks) != 0 {
		t.Errorf("m1.Blocks = %v, want empty", got1.Blocks)
	}
	if len(got2.Needs) != 0 {
		t.Errorf("m2.Needs = %v, want empty", got2.Needs)
	}
}

func TestUnlinkParent(t *testing.T) {
	s := setupTestStore(t)

	child, _ := s.CreateMatter("Child", nil)
	parent, _ := s.CreateMatter("Parent", nil)

	s.LinkMatters(child.ID, "parent", parent.ID)

	if err := s.UnlinkMatters(child.ID, "parent", parent.ID); err != nil {
		t.Fatalf("UnlinkMatters() error: %v", err)
	}

	got, _ := s.GetMatter(child.ID)
	if got.Parent != "" {
		t.Errorf("Parent = %q, want empty", got.Parent)
	}
}

func TestUnlinkNonExistentRelation(t *testing.T) {
	s := setupTestStore(t)

	m1, _ := s.CreateMatter("A", nil)
	m2, _ := s.CreateMatter("B", nil)

	// Unlinking when no link exists should be a no-op, not an error.
	if err := s.UnlinkMatters(m1.ID, "relates", m2.ID); err != nil {
		t.Fatalf("UnlinkMatters() unexpected error: %v", err)
	}
}
