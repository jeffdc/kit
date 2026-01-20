package invoice

import (
	"fmt"
	"io"
	"sort"
	"time"

	"watchmen/internal/model"
)

// ReportData holds data needed to generate a stakeholder report
type ReportData struct {
	ProjectName string
	From        time.Time
	To          time.Time
	TotalHours  float64
	InvoiceRef  string // Optional invoice reference
	Entries     []model.Entry
}

// GenerateReport generates a markdown stakeholder report
func GenerateReport(w io.Writer, data *ReportData) error {
	fmt.Fprintf(w, "# Stakeholder Report: %s\n\n", data.ProjectName)
	fmt.Fprintf(w, "**Period:** %s - %s\n", data.From.Format("Jan 2, 2006"), data.To.Format("Jan 2, 2006"))
	fmt.Fprintf(w, "**Hours:** %.2f\n", data.TotalHours)
	if data.InvoiceRef != "" {
		fmt.Fprintf(w, "**Invoice:** %s\n", data.InvoiceRef)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "## Work Completed\n")

	// Group entries by date
	entriesByDate := make(map[string][]model.Entry)
	for _, e := range data.Entries {
		dateKey := e.StartTime().Format("2006-01-02")
		entriesByDate[dateKey] = append(entriesByDate[dateKey], e)
	}

	// Sort dates
	var dates []string
	for date := range entriesByDate {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	for _, dateKey := range dates {
		entries := entriesByDate[dateKey]
		date, _ := time.Parse("2006-01-02", dateKey)
		fmt.Fprintf(w, "\n### %s\n", date.Format("January 2, 2006"))
		for _, e := range entries {
			if e.Note != "" {
				fmt.Fprintf(w, "- %s\n", e.Note)
			}
		}
	}

	return nil
}
