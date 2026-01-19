package invoice

import (
	"fmt"

	"github.com/jung-kurt/gofpdf"
	"watchmen/internal/model"
)

// GeneratePDF generates a PDF invoice
func GeneratePDF(filename string, data *InvoiceData) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 24)
	pdf.Cell(0, 15, "INVOICE")
	pdf.Ln(20)

	// From and Bill To sections side by side
	if (data.FromContact != nil && hasContactInfo(data.FromContact)) ||
		(data.BillToContact != nil && hasContactInfo(data.BillToContact)) {

		startY := pdf.GetY()

		// From section (left side)
		if data.FromContact != nil && hasContactInfo(data.FromContact) {
			pdf.SetFont("Arial", "B", 10)
			pdf.Cell(90, 6, "FROM:")
			pdf.Ln(6)
			pdf.SetFont("Arial", "", 10)
			writeContactPDF(pdf, data.FromContact)
		}

		// Bill To section (right side)
		if data.BillToContact != nil && hasContactInfo(data.BillToContact) {
			pdf.SetXY(105, startY)
			pdf.SetFont("Arial", "B", 10)
			pdf.Cell(90, 6, "BILL TO:")
			pdf.SetXY(105, startY+6)
			pdf.SetFont("Arial", "", 10)
			writeContactPDFAt(pdf, data.BillToContact, 105)
		}

		pdf.Ln(10)
	}

	// Invoice details
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(30, 6, "Invoice #:")
	pdf.Cell(0, 6, data.InvoiceNumber)
	pdf.Ln(6)

	if data.PurchaseOrder != "" {
		pdf.Cell(30, 6, "PO #:")
		pdf.Cell(0, 6, data.PurchaseOrder)
		pdf.Ln(6)
	}

	pdf.Cell(30, 6, "Date:")
	pdf.Cell(0, 6, data.Date.Format("January 2, 2006"))
	pdf.Ln(6)

	pdf.Cell(30, 6, "Period:")
	pdf.Cell(0, 6, fmt.Sprintf("%s - %s", data.From.Format("Jan 2, 2006"), data.To.Format("Jan 2, 2006")))
	pdf.Ln(12)

	// Project details
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(30, 6, "Project:")
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(0, 6, data.Project.Name)
	pdf.Ln(6)

	if data.Project.Description != "" {
		pdf.Cell(30, 6, "")
		pdf.SetFont("Arial", "I", 10)
		pdf.Cell(0, 6, data.Project.Description)
		pdf.SetFont("Arial", "", 11)
		pdf.Ln(6)
	}

	pdf.Cell(30, 6, "Rate:")
	pdf.Cell(0, 6, fmt.Sprintf("$%.2f/hour", data.Project.HourlyRate))
	pdf.Ln(15)

	// Table header
	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 8, "DATE", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "HOURS", "1", 0, "R", true, 0, "")
	pdf.CellFormat(0, 8, "DESCRIPTION", "1", 1, "L", true, 0, "")

	// Table rows
	pdf.SetFont("Arial", "", 10)
	pageWidth, pageHeight := pdf.GetPageSize()
	marginLeft, _, marginRight, marginBottom := pdf.GetMargins()
	descWidth := pageWidth - marginLeft - marginRight - 30 - 25 // remaining width for description

	// Helper function to draw table header
	drawTableHeader := func() {
		pdf.SetFillColor(240, 240, 240)
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(30, 8, "DATE", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 8, "HOURS", "1", 0, "R", true, 0, "")
		pdf.CellFormat(0, 8, "DESCRIPTION", "1", 1, "L", true, 0, "")
		pdf.SetFont("Arial", "", 10)
	}

	// Helper function to draw a single table row
	drawRow := func(dateStr string, hours float64, note string) {
		// Calculate height needed for the note text
		lines := pdf.SplitText(note, descWidth)
		lineHeight := 5.0
		cellHeight := float64(len(lines)) * lineHeight
		if cellHeight < 7 {
			cellHeight = 7
		}

		// Check if we need a new page before drawing this row
		_, y := pdf.GetXY()
		if y+cellHeight > pageHeight-marginBottom {
			pdf.AddPage()
			drawTableHeader()
		}

		// Get current position (may have changed after page break)
		x, y := pdf.GetXY()

		// Draw date cell
		pdf.CellFormat(30, cellHeight, dateStr, "1", 0, "L", false, 0, "")

		// Draw hours cell
		pdf.CellFormat(25, cellHeight, fmt.Sprintf("%.2f", hours), "1", 0, "R", false, 0, "")

		// Draw description cell with MultiCell for wrapping
		descX := x + 30 + 25
		pdf.SetXY(descX, y)
		// Draw border manually since MultiCell doesn't handle it well in this context
		pdf.Rect(descX, y, descWidth, cellHeight, "D")
		pdf.SetXY(descX+1, y+1) // Small padding
		pdf.MultiCell(descWidth-2, lineHeight, note, "", "L", false)

		// Move to next row
		pdf.SetXY(x, y+cellHeight)
	}

	if data.Condensed {
		// Single line item with total hours and custom description
		period := fmt.Sprintf("%s - %s", data.From.Format("Jan 2"), data.To.Format("Jan 2"))
		desc := data.CondensedDescription
		if desc == "" {
			desc = "Consulting services"
		}
		drawRow(period, data.TotalHours(), desc)
	} else {
		for _, e := range data.Entries {
			note := e.Note
			if note == "" {
				note = "-"
			}
			drawRow(e.StartTime().Format("Jan 2"), e.Duration().Hours(), note)
		}
	}

	// Total hours
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(30, 8, "TOTAL", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, fmt.Sprintf("%.2f", data.TotalHours()), "1", 0, "R", true, 0, "")
	pdf.CellFormat(0, 8, "", "1", 1, "L", true, 0, "")

	pdf.Ln(10)

	// Total due
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(140, 10, "TOTAL DUE:")
	pdf.Cell(0, 10, fmt.Sprintf("$%.2f", data.TotalAmount()))

	return pdf.OutputFileAndClose(filename)
}

func writeContactPDF(pdf *gofpdf.Fpdf, c *model.ContactInfo) {
	if c.Name != "" {
		pdf.Cell(90, 5, c.Name)
		pdf.Ln(5)
	}
	if c.Company != "" {
		pdf.Cell(90, 5, c.Company)
		pdf.Ln(5)
	}
	if c.Address != "" {
		pdf.Cell(90, 5, c.Address)
		pdf.Ln(5)
	}
	if c.Phone != "" {
		pdf.Cell(90, 5, c.Phone)
		pdf.Ln(5)
	}
	if c.Email != "" {
		pdf.Cell(90, 5, c.Email)
		pdf.Ln(5)
	}
}

func writeContactPDFAt(pdf *gofpdf.Fpdf, c *model.ContactInfo, x float64) {
	y := pdf.GetY()
	if c.Name != "" {
		pdf.SetXY(x, y)
		pdf.Cell(90, 5, c.Name)
		y += 5
	}
	if c.Company != "" {
		pdf.SetXY(x, y)
		pdf.Cell(90, 5, c.Company)
		y += 5
	}
	if c.Address != "" {
		pdf.SetXY(x, y)
		pdf.Cell(90, 5, c.Address)
		y += 5
	}
	if c.Phone != "" {
		pdf.SetXY(x, y)
		pdf.Cell(90, 5, c.Phone)
		y += 5
	}
	if c.Email != "" {
		pdf.SetXY(x, y)
		pdf.Cell(90, 5, c.Email)
		y += 5
	}
	pdf.SetY(y)
}
