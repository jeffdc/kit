package invoice

import (
	"fmt"
	"io"
	"strings"
	"time"

	"watchmen/internal/model"
)

// InvoiceData holds data needed to generate an invoice
type InvoiceData struct {
	InvoiceNumber        string
	PurchaseOrder        string             // Client's PO number
	Date                 time.Time
	Project              model.Project
	Entries              []model.Entry
	From                 time.Time
	To                   time.Time
	FromContact          *model.ContactInfo // User's contact info
	BillToContact        *model.ContactInfo // Client's billing contact
	Condensed            bool               // If true, show single line item
	CondensedDescription string             // Description for condensed invoice
}

// TotalHours calculates total hours worked
func (d *InvoiceData) TotalHours() float64 {
	var total time.Duration
	for _, e := range d.Entries {
		total += e.Duration()
	}
	return total.Hours()
}

// TotalAmount calculates total billable amount
func (d *InvoiceData) TotalAmount() float64 {
	return d.TotalHours() * d.Project.HourlyRate
}

// GenerateText generates a plain text invoice
func GenerateText(w io.Writer, data *InvoiceData) error {
	fmt.Fprintf(w, "INVOICE\n")
	fmt.Fprintf(w, "%s\n\n", strings.Repeat("=", 60))

	// From section (user's info)
	if data.FromContact != nil && hasContactInfo(data.FromContact) {
		fmt.Fprintf(w, "FROM:\n")
		writeContactText(w, data.FromContact, "  ")
		fmt.Fprintln(w)
	}

	// Bill To section
	if data.BillToContact != nil && hasContactInfo(data.BillToContact) {
		fmt.Fprintf(w, "BILL TO:\n")
		writeContactText(w, data.BillToContact, "  ")
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Invoice #:  %s\n", data.InvoiceNumber)
	if data.PurchaseOrder != "" {
		fmt.Fprintf(w, "PO #:       %s\n", data.PurchaseOrder)
	}
	fmt.Fprintf(w, "Date:       %s\n", data.Date.Format("January 2, 2006"))
	fmt.Fprintf(w, "Period:     %s - %s\n\n",
		data.From.Format("Jan 2, 2006"),
		data.To.Format("Jan 2, 2006"))

	fmt.Fprintf(w, "Project:    %s\n", data.Project.Name)
	if data.Project.Description != "" {
		fmt.Fprintf(w, "            %s\n", data.Project.Description)
	}
	fmt.Fprintf(w, "Rate:       $%.2f/hour\n\n", data.Project.HourlyRate)

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 60))
	fmt.Fprintf(w, "%-12s %8s  %s\n", "DATE", "HOURS", "DESCRIPTION")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 60))

	if data.Condensed {
		// Single line item with total hours and custom description
		period := fmt.Sprintf("%s - %s", data.From.Format("Jan 2"), data.To.Format("Jan 2"))
		desc := data.CondensedDescription
		if desc == "" {
			desc = "Consulting services"
		}
		fmt.Fprintf(w, "%-12s %8.2f  %s\n", period, data.TotalHours(), desc)
	} else {
		for _, e := range data.Entries {
			note := e.Note
			if note == "" {
				note = "-"
			}
			fmt.Fprintf(w, "%-12s %8.2f  %s\n",
				e.StartTime().Format("Jan 2"),
				e.Duration().Hours(),
				note)
		}
	}

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 60))
	fmt.Fprintf(w, "%-12s %8.2f\n\n", "TOTAL HOURS", data.TotalHours())

	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 60))
	fmt.Fprintf(w, "%-48s %10s\n", "TOTAL DUE:", fmt.Sprintf("$%.2f", data.TotalAmount()))
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 60))

	return nil
}

// GenerateMarkdown generates a markdown invoice
func GenerateMarkdown(w io.Writer, data *InvoiceData) error {
	fmt.Fprintf(w, "# Invoice %s\n\n", data.InvoiceNumber)
	if data.PurchaseOrder != "" {
		fmt.Fprintf(w, "**PO #:** %s\n\n", data.PurchaseOrder)
	}
	fmt.Fprintf(w, "**Date:** %s\n\n", data.Date.Format("January 2, 2006"))

	// From section (user's info)
	if data.FromContact != nil && hasContactInfo(data.FromContact) {
		fmt.Fprintf(w, "## From\n\n")
		writeContactMarkdown(w, data.FromContact)
		fmt.Fprintln(w)
	}

	// Bill To section
	if data.BillToContact != nil && hasContactInfo(data.BillToContact) {
		fmt.Fprintf(w, "## Bill To\n\n")
		writeContactMarkdown(w, data.BillToContact)
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "## Details\n\n")
	fmt.Fprintf(w, "- **Project:** %s\n", data.Project.Name)
	if data.Project.Description != "" {
		fmt.Fprintf(w, "- **Description:** %s\n", data.Project.Description)
	}
	fmt.Fprintf(w, "- **Period:** %s - %s\n",
		data.From.Format("Jan 2, 2006"),
		data.To.Format("Jan 2, 2006"))
	fmt.Fprintf(w, "- **Rate:** $%.2f/hour\n\n", data.Project.HourlyRate)

	fmt.Fprintf(w, "## Time Entries\n\n")
	fmt.Fprintf(w, "| Date | Hours | Description |\n")
	fmt.Fprintf(w, "|------|------:|-------------|\n")

	if data.Condensed {
		// Single line item with total hours and custom description
		period := fmt.Sprintf("%s - %s", data.From.Format("Jan 2"), data.To.Format("Jan 2"))
		desc := data.CondensedDescription
		if desc == "" {
			desc = "Consulting services"
		}
		fmt.Fprintf(w, "| %s | %.2f | %s |\n", period, data.TotalHours(), desc)
	} else {
		for _, e := range data.Entries {
			note := e.Note
			if note == "" {
				note = "-"
			}
			fmt.Fprintf(w, "| %s | %.2f | %s |\n",
				e.StartTime().Format("Jan 2"),
				e.Duration().Hours(),
				note)
		}
	}

	fmt.Fprintf(w, "\n## Summary\n\n")
	fmt.Fprintf(w, "| | |\n")
	fmt.Fprintf(w, "|---|---:|\n")
	fmt.Fprintf(w, "| **Total Hours** | %.2f |\n", data.TotalHours())
	fmt.Fprintf(w, "| **Rate** | $%.2f/hr |\n", data.Project.HourlyRate)
	fmt.Fprintf(w, "| **Total Due** | **$%.2f** |\n", data.TotalAmount())

	return nil
}

func hasContactInfo(c *model.ContactInfo) bool {
	return c.Name != "" || c.Title != "" || c.Company != "" || c.Address != "" || c.Phone != "" || c.Email != ""
}

func writeContactText(w io.Writer, c *model.ContactInfo, prefix string) {
	if c.Name != "" {
		fmt.Fprintf(w, "%s%s\n", prefix, c.Name)
	}
	if c.Title != "" {
		fmt.Fprintf(w, "%s%s\n", prefix, c.Title)
	}
	if c.Company != "" {
		fmt.Fprintf(w, "%s%s\n", prefix, c.Company)
	}
	if c.Address != "" {
		fmt.Fprintf(w, "%s%s\n", prefix, c.Address)
	}
	if c.Phone != "" {
		fmt.Fprintf(w, "%s%s\n", prefix, c.Phone)
	}
	if c.Email != "" {
		fmt.Fprintf(w, "%s%s\n", prefix, c.Email)
	}
}

func writeContactMarkdown(w io.Writer, c *model.ContactInfo) {
	if c.Name != "" {
		fmt.Fprintf(w, "**%s**\n", c.Name)
	}
	if c.Title != "" {
		fmt.Fprintf(w, "*%s*\n", c.Title)
	}
	if c.Company != "" {
		fmt.Fprintf(w, "%s\n", c.Company)
	}
	if c.Address != "" {
		fmt.Fprintf(w, "%s\n", c.Address)
	}
	if c.Phone != "" {
		fmt.Fprintf(w, "%s\n", c.Phone)
	}
	if c.Email != "" {
		fmt.Fprintf(w, "%s\n", c.Email)
	}
}
