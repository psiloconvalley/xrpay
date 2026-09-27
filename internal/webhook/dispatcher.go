package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	EventInvoiceSettled = "invoice.settled"
	EventInvoiceExpired = "invoice.expired"
)

var (
	ErrInvalidSignatureHeader = errors.New("invalid signature header format")
	ErrSignatureMismatch      = errors.New("signature mismatch")
	ErrTimestampExpired       = errors.New("webhook timestamp outside tolerance window")
)

type EventPayload struct {
	ID        string      `json:"id"`
	Event     string      `json:"event"`
	CreatedAt int64       `json:"created_at"`
	Data      interface{} `json:"data"`
}

// Sign computes the HMAC-SHA256 signature for a payload and timestamp.
func Sign(payload []byte, timestamp int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates a webhook signature header against expected secret and tolerance window.
func VerifySignature(payload []byte, sigHeader string, secret string, tolerance time.Duration) error {
	parts := strings.Split(sigHeader, ",")
	if len(parts) != 2 {
		return ErrInvalidSignatureHeader
	}

	timePart := strings.TrimPrefix(parts[0], "t=")
	sigPart := strings.TrimPrefix(parts[1], "v1=")

	ts, err := strconv.ParseInt(timePart, 10, 64)
	if err != nil {
		return ErrInvalidSignatureHeader
	}

	if tolerance > 0 {
		now := time.Now().Unix()
		if now-ts > int64(tolerance.Seconds()) || ts-now > int64(tolerance.Seconds()) {
			return ErrTimestampExpired
		}
	}

	expected := Sign(payload, ts, secret)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sigPart)) != 1 {
		return ErrSignatureMismatch
	}

	return nil
}

type Dispatcher struct {
	secret     string
	httpClient *http.Client
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewDispatcher(secret string, timeout time.Duration) *Dispatcher {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		secret: secret,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (d *Dispatcher) Stop() {
	d.cancel()
	d.wg.Wait()
}

func (d *Dispatcher) DispatchAsync(targetURL string, event string, data interface{}) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if err := d.DispatchWithRetry(d.ctx, targetURL, event, data, 3); err != nil {
			slog.Warn("webhook delivery exhausted all retries", "url", targetURL, "event", event, "err", err)
		}
	}()
}

func (d *Dispatcher) DispatchWithRetry(ctx context.Context, targetURL string, event string, data interface{}, maxRetries int) error {
	payload := EventPayload{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Event:     event,
		CreatedAt: time.Now().Unix(),
		Data:      data,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed encoding webhook payload: %w", err)
	}

	backoff := 1 * time.Second
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err = d.send(ctx, targetURL, bodyBytes, payload.CreatedAt)
		if err == nil {
			slog.Info("webhook delivered successfully", "url", targetURL, "event", event, "attempt", attempt)
			return nil
		}

		slog.Warn("webhook delivery attempt failed", "url", targetURL, "attempt", attempt, "err", err)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, err)
}

func (d *Dispatcher) send(ctx context.Context, targetURL string, body []byte, timestamp int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	sig := Sign(body, timestamp, d.secret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "xrpay-webhook-engine/1.0")
	req.Header.Set("X-XRPAY-SIGNATURE", fmt.Sprintf("t=%d,v1=%s", timestamp, sig))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}

	return nil
}
