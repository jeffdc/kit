package cmd

import (
	"testing"
	"time"
)

func TestParseMilitaryTime(t *testing.T) {
	tests := []struct {
		input    string
		wantHour int
		wantMin  int
		wantOk   bool
	}{
		{"1700", 17, 0, true},
		{"2330", 23, 30, true},
		{"0900", 9, 0, true},
		{"900", 9, 0, true},
		{"0000", 0, 0, true},
		{"2359", 23, 59, true},
		{"1234", 12, 34, true},
		// Invalid cases
		{"2400", 0, 0, false},  // hour too high
		{"1260", 0, 0, false},  // minute too high
		{"12345", 0, 0, false}, // too many digits
		{"12", 0, 0, false},    // too few digits
		{"12:00", 0, 0, false}, // contains colon
		{"9am", 0, 0, false},   // contains letters
		{"", 0, 0, false},      // empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			hour, min, ok := parseMilitaryTime(tt.input)
			if ok != tt.wantOk {
				t.Errorf("parseMilitaryTime(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
				return
			}
			if ok && (hour != tt.wantHour || min != tt.wantMin) {
				t.Errorf("parseMilitaryTime(%q) = %d:%02d, want %d:%02d", tt.input, hour, min, tt.wantHour, tt.wantMin)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	baseDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.Local)

	tests := []struct {
		input    string
		wantHour int
		wantMin  int
		wantErr  bool
	}{
		// AM/PM lowercase
		{"9am", 9, 0, false},
		{"9pm", 21, 0, false},
		{"12am", 0, 0, false},
		{"12pm", 12, 0, false},
		// AM/PM uppercase
		{"9AM", 9, 0, false},
		{"9PM", 21, 0, false},
		{"12AM", 0, 0, false},
		{"12PM", 12, 0, false},
		// With minutes
		{"9:30am", 9, 30, false},
		{"9:30pm", 21, 30, false},
		{"9:30AM", 9, 30, false},
		{"9:30PM", 21, 30, false},
		// With space
		{"9 am", 9, 0, false},
		{"9 pm", 21, 0, false},
		{"9 AM", 9, 0, false},
		{"9 PM", 21, 0, false},
		{"9:30 am", 9, 30, false},
		{"9:30 pm", 21, 30, false},
		// 24-hour with colon
		{"14:30", 14, 30, false},
		{"09:00", 9, 0, false},
		{"00:00", 0, 0, false},
		{"23:59", 23, 59, false},
		// Military time (no colon)
		{"1700", 17, 0, false},
		{"2330", 23, 30, false},
		{"0900", 9, 0, false},
		{"900", 9, 0, false},
		// With whitespace
		{" 9am ", 9, 0, false},
		{" 1700 ", 17, 0, false},
		// Invalid
		{"invalid", 0, 0, true},
		{"25:00", 0, 0, true},
		{"", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseTime(tt.input, baseDate)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err == nil {
				if result.Hour() != tt.wantHour || result.Minute() != tt.wantMin {
					t.Errorf("parseTime(%q) = %d:%02d, want %d:%02d", tt.input, result.Hour(), result.Minute(), tt.wantHour, tt.wantMin)
				}
				if result.Year() != 2024 || result.Month() != 6 || result.Day() != 15 {
					t.Errorf("parseTime(%q) date = %v, want 2024-06-15", tt.input, result.Format("2006-01-02"))
				}
			}
		})
	}
}
