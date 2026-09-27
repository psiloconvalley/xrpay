package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/api"
	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

const testMerchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

func TestHandler_LandingPage(t *testing.T) {
	memStore := store.NewMemoryStore(1000)
	handler := api.NewHandler(memStore, testMerchantAddr, 15*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.LandingHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "xrpay") || !strings.Contains(body, "Self-host XRP payments") {
		t.Fatalf("expected elegant terminal title on root page, got: %s", body)
	}
}

func TestHandler_Telemetry(t *testing.T) {
	memStore := store.NewMemoryStore(1000)
	handler := api.NewHandler(memStore, testMerchantAddr, 15*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry", nil)
	rr := httptest.NewRecorder()

	handler.TelemetryHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var resp api.TelemetryResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding telemetry json: %v", err)
	}

	if resp.MerchantAddress != testMerchantAddr {
		t.Errorf("mismatched merchant address: got %s", resp.MerchantAddress)
	}
	if resp.UptimeSeconds < 0 {
		t.Errorf("invalid uptime seconds: %d", resp.UptimeSeconds)
	}
}

func TestHandler_DemoInvoice(t *testing.T) {
	memStore := store.NewMemoryStore(1000)
	handler := api.NewHandler(memStore, testMerchantAddr, 15*time.Minute)

	t.Run("successful form POST redirect", func(t *testing.T) {
		form := url.Values{}
		form.Set("amount", "5.00")
		form.Set("memo", "SaaS Sandbox Renewal")

		req := httptest.NewRequest(http.MethodPost, "/demo/invoice", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handler.DemoInvoiceHandler(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 StatusSeeOther redirect, got %d", rr.Code)
		}

		loc := rr.Header().Get("Location")
		if !strings.Contains(loc, "/checkout/inv_") {
			t.Errorf("unexpected redirect location: %s", loc)
		}
	})

	t.Run("rejects demo amount under 1.00 XRP to block spam", func(t *testing.T) {
		form := url.Values{}
		form.Set("amount", "0.50")
		form.Set("memo", "Micro Settle Demo")

		req := httptest.NewRequest(http.MethodPost, "/demo/invoice", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		handler.DemoInvoiceHandler(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request on micro-spam, got %d", rr.Code)
		}
	})
}

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
