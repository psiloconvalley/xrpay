package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

// Sentinel errors for webhook operations.
var (
	ErrInvalidWebhookConfig = errors.New("invalid webhook configuration")
	ErrDeliveryFailed       = errors.New("webhook delivery failed after maximum retries")
	ErrEmptyPayload         = errors.New("webhook payload cannot be empty")
)

// EventType represents the category of the webhook notification.
type EventType string

const (
	EventInvoiceSettled       EventType = "invoice.settled"
	EventInvoicePartiallyPaid EventType = "invoice.partially_paid"
	EventInvoiceOverpaid      EventType = "invoice.overpaid"
	EventInvoiceExpired       EventType = "invoice.expired"
)

// Event is the standardized JSON payload sent to merchant webhook endpoints.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	CreatedAt time.Time       `json:"created_at"`
	Invoice   *domain.Invoice `json:"invoice"`
	TxHash    string          `json:"tx_hash,omitempty"`
}

// HTTPClient abstracts the HTTP transport for deterministic testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config holds configuration parameters for the Webhook Dispatcher.
type Config struct {
	WebhookSecret string
	MaxRetries    int
	InitialDelay  time.Duration
	HTTPClient    HTTPClient
	Logger        *slog.Logger
}

// Dispatcher manages reliable, HMAC-signed webhook delivery to merchant servers.
type Dispatcher struct {
	cfg        Config
	httpClient HTTPClient
	logger     *slog.Logger
	queue      chan Event
	wg         sync.WaitGroup
	stopChan   chan struct{}
}

// NewDispatcher constructs and validates a new Dispatcher.
func NewDispatcher(cfg Config) (*Dispatcher, error) {
	if cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("%w: webhook secret is required", ErrInvalidWebhookConfig)
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 500 * time.Millisecond
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	d := &Dispatcher{
		cfg:        cfg,
		httpClient: cfg.HTTPClient,
		logger:     cfg.Logger.With("component", "webhook.dispatcher"),
		queue:      make(chan Event, 256),
		stopChan:   make(chan struct{}),
	}

	return d, nil
}

// Sign generates an HMAC-SHA256 signature for the given payload and timestamp.
// Signature format: hex(HMAC-SHA256(secret, "t=" + timestamp + "." + body))
func Sign(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	signedData := fmt.Sprintf("t=%d.", timestamp)
	mac.Write([]byte(signedData))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// DispatchSync sends an event immediately to the invoice's WebhookURL with exponential backoff.
func (d *Dispatcher) DispatchSync(ctx context.Context, event Event) error {
	if event.Invoice == nil || event.Invoice.WebhookURL == "" {
		d.logger.DebugContext(ctx, "no webhook url configured for invoice; skipping dispatch",
			"invoice_id", event.Invoice.ID,
		)
		return nil
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal webhook event: %w", err)
	}

	url := event.Invoice.WebhookURL
	timestamp := time.Now().UTC().Unix()
	signature := Sign(payload, d.cfg.WebhookSecret, timestamp)

	delay := d.cfg.InitialDelay

	for attempt := 1; attempt <= d.cfg.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create webhook request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "xrpay-gateway/1.0")
		req.Header.Set("X-XRPay-Signature", signature)
		req.Header.Set("X-XRPay-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-XRPay-Event", string(event.Type))

		resp, err := d.httpClient.Do(req)
		if err == nil {
			// Drain and close response body to reuse connections
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				d.logger.InfoContext(ctx, "webhook delivered successfully",
					"invoice_id", event.Invoice.ID,
					"event_type", event.Type,
					"url", url,
					"attempt", attempt,
					"status_code", resp.StatusCode,
				)
				return nil
			}

			d.logger.WarnContext(ctx, "webhook endpoint returned non-2xx status",
				"invoice_id", event.Invoice.ID,
				"url", url,
				"status_code", resp.StatusCode,
				"attempt", attempt,
			)
		} else {
			d.logger.WarnContext(ctx, "webhook network error",
				"invoice_id", event.Invoice.ID,
				"url", url,
				"error", err,
				"attempt", attempt,
			)
		}

		if attempt < d.cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay *= 2 // Exponential backoff
			}
		}
	}

	return fmt.Errorf("%w: %s (attempts: %d)", ErrDeliveryFailed, url, d.cfg.MaxRetries)
}
