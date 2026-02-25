package cmd

import "testing"

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"30d", 30, false},
		{"60d", 60, false},
		{"7d", 7, false},
		{"90", 90, false},
		{"", 30, false},
		{"-1d", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		got, err := parseDuration(tt.input)
		if tt.err && err == nil {
			t.Errorf("parseDuration(%q) expected error, got %d", tt.input, got)
		}
		if !tt.err && err != nil {
			t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("parseDuration(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
