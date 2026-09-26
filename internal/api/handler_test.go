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
	merchantAddr := "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
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
func TestCreateInvoice_Authentication(t *testing.T) {
	merchantAddr := "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
	memStore := store.NewMemoryStore()
	apiKey := "sk_test_secret_api_key_12345"

	handler, err := api.NewHandler(memStore, api.Config{
		MerchantAccount: merchantAddr,
		APIKey:          apiKey,
	})
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	payload := `{"order_id":"auth_test","amount":"10.0"}`

	// 1. Request without Authorization header must fail with 401
	reqNoAuth := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader([]byte(payload)))
	wNoAuth := httptest.NewRecorder()
	mux.ServeHTTP(wNoAuth, reqNoAuth)
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing auth, got %d", wNoAuth.Code)
	}

	// 2. Request with invalid API key must fail with 401
	reqWrongAuth := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader([]byte(payload)))
	reqWrongAuth.Header.Set("Authorization", "Bearer sk_wrong_key")
	wWrongAuth := httptest.NewRecorder()
	mux.ServeHTTP(wWrongAuth, reqWrongAuth)
	if wWrongAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for wrong key, got %d", wWrongAuth.Code)
	}

	// 3. Request with valid API key must succeed with 201
	reqValidAuth := httptest.NewRequest(http.MethodPost, "/api/v1/invoices", bytes.NewReader([]byte(payload)))
	reqValidAuth.Header.Set("Authorization", "Bearer "+apiKey)
	wValidAuth := httptest.NewRecorder()
	mux.ServeHTTP(wValidAuth, reqValidAuth)
	if wValidAuth.Code != http.StatusCreated {
		t.Errorf("expected 201 Created for valid auth, got %d", wValidAuth.Code)
	}
}

func TestCORS_Preflight(t *testing.T) {
	mux, _ := setupTestServer(t)
	wrapped := api.CORSMiddleware(mux)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/invoices", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *")
	}
}
