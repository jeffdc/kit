package model

import "testing"

func TestValidStatuses(t *testing.T) {
	valid := []string{"wishlist", "reading", "paused", "read", "dropped"}
	for _, s := range valid {
		if !ValidStatus(s) {
			t.Errorf("expected %q to be a valid status", s)
		}
	}

	invalid := []string{"", "done", "active", "foo"}
	for _, s := range invalid {
		if ValidStatus(s) {
			t.Errorf("expected %q to be an invalid status", s)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	if !IsTerminal("dropped") {
		t.Error("expected dropped to be terminal")
	}
	for _, s := range []string{"wishlist", "reading", "paused", "read"} {
		if IsTerminal(s) {
			t.Errorf("expected %q to not be terminal", s)
		}
	}
}
