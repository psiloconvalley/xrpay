package xrpl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
	"github.com/psiloconvalley/xrpay/internal/webhook"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

const testMerchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

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
	memStore := store.NewMemoryStore(1000)

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
	_ = memStore.Save(ctx, inv)

	client := xrpl.NewClient(server.URL, 5*time.Second)
	dispatcher := webhook.NewDispatcher("secret", 5*time.Second)
	defer dispatcher.Stop()

	poller := xrpl.NewPoller(client, memStore, dispatcher, testMerchantAddr, 100*time.Millisecond, 0)
	poller.Start(ctx)
	defer poller.Stop()

	// Wait briefly for poller tick to find payment
	time.Sleep(200 * time.Millisecond)

	updatedInv, err := memStore.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed fetching updated invoice: %v", err)
	}

	if updatedInv.Status != domain.StatusSettled {
		t.Errorf("expected invoice to be settled, got %s", updatedInv.Status)
	}

	if updatedInv.TxHash != "TX_HASH_POLLED" {
		t.Errorf("expected transaction hash TX_HASH_POLLED, got %s", updatedInv.TxHash)
	}
}
