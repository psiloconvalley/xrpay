package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/webhook"
)

func TestSign_Deterministic(t *testing.T) {
	secret := "whsec_test_secret_key_12345"
	payload := []byte(`{"id":"evt_123","type":"invoice.settled"}`)
	timestamp := int64(1700000000)

	sig1 := webhook.Sign(payload, secret, timestamp)
	sig2 := webhook.Sign(payload, secret, timestamp)

	if sig1 == "" {
		t.Fatal("expected non-empty signature")
	}
	if sig1 != sig2 {
		t.Fatalf("signatures are not deterministic: %s != %s", sig1, sig2)
	}

	// Different timestamp must produce different signature
	sig3 := webhook.Sign(payload, secret, timestamp+1)
	if sig1 == sig3 {
		t.Fatal("expected different signature for different timestamp")
	}
}

func TestDispatcher_DispatchSync_Success(t *testing.T) {
	var receivedSig string
	var receivedTimestamp string
	var receivedEvent webhook.Event

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-XRPay-Signature")
		receivedTimestamp = r.Header.Get("X-XRPay-Timestamp")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		_ = json.Unmarshal(body, &receivedEvent)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	amount, _ := domain.ParseXRP("10.0")
	inv, _ := domain.NewInvoice("order_1", "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh", 1001, amount, 15*time.Minute, server.URL, nil)

	secret := "whsec_test_mock_secret"
	dispatcher, err := webhook.NewDispatcher(webhook.Config{
		WebhookSecret: secret,
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatalf("NewDispatcher failed: %v", err)
	}

	event := webhook.Event{
		ID:        "evt_test_123",
		Type:      webhook.EventInvoiceSettled,
		CreatedAt: time.Now().UTC(),
		Invoice:   inv,
		TxHash:    "TX_HASH_XYZ",
	}

	ctx := context.Background()
	if err := dispatcher.DispatchSync(ctx, event); err != nil {
		t.Fatalf("DispatchSync failed: %v", err)
	}

	if receivedSig == "" {
		t.Error("expected X-XRPay-Signature header to be set")
	}
	if receivedTimestamp == "" {
		t.Error("expected X-XRPay-Timestamp header to be set")
	}
	if receivedEvent.ID != "evt_test_123" {
		t.Errorf("expected event ID %q, got %q", "evt_test_123", receivedEvent.ID)
	}
}

func TestDispatcher_DispatchSync_RetryThenFail(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	amount, _ := domain.ParseXRP("10.0")
	inv, _ := domain.NewInvoice("order_fail", "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh", 1002, amount, 15*time.Minute, server.URL, nil)

	dispatcher, err := webhook.NewDispatcher(webhook.Config{
		WebhookSecret: "whsec_test",
		MaxRetries:    3,
		InitialDelay:  5 * time.Millisecond,
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatalf("NewDispatcher failed: %v", err)
	}

	event := webhook.Event{
		ID:      "evt_fail_123",
		Type:    webhook.EventInvoiceSettled,
		Invoice: inv,
	}

	ctx := context.Background()
	err = dispatcher.DispatchSync(ctx, event)
	if err == nil {
		t.Fatal("expected delivery to fail, but got nil")
	}

	if attempts.Load() != 3 {
		t.Errorf("expected 3 retry attempts, got %d", attempts.Load())
	}
}
