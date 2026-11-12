package guide

import "testing"

func TestPersonal(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Only the human commits.", "Only Jeff commits."},
		{"The human agrees it; the human's call.", "Jeff agrees it; Jeff's call."},
		{"ask the human: then wait (the human)", "ask Jeff: then wait (Jeff)"},
		{"the human-feedback rule stays", "the human-feedback rule stays"},
		{"a human rendering, the humans", "a human rendering, the humans"},
		{"end with the human", "end with Jeff"},
	} {
		if got := Personal(c.in, "Jeff"); got != c.want {
			t.Errorf("Personal(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := Personal("the human", " "); got != "the human" {
		t.Errorf("no name changed the text: %q", got)
	}
	if NameRule("") != "" || NameRule("Jeff") == "" {
		t.Error("NameRule")
	}
}
