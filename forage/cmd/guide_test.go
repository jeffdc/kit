package cmd

import (
	"testing"
)

func TestGuideContainsKeyInfo(t *testing.T) {
	guide := guideText()

	required := []string{
		// Statuses
		"wishlist", "owned", "reading", "paused", "read", "dropped",
		// Settable fields
		"title", "author", "status", "rating", "tags", "date_read",
		// Commands
		"forage add", "forage list", "forage show", "forage set",
		"forage search", "forage read", "forage drop", "forage prime",
		"forage export", "forage import",
		// Workflow concepts
		"Quick Start",
		// LLM section
		"LLM",
	}

	for _, s := range required {
		if !contains(guide, s) {
			t.Errorf("guide missing required content: %q", s)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		// simple substring check
		stringContains(haystack, needle)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
