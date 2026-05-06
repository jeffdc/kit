package cmd

import (
	"testing"
	"time"
)

func TestFormatStartedAt(t *testing.T) {
	loc := time.FixedZone("EDT", -4*60*60)

	tests := []struct {
		name      string
		startedAt time.Time
		now       time.Time
		want      string
	}{
		{
			name:      "same day shows time only",
			startedAt: time.Date(2026, 4, 25, 8, 0, 0, 0, loc),
			now:       time.Date(2026, 4, 25, 21, 31, 56, 0, loc),
			want:      "8:00 AM",
		},
		{
			name:      "different day shows date and time",
			startedAt: time.Date(2026, 4, 24, 8, 0, 0, 0, loc),
			now:       time.Date(2026, 4, 25, 21, 31, 56, 0, loc),
			want:      "Apr 24, 2026 8:00 AM",
		},
		{
			name:      "zero time returns empty string",
			startedAt: time.Time{},
			now:       time.Date(2026, 4, 25, 21, 31, 56, 0, loc),
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatStartedAt(tt.startedAt, tt.now); got != tt.want {
				t.Fatalf("formatStartedAt() = %q, want %q", got, tt.want)
			}
		})
	}
}
