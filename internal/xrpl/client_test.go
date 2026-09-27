package xrpl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

func TestClient_GetAccountPayments_Success(t *testing.T) {
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
						"hash": "TX_HASH_XYZ",
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

	client := xrpl.NewClient(server.URL, 5*time.Second)

	payments, latestLedger, err := client.GetAccountPayments(context.Background(), "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V", 0, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(payments) != 1 {
		t.Fatalf("expected 1 payment, got %d", len(payments))
	}

	pay := payments[0]
	if pay.TxHash != "TX_HASH_XYZ" {
		t.Errorf("expected TX_HASH_XYZ, got %s", pay.TxHash)
	}
	if pay.DeliveredDrops != domain.Drops(5000000) {
		t.Errorf("expected 5000000 drops, got %d", pay.DeliveredDrops)
	}
	if pay.DestinationTag != 1001 {
		t.Errorf("expected tag 1001, got %d", pay.DestinationTag)
	}
	if latestLedger != 42000 {
		t.Errorf("expected latest ledger 42000, got %d", latestLedger)
	}
}

func TestClient_GetAccountPayments_Unfunded(t *testing.T) {
	mockResponse := `{
		"result": {
			"status": "error",
			"error": "actNotFound",
			"error_message": "Account not found."
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := xrpl.NewClient(server.URL, 5*time.Second)

	_, _, err := client.GetAccountPayments(context.Background(), "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V", 0, -1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err != xrpl.ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}
