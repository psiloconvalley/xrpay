package webhook_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/webhook"
)

const testAddress = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

func TestDispatcher_DeliveryAndSignature(t *testing.T) {
	secret := "whsec_test_secret_12345"
	var rawReceivedBody []byte
	var receivedSigHeader string
	var serverHits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverHits, 1)
		receivedSigHeader = r.Header.Get("X-XRPAY-SIGNATURE")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		rawReceivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := webhook.NewDispatcher(secret, 5*time.Second)
	defer dispatcher.Stop()

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	inv, err := domain.NewInvoice(
		"ORD-999",
		testAddress,
		1001,
		domain.Drops(5000000),
		15*time.Minute,
		now,
		"https://example.com/return",
		server.URL,
		map[string]string{"user_id": "usr_42"},
	)
	if err != nil {
		t.Fatalf("failed creating invoice: %v", err)
	}

	ctx := context.Background()
	err = dispatcher.DispatchWithRetry(ctx, server.URL, webhook.EventInvoiceSettled, inv, 1)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}

	if atomic.LoadInt32(&serverHits) != 1 {
		t.Fatalf("expected 1 hit, got %d", serverHits)
	}

	// Verify signature using standard verifier
	if err := webhook.VerifySignature(rawReceivedBody, receivedSigHeader, secret, 0); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

func TestDispatcher_RetryOn500(t *testing.T) {
	secret := "whsec_retry_secret"
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&attempts, 1)
		if current < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := webhook.NewDispatcher(secret, 5*time.Second)
	defer dispatcher.Stop()

	ctx := context.Background()
	err := dispatcher.DispatchWithRetry(ctx, server.URL, webhook.EventInvoiceSettled, map[string]string{"foo": "bar"}, 3)
	if err != nil {
		t.Fatalf("expected retry to succeed on 2nd attempt, got err: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
