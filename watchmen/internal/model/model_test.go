package model

import (
	"testing"
	"time"
)

func TestTimeSegmentDuration(t *testing.T) {
	start := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)
	end := time.Date(2024, 1, 15, 11, 30, 0, 0, time.Local)

	tests := []struct {
		name    string
		segment TimeSegment
		want    time.Duration
	}{
		{
			name:    "completed segment",
			segment: TimeSegment{Start: start, End: &end},
			want:    2*time.Hour + 30*time.Minute,
		},
		{
			name:    "running segment returns time since start",
			segment: TimeSegment{Start: time.Now().Add(-1 * time.Hour), End: nil},
			want:    1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.segment.Duration()
			// Allow 1 second tolerance for running segment
			if tt.segment.End == nil {
				if got < tt.want-time.Second || got > tt.want+time.Second {
					t.Errorf("TimeSegment.Duration() = %v, want approximately %v", got, tt.want)
				}
			} else {
				if got != tt.want {
					t.Errorf("TimeSegment.Duration() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestEntryDuration(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)
	end1 := baseTime.Add(2 * time.Hour)
	start2 := baseTime.Add(3 * time.Hour)
	end2 := baseTime.Add(5 * time.Hour)

	tests := []struct {
		name  string
		entry Entry
		want  time.Duration
	}{
		{
			name: "single completed segment",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &end1},
				},
				Completed: true,
			},
			want: 2 * time.Hour,
		},
		{
			name: "multiple completed segments",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &end1},
					{Start: start2, End: &end2},
				},
				Completed: true,
			},
			want: 4 * time.Hour, // 2h + 2h
		},
		{
			name: "empty segments",
			entry: Entry{
				Segments:  []TimeSegment{},
				Completed: false,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.Duration()
			if got != tt.want {
				t.Errorf("Entry.Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntryIsRunning(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)
	endTime := baseTime.Add(2 * time.Hour)

	tests := []struct {
		name  string
		entry Entry
		want  bool
	}{
		{
			name: "running - last segment has no end",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: nil},
				},
				Completed: false,
			},
			want: true,
		},
		{
			name: "paused - last segment has end, not completed",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: false,
			},
			want: false,
		},
		{
			name: "completed entry",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: true,
			},
			want: false,
		},
		{
			name: "empty segments",
			entry: Entry{
				Segments:  []TimeSegment{},
				Completed: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.IsRunning(); got != tt.want {
				t.Errorf("Entry.IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntryIsPaused(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)
	endTime := baseTime.Add(2 * time.Hour)

	tests := []struct {
		name  string
		entry Entry
		want  bool
	}{
		{
			name: "paused - last segment has end, not completed",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: false,
			},
			want: true,
		},
		{
			name: "running - last segment has no end",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: nil},
				},
				Completed: false,
			},
			want: false,
		},
		{
			name: "completed entry",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: true,
			},
			want: false,
		},
		{
			name: "empty segments",
			entry: Entry{
				Segments:  []TimeSegment{},
				Completed: false,
			},
			want: false,
		},
		{
			name: "multiple segments, last paused",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
					{Start: endTime.Add(time.Hour), End: &endTime},
				},
				Completed: false,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.IsPaused(); got != tt.want {
				t.Errorf("Entry.IsPaused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntryStartTime(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.Local)
	endTime := baseTime.Add(2 * time.Hour)

	tests := []struct {
		name  string
		entry Entry
		want  time.Time
	}{
		{
			name: "single segment",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
				},
			},
			want: baseTime,
		},
		{
			name: "multiple segments returns first",
			entry: Entry{
				Segments: []TimeSegment{
					{Start: baseTime, End: &endTime},
					{Start: endTime.Add(time.Hour), End: nil},
				},
			},
			want: baseTime,
		},
		{
			name: "empty segments returns zero time",
			entry: Entry{
				Segments: []TimeSegment{},
			},
			want: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.StartTime(); !got.Equal(tt.want) {
				t.Errorf("Entry.StartTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
