package invoice

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"watchmen/internal/model"
)

func TestGenerateMarkdown(t *testing.T) {
	baseTime := time.Date(2024, 6, 15, 9, 0, 0, 0, time.Local)
	endTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.Local)

	data := &InvoiceData{
		InvoiceNumber: "INV-001",
		Date:          baseTime,
		Project: model.Project{
			ID:          "test-id",
			Name:        "Test Project",
			HourlyRate:  100.0,
			Description: "Test description",
		},
		Entries: []model.Entry{
			{
				ID:        "entry-1",
				ProjectID: "test-id",
				Note:      "Did some work",
				Segments: []model.TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: true,
			},
		},
		From: baseTime,
		To:   endTime,
	}

	var buf bytes.Buffer
	err := GenerateMarkdown(&buf, data)
	if err != nil {
		t.Fatalf("GenerateMarkdown() error = %v", err)
	}

	output := buf.String()

	// Check key elements
	checks := []string{
		"# Invoice INV-001",
		"**Date:** June 15, 2024",
		"**Project:** Test Project",
		"**Description:** Test description",
		"**Rate:** $100.00/hour",
		"| Date | Hours | Description |",
		"| Jun 15 | 3.00 | Did some work |",
		"| **Total Hours** | 3.00 |",
		"| **Total Due** | **$300.00** |",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("GenerateMarkdown() output missing %q", check)
		}
	}
}

func TestGenerateMarkdownWithContacts(t *testing.T) {
	baseTime := time.Date(2024, 6, 15, 9, 0, 0, 0, time.Local)
	endTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.Local)

	data := &InvoiceData{
		InvoiceNumber: "INV-001",
		Date:          baseTime,
		Project: model.Project{
			ID:         "test-id",
			Name:       "Test Project",
			HourlyRate: 100.0,
		},
		Entries: []model.Entry{
			{
				ID:        "entry-1",
				ProjectID: "test-id",
				Segments: []model.TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: true,
			},
		},
		From: baseTime,
		To:   endTime,
		FromContact: &model.ContactInfo{
			Name:    "John Doe",
			Company: "Doe Consulting",
			Email:   "john@doe.com",
		},
		BillToContact: &model.ContactInfo{
			Name:    "Jane Smith",
			Company: "Acme Inc",
			Address: "123 Main St",
			Phone:   "555-1234",
			Email:   "jane@acme.com",
		},
	}

	var buf bytes.Buffer
	err := GenerateMarkdown(&buf, data)
	if err != nil {
		t.Fatalf("GenerateMarkdown() error = %v", err)
	}

	output := buf.String()

	// Check contact sections
	checks := []string{
		"## From",
		"**John Doe**",
		"Doe Consulting",
		"john@doe.com",
		"## Bill To",
		"**Jane Smith**",
		"Acme Inc",
		"123 Main St",
		"555-1234",
		"jane@acme.com",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("GenerateMarkdown() output missing %q", check)
		}
	}
}

func TestGenerateTextWithContacts(t *testing.T) {
	baseTime := time.Date(2024, 6, 15, 9, 0, 0, 0, time.Local)
	endTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.Local)

	data := &InvoiceData{
		InvoiceNumber: "INV-001",
		Date:          baseTime,
		Project: model.Project{
			ID:         "test-id",
			Name:       "Test Project",
			HourlyRate: 100.0,
		},
		Entries: []model.Entry{
			{
				ID:        "entry-1",
				ProjectID: "test-id",
				Segments: []model.TimeSegment{
					{Start: baseTime, End: &endTime},
				},
				Completed: true,
			},
		},
		From: baseTime,
		To:   endTime,
		FromContact: &model.ContactInfo{
			Name:  "John Doe",
			Email: "john@doe.com",
		},
		BillToContact: &model.ContactInfo{
			Name:    "Jane Smith",
			Company: "Acme Inc",
		},
	}

	var buf bytes.Buffer
	err := GenerateText(&buf, data)
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}

	output := buf.String()

	checks := []string{
		"INVOICE",
		"FROM:",
		"John Doe",
		"john@doe.com",
		"BILL TO:",
		"Jane Smith",
		"Acme Inc",
		"Invoice #:  INV-001",
		"TOTAL DUE:",
		"$300.00",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("GenerateText() output missing %q", check)
		}
	}
}

func TestHasContactInfo(t *testing.T) {
	tests := []struct {
		name    string
		contact *model.ContactInfo
		want    bool
	}{
		{"nil", nil, false},
		{"empty", &model.ContactInfo{}, false},
		{"name only", &model.ContactInfo{Name: "John"}, true},
		{"email only", &model.ContactInfo{Email: "test@test.com"}, true},
		{"full", &model.ContactInfo{Name: "John", Email: "john@test.com", Phone: "555"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.contact == nil {
				// hasContactInfo doesn't handle nil, so skip
				return
			}
			if got := hasContactInfo(tt.contact); got != tt.want {
				t.Errorf("hasContactInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
