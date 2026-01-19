package model

import "time"

// ContactInfo holds contact details for invoicing
type ContactInfo struct {
	Name    string `json:"name,omitempty"`
	Title   string `json:"title,omitempty"`
	Company string `json:"company,omitempty"`
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Email   string `json:"email,omitempty"`
}

// Project represents a client project
type Project struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	HourlyRate     float64      `json:"hourly_rate"`
	Description    string       `json:"description,omitempty"`
	BillingContact *ContactInfo `json:"billing_contact,omitempty"`
	PurchaseOrder  string       `json:"purchase_order,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

// TimeSegment represents a continuous period of work
type TimeSegment struct {
	Start time.Time  `json:"start"`
	End   *time.Time `json:"end,omitempty"`
}

// Duration returns the duration of the segment
func (s *TimeSegment) Duration() time.Duration {
	if s.End == nil {
		return time.Since(s.Start)
	}
	return s.End.Sub(s.Start)
}

// Entry represents a time entry with one or more segments
type Entry struct {
	ID        string        `json:"id"`
	ProjectID string        `json:"project_id"`
	Note      string        `json:"note,omitempty"`
	Segments  []TimeSegment `json:"segments"`
	Completed bool          `json:"completed,omitempty"`
}

// Duration returns the total duration across all segments
func (e *Entry) Duration() time.Duration {
	var total time.Duration
	for _, seg := range e.Segments {
		total += seg.Duration()
	}
	return total
}

// IsRunning returns true if the entry is actively tracking (not paused, not completed)
func (e *Entry) IsRunning() bool {
	if e.Completed || len(e.Segments) == 0 {
		return false
	}
	return e.Segments[len(e.Segments)-1].End == nil
}

// IsPaused returns true if the entry is paused (can be resumed)
func (e *Entry) IsPaused() bool {
	if e.Completed || len(e.Segments) == 0 {
		return false
	}
	return e.Segments[len(e.Segments)-1].End != nil
}

// StartTime returns the start time of the first segment
func (e *Entry) StartTime() time.Time {
	if len(e.Segments) == 0 {
		return time.Time{}
	}
	return e.Segments[0].Start
}

// Settings holds user configuration
type Settings struct {
	UserContact *ContactInfo `json:"user_contact,omitempty"`
}

// InvoiceStatus represents the payment status of an invoice
type InvoiceStatus string

const (
	InvoiceStatusPending InvoiceStatus = "pending"
	InvoiceStatusPaid    InvoiceStatus = "paid"
)

// Invoice represents a generated invoice record
type Invoice struct {
	ID          string        `json:"id"`
	ProjectID   string        `json:"project_id"`
	ProjectName string        `json:"project_name"`
	CreatedAt   time.Time     `json:"created_at"`
	PeriodStart time.Time     `json:"period_start"`
	PeriodEnd   time.Time     `json:"period_end"`
	Hours       float64       `json:"hours"`
	Rate        float64       `json:"rate"`
	Amount      float64       `json:"amount"`
	Status      InvoiceStatus `json:"status"`
	PaidAt      *time.Time    `json:"paid_at,omitempty"`
	Description string        `json:"description,omitempty"`
	Condensed   bool          `json:"condensed,omitempty"`
}

// Data is the root structure for JSON storage
type Data struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Entries  []Entry   `json:"entries"`
	Invoices []Invoice `json:"invoices,omitempty"`
	Settings *Settings `json:"settings,omitempty"`
}
