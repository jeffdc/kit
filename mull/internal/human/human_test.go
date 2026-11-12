package human

import "testing"

func TestName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if n, err := Name(); err != nil || n != "" {
		t.Fatalf("unset name = %q, %v", n, err)
	}
	if err := SetName("  Jeff "); err != nil {
		t.Fatal(err)
	}
	if n, _ := Name(); n != "Jeff" {
		t.Fatalf("name = %q", n)
	}
	if err := SetName("two\nlines"); err == nil {
		t.Fatal("a two-line name was accepted")
	}
	if err := SetName(""); err != nil {
		t.Fatal(err)
	}
	if n, _ := Name(); n != "" {
		t.Fatalf("cleared name = %q", n)
	}
}
