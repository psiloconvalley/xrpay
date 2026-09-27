package domain_test

import (
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

const testMerchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

func TestInvoiceLifecycle(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("successful full settlement with payment history", func(t *testing.T) {
		inv, err := domain.NewInvoice("ORD-1", testMerchantAddr, 1001, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if inv.Status != domain.StatusPending {
			t.Fatalf("expected PENDING, got %s", inv.Status)
		}

		err = inv.ApplyPayment(domain.Drops(5000000), "TX_HASH_123", now.Add(1*time.Minute))
		if err != nil {
			t.Fatalf("unexpected payment error: %v", err)
		}

		if inv.Status != domain.StatusSettled {
			t.Fatalf("expected SETTLED, got %s", inv.Status)
		}
		if len(inv.Payments) != 1 {
			t.Fatalf("expected 1 payment record, got %d", len(inv.Payments))
		}
		if inv.Payments[0].TxHash != "TX_HASH_123" {
			t.Fatalf("expected TX_HASH_123, got %s", inv.Payments[0].TxHash)
		}
		if !inv.IsTerminal() {
			t.Fatal("expected invoice to be in terminal state")
		}
	})

	t.Run("partial payment then settlement", func(t *testing.T) {
		inv, err := domain.NewInvoice("ORD-2", testMerchantAddr, 1002, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// First payment: 2 XRP
		err = inv.ApplyPayment(domain.Drops(2000000), "TX_PART_1", now.Add(1*time.Minute))
		if err != nil {
			t.Fatalf("unexpected payment error: %v", err)
		}
		if inv.Status != domain.StatusPartiallyPaid {
			t.Fatalf("expected PARTIALLY_PAID, got %s", inv.Status)
		}
		if inv.IsTerminal() {
			t.Fatal("partially paid invoice should not be terminal")
		}

		// Second payment: 3 XRP
		err = inv.ApplyPayment(domain.Drops(3000000), "TX_PART_2", now.Add(2*time.Minute))
		if err != nil {
			t.Fatalf("unexpected payment error: %v", err)
		}
		if inv.Status != domain.StatusSettled {
			t.Fatalf("expected SETTLED, got %s", inv.Status)
		}
		if len(inv.Payments) != 2 {
			t.Fatalf("expected 2 payment records, got %d", len(inv.Payments))
		}
		if inv.Payments[0].TxHash != "TX_PART_1" || inv.Payments[1].TxHash != "TX_PART_2" {
			t.Fatalf("unexpected payment history: %+v", inv.Payments)
		}
	})

	t.Run("overpaid invoice", func(t *testing.T) {
		inv, err := domain.NewInvoice("ORD-3", testMerchantAddr, 1003, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = inv.ApplyPayment(domain.Drops(6000000), "TX_OVER", now.Add(1*time.Minute))
		if err != nil {
			t.Fatalf("unexpected payment error: %v", err)
		}
		if inv.Status != domain.StatusOverpaid {
			t.Fatalf("expected OVERPAID, got %s", inv.Status)
		}
	})

	t.Run("expired invoice cannot be paid", func(t *testing.T) {
		inv, err := domain.NewInvoice("ORD-4", testMerchantAddr, 1004, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired := inv.Expire(now.Add(16 * time.Minute))
		if !expired {
			t.Fatal("expected invoice to expire")
		}

		err = inv.ApplyPayment(domain.Drops(5000000), "TX_LATE", now.Add(17*time.Minute))
		if err == nil {
			t.Fatal("expected error applying payment to expired invoice")
		}
	})
}
