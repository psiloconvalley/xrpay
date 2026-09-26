package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status represents the finite state of an invoice.
type Status string

const (
	StatusPending       Status = "pending"        // Awaiting transaction on XRPL
	StatusDetected      Status = "detected"       // Seen on network, awaiting validated ledger
	StatusPartiallyPaid Status = "partially_paid" // Received payment, but below amount expected
	StatusSettled       Status = "settled"        // Full payment validated in a closed ledger (final)
	StatusOverpaid      Status = "overpaid"       // Validated, but customer sent more than requested
	StatusExpired       Status = "expired"        // Timed out before full payment
	StatusCanceled      Status = "canceled"       // Canceled by merchant
)

var (
	ErrInvalidAddress    = errors.New("invalid merchant XRPL address (must start with 'r')")
	ErrInvalidTag        = errors.New("destination tag cannot be zero")
	ErrInvalidDuration   = errors.New("invoice expiration duration must be positive")
	ErrIllegalTransition = errors.New("illegal invoice status transition")
	ErrEmptyTxHash       = errors.New("tx hash cannot be empty on settlement")
	ErrNegativePayment   = errors.New("payment amount cannot be negative")
)

// Invoice represents a robust payment request tied to a unique DestinationTag.
type Invoice struct {
	ID             string            `json:"id"`
	OrderID        string            `json:"order_id,omitempty"`       // Merchant's internal reference ID
	MerchantAddr   string            `json:"merchant_address"`
	DestinationTag uint32            `json:"destination_tag"`
	AmountExpected Drops             `json:"amount_expected_drops"`
	AmountPaid     Drops             `json:"amount_paid_drops"`        // Actual drops received so far
	Status         Status            `json:"status"`
	TxHash         string            `json:"tx_hash,omitempty"`
	WebhookURL     string            `json:"webhook_url,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Version        int64             `json:"version"`                  // Optimistic concurrency counter
	CreatedAt      time.Time         `json:"created_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	SettledAt      *time.Time        `json:"settled_at,omitempty"`
}

// NewInvoice creates a hardened pending invoice.
func NewInvoice(
	orderID string,
	merchantAddr string,
	tag uint32,
	amount Drops,
	duration time.Duration,
	webhookURL string,
	metadata map[string]string,
) (*Invoice, error) {
	merchantAddr = strings.TrimSpace(merchantAddr)
	if !strings.HasPrefix(merchantAddr, "r") || len(merchantAddr) < 25 || len(merchantAddr) > 35 {
		return nil, ErrInvalidAddress
	}

	if tag == 0 {
		return nil, ErrInvalidTag
	}

	if amount <= 0 {
		return nil, ErrAmountTooSmall
	}

	if duration <= 0 {
		return nil, ErrInvalidDuration
	}

	id, err := generateInvoiceID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice id: %w", err)
	}

	now := time.Now().UTC()
	return &Invoice{
		ID:             id,
		OrderID:        strings.TrimSpace(orderID),
		MerchantAddr:   merchantAddr,
		DestinationTag: tag,
		AmountExpected: amount,
		AmountPaid:     0,
		Status:         StatusPending,
		WebhookURL:     strings.TrimSpace(webhookURL),
		Metadata:       metadata,
		Version:        1,
		CreatedAt:      now,
		ExpiresAt:      now.Add(duration),
	}, nil
}

// ApplyPayment records an incoming validated payment on XRPL and transitions state accurately.
func (inv *Invoice) ApplyPayment(deliveredAmount Drops, txHash string, settledAt time.Time) error {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return ErrEmptyTxHash
	}
	if deliveredAmount <= 0 {
		return ErrNegativePayment
	}

	if inv.Status == StatusSettled || inv.Status == StatusOverpaid {
		return fmt.Errorf("%w: invoice is already finalized", ErrIllegalTransition)
	}

	if settledAt.IsZero() {
		settledAt = time.Now().UTC()
	}

	inv.AmountPaid += deliveredAmount
	inv.TxHash = txHash
	inv.Version++

	if inv.AmountPaid == inv.AmountExpected {
		inv.Status = StatusSettled
		inv.SettledAt = &settledAt
	} else if inv.AmountPaid > inv.AmountExpected {
		inv.Status = StatusOverpaid
		inv.SettledAt = &settledAt
	} else {
		inv.Status = StatusPartiallyPaid
	}

	return nil
}

// MarkDetected transitions the invoice to detected when seen in the mempool.
func (inv *Invoice) MarkDetected(txHash string) error {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return ErrEmptyTxHash
	}

	if inv.Status != StatusPending && inv.Status != StatusPartiallyPaid {
		return fmt.Errorf("%w: cannot mark %s as detected", ErrIllegalTransition, inv.Status)
	}

	inv.Status = StatusDetected
	inv.TxHash = txHash
	inv.Version++
	return nil
}

// MarkExpired expires an unpaid or partially-paid invoice whose timer elapsed.
func (inv *Invoice) MarkExpired(now time.Time) error {
	if inv.Status == StatusSettled || inv.Status == StatusOverpaid {
		return fmt.Errorf("%w: settled invoices cannot expire", ErrIllegalTransition)
	}

	if inv.Status == StatusExpired || inv.Status == StatusCanceled {
		return nil // idempotent
	}

	inv.Status = StatusExpired
	inv.Version++
	return nil
}

// MarkCanceled voids a pending invoice.
func (inv *Invoice) MarkCanceled() error {
	if inv.Status == StatusSettled || inv.Status == StatusOverpaid {
		return fmt.Errorf("%w: settled invoices cannot be canceled", ErrIllegalTransition)
	}

	if inv.Status != StatusPending && inv.Status != StatusPartiallyPaid {
		return fmt.Errorf("%w: cannot cancel invoice with status %s", ErrIllegalTransition, inv.Status)
	}

	inv.Status = StatusCanceled
	inv.Version++
	return nil
}

// IsTerminal returns true if the invoice has reached an immutable end state.
func (inv *Invoice) IsTerminal() bool {
	return inv.Status == StatusSettled || inv.Status == StatusOverpaid || inv.Status == StatusExpired || inv.Status == StatusCanceled
}

// IsExpired checks if the current time exceeds the expiration timestamp.
func (inv *Invoice) IsExpired(now time.Time) bool {
	return now.After(inv.ExpiresAt) && !inv.IsTerminal()
}

// generateInvoiceID generates a secure prefix + 16-hex character ID: "inv_4a9b81f01c2d3e4f".
func generateInvoiceID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "inv_" + hex.EncodeToString(b), nil
}
