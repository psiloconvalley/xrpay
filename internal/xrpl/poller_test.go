package xrpl_test

import (
	"context"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

// mockPaymentFetcher simulates XRPL RPC query responses.
type mockPaymentFetcher struct {
	payments []xrpl.PaymentEvent
	err      error
}

func (m *mockPaymentFetcher) GetAccountPayments(ctx context.Context, account string, minLedger int64) ([]xrpl.PaymentEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.payments, nil
}

// mockClock provides static time control for testing.
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func TestPoller_PollOnce_SettlesInvoice(t *testing.T) {
	ctx := context.Background()
	merchantAddr := "rPT1Sjq2YGrBMTttX4GZHjKu9DYfzbpAYe"
	tag := uint32(1001)

	// 1. Setup Memory Store and create a pending 10 XRP invoice
	memStore := store.NewMemoryStore()
	invAmount, err := domain.ParseXRP("10.0")
	if err != nil {
		t.Fatalf("ParseXRP failed: %v", err)
	}

	inv, err := domain.NewInvoice("order_123", merchantAddr, tag, invAmount, 15*time.Minute, "", nil)
	if err != nil {
		t.Fatalf("NewInvoice failed: %v", err)
	}
	if err := memStore.Save(ctx, inv); err != nil {
		t.Fatalf("Store.Save failed: %v", err)
	}

	// 2. Setup mock payment fetcher delivering exactly 10 XRP (10,000,000 drops)
	deliveredDrops, _ := domain.FromDrops(10_000_000)
	fetcher := &mockPaymentFetcher{
		payments: []xrpl.PaymentEvent{
			{
				TxHash:          "A1B2C3D4E5F67890",
				Sender:          "rCustomerWalletAddress1234567890",
				Destination:     merchantAddr,
				DestinationTag:  tag,
				DeliveredDrops:  deliveredDrops,
				LedgerIndex:     85000000,
				Validated:       true,
			},
		},
	}

	// 3. Track listener invocation
	var listenerCalled bool
	var settledStatus domain.Status

	poller, err := xrpl.NewPoller(fetcher, memStore, xrpl.PollerConfig{
		MerchantAccount: merchantAddr,
		PollInterval:    100 * time.Millisecond,
		StartLedger:     84999990,
		OnPayment: func(ctx context.Context, updatedInv *domain.Invoice, event xrpl.PaymentEvent) {
			listenerCalled = true
			settledStatus = updatedInv.Status
		},
	})
	if err != nil {
		t.Fatalf("NewPoller failed: %v", err)
	}

	// 4. Execute single poll pass
	if err := poller.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce failed: %v", err)
	}

	// 5. Verify Store has updated invoice to StatusSettled
	updated, err := memStore.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if updated.Status != domain.StatusSettled {
		t.Errorf("expected status %q, got %q", domain.StatusSettled, updated.Status)
	}
	if updated.AmountPaid != deliveredDrops {
		t.Errorf("expected amount paid %v, got %v", deliveredDrops, updated.AmountPaid)
	}
	if updated.TxHash != "A1B2C3D4E5F67890" {
		t.Errorf("expected tx_hash %q, got %q", "A1B2C3D4E5F67890", updated.TxHash)
	}
	if !listenerCalled {
		t.Errorf("expected payment listener to be called")
	}
	if settledStatus != domain.StatusSettled {
		t.Errorf("expected listener status %q, got %q", domain.StatusSettled, settledStatus)
	}
	if poller.LastLedger() != 85000000 {
		t.Errorf("expected LastLedger to advance to 85000000, got %d", poller.LastLedger())
	}
}

func TestPoller_PollOnce_ExpiresOverdueInvoices(t *testing.T) {
	ctx := context.Background()
	merchantAddr := "rPT1Sjq2YGrBMTttX4GZHjKu9DYfzbpAYe"
	tag := uint32(1002)

	memStore := store.NewMemoryStore()
	invAmount, _ := domain.ParseXRP("5.0")

	inv, err := domain.NewInvoice("order_exp", merchantAddr, tag, invAmount, 5*time.Minute, "", nil)
	if err != nil {
		t.Fatalf("NewInvoice failed: %v", err)
	}
	if err := memStore.Save(ctx, inv); err != nil {
		t.Fatalf("Store.Save failed: %v", err)
	}

	// Time is now 10 minutes in the future (past 5 min duration)
	futureTime := inv.CreatedAt.Add(10 * time.Minute)
	clock := &mockClock{now: futureTime}

	fetcher := &mockPaymentFetcher{payments: nil} // No payments on-chain

	poller, err := xrpl.NewPoller(fetcher, memStore, xrpl.PollerConfig{
		MerchantAccount: merchantAddr,
		Clock:           clock,
	})
	if err != nil {
		t.Fatalf("NewPoller failed: %v", err)
	}

	if err := poller.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce failed: %v", err)
	}

	// Verify invoice transitioned to StatusExpired
	updated, err := memStore.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if updated.Status != domain.StatusExpired {
		t.Errorf("expected status %q, got %q", domain.StatusExpired, updated.Status)
	}
}

func TestPoller_Lifecycle_StartAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	merchantAddr := "rPT1Sjq2YGrBMTttX4GZHjKu9DYfzbpAYe"
	memStore := store.NewMemoryStore()
	fetcher := &mockPaymentFetcher{payments: nil}

	poller, err := xrpl.NewPoller(fetcher, memStore, xrpl.PollerConfig{
		MerchantAccount: merchantAddr,
		PollInterval:    20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewPoller failed: %v", err)
	}

	// Start poller
	if err := poller.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Second start must fail
	if err := poller.Start(ctx); err != xrpl.ErrPollerAlreadyRunning {
		t.Errorf("expected ErrPollerAlreadyRunning, got %v", err)
	}

	// Stop poller gracefully
	if err := poller.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Second stop must fail
	if err := poller.Stop(); err != xrpl.ErrPollerNotRunning {
		t.Errorf("expected ErrPollerNotRunning, got %v", err)
	}
}
