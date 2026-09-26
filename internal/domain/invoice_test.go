package domain_test

import (
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

const validMerchant = "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
const validTx = "4B8F9A1C2D3E4F5A6B7C8D9E0F1A2B3C4D5E6F7A8B9C0D1E2F3A4B5C6D7E8F9"

func TestNewInvoice_Success(t *testing.T) {
	drops, _ := domain.ParseXRP("10.5")
	meta := map[string]string{"customer_email": "alice@example.com"}

	inv, err := domain.NewInvoice("order_100", validMerchant, 1001, drops, 15*time.Minute, "https://example.com/webhook", meta)
	if err != nil {
		t.Fatalf("NewInvoice unexpected error: %v", err)
	}

	if inv.OrderID != "order_100" {
		t.Errorf("expected order_id 'order_100', got %q", inv.OrderID)
	}
	if inv.Status != domain.StatusPending {
		t.Errorf("expected status %s, got %s", domain.StatusPending, inv.Status)
	}
	if inv.AmountExpected.Int64() != 10_500_000 {
		t.Errorf("expected 10500000 drops, got %d", inv.AmountExpected.Int64())
	}
	if inv.AmountPaid.Int64() != 0 {
		t.Errorf("expected 0 drops paid, got %d", inv.AmountPaid.Int64())
	}
	if inv.Version != 1 {
		t.Errorf("expected initial version 1, got %d", inv.Version)
	}
	if inv.Metadata["customer_email"] != "alice@example.com" {
		t.Errorf("metadata missing")
	}
}

func TestNewInvoice_ValidationErrors(t *testing.T) {
	drops, _ := domain.ParseXRP("1")

	tests := []struct {
		name         string
		merchantAddr string
		tag          uint32
		amount       domain.Drops
		duration     time.Duration
	}{
		{name: "invalid prefix", merchantAddr: "1BitcoinAddressFormatNotXRP123", tag: 1, amount: drops, duration: time.Minute},
		{name: "too short", merchantAddr: "rShort", tag: 1, amount: drops, duration: time.Minute},
		{name: "zero tag", merchantAddr: validMerchant, tag: 0, amount: drops, duration: time.Minute},
		{name: "zero amount", merchantAddr: validMerchant, tag: 1, amount: 0, duration: time.Minute},
		{name: "zero duration", merchantAddr: validMerchant, tag: 1, amount: drops, duration: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewInvoice("order_err", tt.merchantAddr, tt.tag, tt.amount, tt.duration, "", nil)
			if err == nil {
				t.Errorf("expected validation error for %s, got nil", tt.name)
			}
		})
	}
}

func TestInvoice_ApplyPayment_Scenarios(t *testing.T) {
	dropsExpected, _ := domain.ParseXRP("10") // 10 XRP = 10,000,000 drops

	t.Run("Exact Settle (10 XRP -> Settled)", func(t *testing.T) {
		inv, _ := domain.NewInvoice("o1", validMerchant, 101, dropsExpected, 10*time.Minute, "", nil)
		settleTime := time.Now().UTC()

		if err := inv.ApplyPayment(dropsExpected, validTx, settleTime); err != nil {
			t.Fatalf("ApplyPayment error: %v", err)
		}

		if inv.Status != domain.StatusSettled {
			t.Errorf("expected %s, got %s", domain.StatusSettled, inv.Status)
		}
		if inv.AmountPaid != dropsExpected {
			t.Errorf("expected paid %d, got %d", dropsExpected.Int64(), inv.AmountPaid.Int64())
		}
		if inv.Version != 2 {
			t.Errorf("expected version 2, got %d", inv.Version)
		}
		if !inv.IsTerminal() {
			t.Error("settled invoice must be terminal")
		}
	})

	t.Run("Partial Payment (6 XRP -> PartiallyPaid, then +4 XRP -> Settled)", func(t *testing.T) {
		inv, _ := domain.NewInvoice("o2", validMerchant, 102, dropsExpected, 10*time.Minute, "", nil)

		p1, _ := domain.ParseXRP("6")
		if err := inv.ApplyPayment(p1, validTx, time.Now().UTC()); err != nil {
			t.Fatalf("ApplyPayment p1 error: %v", err)
		}

		if inv.Status != domain.StatusPartiallyPaid {
			t.Errorf("expected %s, got %s", domain.StatusPartiallyPaid, inv.Status)
		}
		if inv.AmountPaid.Int64() != 6_000_000 {
			t.Errorf("expected 6M drops paid, got %d", inv.AmountPaid.Int64())
		}

		p2, _ := domain.ParseXRP("4")
		if err := inv.ApplyPayment(p2, validTx, time.Now().UTC()); err != nil {
			t.Fatalf("ApplyPayment p2 error: %v", err)
		}

		if inv.Status != domain.StatusSettled {
			t.Errorf("expected %s, got %s", domain.StatusSettled, inv.Status)
		}
		if inv.AmountPaid != dropsExpected {
			t.Errorf("expected 10M drops total paid, got %d", inv.AmountPaid.Int64())
		}
		if inv.Version != 3 {
			t.Errorf("expected version 3 after two payments, got %d", inv.Version)
		}
	})

	t.Run("Overpayment (15 XRP -> Overpaid)", func(t *testing.T) {
		inv, _ := domain.NewInvoice("o3", validMerchant, 103, dropsExpected, 10*time.Minute, "", nil)
		over, _ := domain.ParseXRP("15")

		if err := inv.ApplyPayment(over, validTx, time.Now().UTC()); err != nil {
			t.Fatalf("ApplyPayment over error: %v", err)
		}

		if inv.Status != domain.StatusOverpaid {
			t.Errorf("expected %s, got %s", domain.StatusOverpaid, inv.Status)
		}
		if !inv.IsTerminal() {
			t.Error("overpaid invoice must be terminal")
		}
	})
}

func TestInvoice_ExpirationsAndCancellations(t *testing.T) {
	dropsExpected, _ := domain.ParseXRP("5")

	t.Run("Expire pending invoice", func(t *testing.T) {
		inv, _ := domain.NewInvoice("o4", validMerchant, 104, dropsExpected, 1*time.Minute, "", nil)
		past := time.Now().UTC().Add(2 * time.Minute)

		if !inv.IsExpired(past) {
			t.Error("expected invoice to report expired")
		}
		if err := inv.MarkExpired(past); err != nil {
			t.Fatalf("MarkExpired error: %v", err)
		}
		if inv.Status != domain.StatusExpired {
			t.Errorf("expected status %s, got %s", domain.StatusExpired, inv.Status)
		}
		if !inv.IsTerminal() {
			t.Error("expired invoice must be terminal")
		}
	})

	t.Run("Cancel pending invoice", func(t *testing.T) {
		inv, _ := domain.NewInvoice("o5", validMerchant, 105, dropsExpected, 10*time.Minute, "", nil)
		if err := inv.MarkCanceled(); err != nil {
			t.Fatalf("MarkCanceled error: %v", err)
		}
		if inv.Status != domain.StatusCanceled {
			t.Errorf("expected status %s, got %s", domain.StatusCanceled, inv.Status)
		}
		if !inv.IsTerminal() {
			t.Error("canceled invoice must be terminal")
		}
	})

	t.Run("Illegal: Cannot cancel or expire a settled invoice", func(t *testing.T) {
		inv, _ := domain.NewInvoice("o6", validMerchant, 106, dropsExpected, 10*time.Minute, "", nil)
		_ = inv.ApplyPayment(dropsExpected, validTx, time.Now().UTC())

		if err := inv.MarkExpired(time.Now().UTC()); err == nil {
			t.Error("expected error when expiring settled invoice, got nil")
		}
		if err := inv.MarkCanceled(); err == nil {
			t.Error("expected error when canceling settled invoice, got nil")
		}
	})
}
