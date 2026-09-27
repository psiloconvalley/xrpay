package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/api"
	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

const testMerchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

func TestCreateInvoiceHandler(t *testing.T) {
	memStore := store.NewMemoryStore(1000)
	handler := api.NewHandler(memStore, testMerchantAddr, 15*time.Minute)

	reqBody := []byte(`{
		"order_id": "ORD-TEST-1",
		"amount": "5.5",
		"expiry_seconds": 600
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	handler.CreateInvoiceHandler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var inv domain.Invoice
	if err := json.NewDecoder(rec.Body).Decode(&inv); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	if inv.AmountDrops != 5500000 {
		t.Fatalf("expected 5500000 drops, got %d", inv.AmountDrops)
	}
	if inv.MerchantAddress != testMerchantAddr {
		t.Fatalf("merchant address mismatch: got %s, want %s", inv.MerchantAddress, testMerchantAddr)
	}
}

func TestGetInvoicePublicStatusHandler(t *testing.T) {
	memStore := store.NewMemoryStore(1000)
	handler := api.NewHandler(memStore, testMerchantAddr, 15*time.Minute)

	now := time.Now().UTC()
	inv, err := domain.NewInvoice("ORD-STATUS", testMerchantAddr, 1001, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
	if err != nil {
		t.Fatalf("failed creating invoice: %v", err)
	}
	_ = memStore.Save(context.Background(), inv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+inv.ID+"/status", nil)
	rec := httptest.NewRecorder()

	handler.GetInvoicePublicStatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp api.InvoiceStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding status response: %v", err)
	}

	if resp.ID != inv.ID || resp.Status != domain.StatusPending {
		t.Fatalf("unexpected status response: %+v", resp)
	}
}
