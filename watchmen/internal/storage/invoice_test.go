package storage

import (
	"testing"
	"time"

	"watchmen/internal/model"
)

func TestMarkInvoicePaidUsesProvidedDate(t *testing.T) {
	store, _ := setupTestStore(t)

	invoice := &model.Invoice{
		ID:          "INV-test-1",
		ProjectID:   "proj1",
		ProjectName: "Test Project",
		CreatedAt:   time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC),
		PeriodStart: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		Hours:       10,
		Rate:        100,
		Amount:      1000,
		Status:      model.InvoiceStatusPending,
	}
	if err := store.SaveInvoice(invoice); err != nil {
		t.Fatalf("SaveInvoice failed: %v", err)
	}

	paidAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	updated, err := store.MarkInvoicePaid(invoice.ID, &paidAt)
	if err != nil {
		t.Fatalf("MarkInvoicePaid failed: %v", err)
	}

	if updated.Status != model.InvoiceStatusPaid {
		t.Fatalf("Status = %q, want %q", updated.Status, model.InvoiceStatusPaid)
	}
	if updated.PaidAt == nil {
		t.Fatal("PaidAt was nil")
	}
	if !updated.PaidAt.Equal(paidAt) {
		t.Fatalf("PaidAt = %s, want %s", updated.PaidAt.Format(time.RFC3339), paidAt.Format(time.RFC3339))
	}

	reloaded, err := store.GetInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if reloaded.PaidAt == nil || !reloaded.PaidAt.Equal(paidAt) {
		t.Fatalf("persisted PaidAt = %v, want %s", reloaded.PaidAt, paidAt.Format(time.RFC3339))
	}
}
