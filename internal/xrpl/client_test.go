package xrpl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

const merchantAddr = "rPT1Sjq2YGrBMTttX4GZHjKu9DYfzbpAYe"

func TestClient_GetAccountPayments_Success(t *testing.T) {
	// Mock XRPL RPC server response
	mockResponse := `{
		"result": {
			"account": "` + merchantAddr + `",
			"status": "success",
			"validated": true,
			"transactions": [
				{
					"validated": true,
					"tx": {
						"TransactionType": "Payment",
						"Account": "rCustomerAddress123456789012345678",
						"Destination": "` + merchantAddr + `",
						"DestinationTag": 1042,
						"hash": "4B8F9A1C2D3E4F5A6B7C8D9E0F1A2B3C4D5E6F7A8B9C0D1E2F3A4B5C6D7E8F9A",
						"ledger_index": 500123
					},
					"meta": {
						"TransactionResult": "tesSUCCESS",
						"delivered_amount": "25500000"
					}
				},
				{
					"validated": true,
					"tx": {
						"TransactionType": "Payment",
						"Account": "rCustomer2",
						"Destination": "` + merchantAddr + `",
						"DestinationTag": 1043,
						"hash": "FAILEDFASH123",
						"ledger_index": 500124
					},
					"meta": {
						"TransactionResult": "tecPATH_PARTIAL",
						"delivered_amount": "1000000"
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

	client := xrpl.NewClient(server.URL)
	payments, err := client.GetAccountPayments(context.Background(), merchantAddr, -1)
	if err != nil {
		t.Fatalf("GetAccountPayments unexpected error: %v", err)
	}

	// Should only include the successful payment (tecPATH_PARTIAL ignored)
	if len(payments) != 1 {
		t.Fatalf("expected 1 valid payment, got %d", len(payments))
	}

	p := payments[0]
	if p.DestinationTag != 1042 {
		t.Errorf("expected tag 1042, got %d", p.DestinationTag)
	}
	if p.DeliveredDrops != domain.Drops(25_500_000) {
		t.Errorf("expected 25500000 drops, got %d", p.DeliveredDrops)
	}
	if p.TxHash != "4B8F9A1C2D3E4F5A6B7C8D9E0F1A2B3C4D5E6F7A8B9C0D1E2F3A4B5C6D7E8F9A" {
		t.Errorf("tx hash mismatch: %s", p.TxHash)
	}
}
