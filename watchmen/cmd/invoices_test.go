package cmd

import (
	"strings"
	"testing"
	"time"

	"watchmen/internal/model"
	"watchmen/internal/storage"
)

func TestInvoicesPaidCommandRejectsInvalidDate(t *testing.T) {
	cmd := *invoicesPaidCmd
	if err := cmd.Flags().Set("paid-at", "05/01/2026"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	err := cmd.RunE(&cmd, []string{"INV-test-1"})
	if err == nil {
		t.Fatal("expected error for invalid --paid-at date")
	}
	if !strings.Contains(err.Error(), "invalid date format for --paid-at, use YYYY-MM-DD") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvoicesPaidCommandSetsProvidedDate(t *testing.T) {
	tmpStore, _ := setupPaidInvoiceTestStore(t)
	store = tmpStore

	invoice := &model.Invoice{
		ID:          "INV-test-2",
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

	cmd := *invoicesPaidCmd
	if err := cmd.Flags().Set("paid-at", "2026-05-01"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	if err := cmd.RunE(&cmd, []string{invoice.ID}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	updated, err := store.GetInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}

	want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if updated.PaidAt == nil || !updated.PaidAt.Equal(want) {
		t.Fatalf("PaidAt = %v, want %s", updated.PaidAt, want.Format(time.RFC3339))
	}
}

func setupPaidInvoiceTestStore(t *testing.T) (*storage.Store, string) {
	t.Helper()

	tmpDir := t.TempDir()
	path := tmpDir + "/test_data.json"
	store, err := storage.New(path)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return store, path
}
