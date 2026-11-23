package mull

import (
	"testing"
	"time"
)

// The backlog lists what was captured last at the top, whatever its
// category.
func TestBacklogNewestFirst(t *testing.T) {
	m, _ := setup(t)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	at := func(min int) { m.Now = func() time.Time { return base.Add(time.Duration(min) * time.Minute) } }
	capture := func(title, category string) string {
		t.Helper()
		return must[ItemView](t)(m.CaptureItem(NewCapture{Kind: "bug", Title: title, Category: category})).ID
	}
	at(0)
	old := capture("Old, with a category", "ui")
	at(1)
	mid := capture("Captured in between", "")
	at(2)
	fresh := capture("Just captured, no category", "")

	bl := must[Backlog](t)(m.BacklogPage(""))
	var got []string
	for _, r := range bl.Items {
		got = append(got, r.ID)
	}
	want := []string{fresh, mid, old}
	if len(got) != len(want) {
		t.Fatalf("backlog = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("backlog = %v, want %v (newest first)", got, want)
		}
	}
}
