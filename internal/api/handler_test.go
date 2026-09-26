package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/api"
	"github.com/psiloconvalley/xrpay/internal/store"
)

func setupTestServer(t *testing.T) (*http.ServeMux, string) {
	merchantAddr := "rPT1Sjq2YGrBMTttX4GZHjKu9DYfzbpAYe"
	memStore := store.NewMemoryStore()

	handler, err := api.NewHandler(memStore, api.Config{
		MerchantAccount: merchantAddr,
		BaseURL:         "https://xrpay.gopherit.dev",
		DefaultDuration: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, merchantAddr
}

func TestHealthCheck(t *testing.T) {
	mux, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %q", body["status"])
	}
}

func TestCreateAndGetInvoice_Workflow(t *testing.T) {
	mux, merchantAddr := setupTestServer(t)

	// 1. POST /api/v1/invoices (Create Invoice)
	createPayload := `{"order_id":"order_web_123","amount":"25.50","duration_minutes":20}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader([]byte(createPayload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created api.InvoiceResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created invoice: %v", err)
	}

	if created.ID == "" || !strings.HasPrefix(created.ID, "inv_") {
		t.Errorf("expected invoice ID starting with 'inv_', got %q", created.ID)
	}
	if created.MerchantAddress != merchantAddr {
		t.Errorf("expected merchant address %q, got %q", merchantAddr, created.MerchantAddress)
	}
	if created.DestinationTag == 0 {
		t.Errorf("expected allocated destination tag != 0")
	}
	if created.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", created.Status)
	}
	if created.AmountExpected != "25.5 XRP" {
		t.Errorf("expected amount '25.5 XRP', got %q", created.AmountExpected)
	}

	// 2. GET /api/v1/invoices/{id} (Retrieve Invoice)
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+created.ID, nil)
	getW := httptest.NewRecorder()

	mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on get, got %d", getW.Code)
	}

	var fetched api.InvoiceResponse
	if err := json.NewDecoder(getW.Body).Decode(&fetched); err != nil {
		t.Fatalf("failed to decode fetched invoice: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, fetched.ID)
	}

	// 3. GET /checkout/{id} (Render Checkout UI)
	uiReq := httptest.NewRequest(http.MethodGet, "/checkout/"+created.ID, nil)
	uiW := httptest.NewRecorder()

	mux.ServeHTTP(uiW, uiReq)

	if uiW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on checkout UI, got %d", uiW.Code)
	}

	bodyStr := uiW.Body.String()
	if !strings.Contains(bodyStr, created.ID) {
		t.Errorf("expected checkout HTML to contain invoice ID %q", created.ID)
	}
	if !strings.Contains(bodyStr, merchantAddr) {
		t.Errorf("expected checkout HTML to contain merchant address")
	}
}

func TestCreateInvoice_InvalidPayloads(t *testing.T) {
	mux, _ := setupTestServer(t)

	testCases := []struct {
		name       string
		payload    string
		expectCode int
	}{
		{"missing order_id", `{"amount":"10.0"}`, http.StatusBadRequest},
		{"negative amount", `{"order_id":"1","amount":"-5.0"}`, http.StatusBadRequest},
		{"malformed json", `{broken`, http.StatusBadRequest},
		{"zero amount", `{"order_id":"1","amount":"0.0"}`, http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader([]byte(tc.payload)))
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d", tc.expectCode, w.Code)
			}
		})
	}
}
