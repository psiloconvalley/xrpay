package xrpl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
	"github.com/psiloconvalley/xrpay/internal/webhook"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

const testMerchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

type mockBookmarkStore struct {
	*store.MemoryStore
	mu             sync.Mutex
	recordedLedger int64
}

func newMockBookmarkStore() *mockBookmarkStore {
	return &mockBookmarkStore{
		MemoryStore: store.NewMemoryStore(100),
	}
}

func (m *mockBookmarkStore) SetLastProcessedLedger(ctx context.Context, index int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordedLedger = index
	return nil
}

func (m *mockBookmarkStore) GetRecordedLedger() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recordedLedger
}

func TestPoller_ReconcileSuccess(t *testing.T) {
	mockResponse := `{
		"result": {
			"status": "success",
			"account": "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V",
			"transactions": [
				{
					"validated": true,
					"tx": {
						"TransactionType": "Payment",
						"Account": "rCustomer...",
						"Destination": "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V",
						"DestinationTag": 1001,
						"Amount": "5000000",
						"hash": "TX_HASH_POLLED",
						"date": 788918400,
						"ledger_index": 42000
					},
					"meta": {
						"TransactionResult": "tesSUCCESS",
						"delivered_amount": "5000000"
					}
				}
			]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	ctx := context.Background()
	mockStore := newMockBookmarkStore()

	// Save an active invoice matching tag 1001
	now := time.Now().UTC()
	inv, err := domain.NewInvoice(
		"ORD-POLLED",
		testMerchantAddr,
		1001,
		domain.Drops(5000000),
		15*time.Minute,
		now,
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("failed creating invoice: %v", err)
	}
	_ = mockStore.Save(ctx, inv)

	client := xrpl.NewClient(server.URL, 5*time.Second)
	dispatcher := webhook.NewDispatcher("secret", 5*time.Second)
	defer dispatcher.Stop()

	poller := xrpl.NewPoller(client, mockStore, dispatcher, testMerchantAddr, 50*time.Millisecond, 0)
	poller.Start(ctx)
	defer poller.Stop()

	// Wait briefly for poller tick to find payment
	time.Sleep(150 * time.Millisecond)

	updatedInv, err := mockStore.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed fetching updated invoice: %v", err)
	}

	if updatedInv.Status != domain.StatusSettled {
		t.Errorf("expected invoice to be settled, got %s", updatedInv.Status)
	}

	if updatedInv.TxHash != "TX_HASH_POLLED" {
		t.Errorf("expected transaction hash TX_HASH_POLLED, got %s", updatedInv.TxHash)
	}

	if recorded := mockStore.GetRecordedLedger(); recorded != 42000 {
		t.Errorf("expected recorded bookmark ledger 42000, got %d", recorded)
	}
}
