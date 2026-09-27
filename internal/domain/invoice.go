package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvoiceNotFound        = errors.New("invoice not found")
	ErrInvalidInvoiceAmount   = errors.New("invoice amount must be strictly positive")
	ErrInvoiceExpired         = errors.New("invoice has expired")
	ErrInvoiceAlreadySettled   = errors.New("invoice is already settled")
	ErrInvoiceTerminalState   = errors.New("invoice is in a terminal state")
	ErrOptimisticLockConflict = errors.New("optimistic lock conflict: version mismatch")
	ErrMetadataLimitExceeded  = errors.New("metadata limit exceeded")
)

type InvoiceStatus string

const (
	StatusPending       InvoiceStatus = "PENDING"
	StatusPartiallyPaid InvoiceStatus = "PARTIALLY_PAID"
	StatusSettled       InvoiceStatus = "SETTLED"
	StatusOverpaid      InvoiceStatus = "OVERPAID"
	StatusExpired       InvoiceStatus = "EXPIRED"
	StatusCancelled     InvoiceStatus = "CANCELLED"
	StatusRefunded      InvoiceStatus = "REFUNDED"
)

func (s InvoiceStatus) IsTerminal() bool {
	switch s {
	case StatusSettled, StatusOverpaid, StatusExpired, StatusCancelled, StatusRefunded:
		return true
	default:
		return false
	}
}

// PaymentRecord captures an individual on-chain payment event.
type PaymentRecord struct {
	TxHash         string    `json:"tx_hash"`
	DeliveredDrops Drops     `json:"delivered_drops"`
	DeliveredXRP   string    `json:"delivered_xrp"`
	Timestamp      time.Time `json:"timestamp"`
}

// Invoice represents a merchant billing request settled via XRPL.
type Invoice struct {
	ID              string            `json:"id"`
	OrderID         string            `json:"order_id"`
	MerchantAddress string            `json:"merchant_address"`
	DestinationTag  uint32            `json:"destination_tag"`
	AmountDrops     Drops             `json:"amount_drops"`
	AmountXRP       string            `json:"amount_xrp"`
	PaidDrops       Drops             `json:"paid_drops"`
	PaidXRP         string            `json:"paid_xrp"`
	Status          InvoiceStatus     `json:"status"`
	RedirectURL     string            `json:"redirect_url,omitempty"`
	WebhookURL      string            `json:"webhook_url,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	TxHash          string            `json:"tx_hash,omitempty"`
	Payments        []PaymentRecord   `json:"payments,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	ExpiresAt       time.Time         `json:"expires_at"`
	SettledAt       *time.Time        `json:"settled_at,omitempty"`
	Version         uint64            `json:"version"`
}

// IsTerminal checks if the invoice has reached a final state.
func (inv *Invoice) IsTerminal() bool {
	return inv.Status.IsTerminal()
}

// NewInvoice creates a new pending invoice.
func NewInvoice(
	orderID string,
	merchantAddr string,
	destTag uint32,
	amount Drops,
	expiry time.Duration,
	now time.Time,
	redirectURL string,
	webhookURL string,
	metadata map[string]string,
) (*Invoice, error) {
	if amount <= 0 {
		return nil, ErrInvalidInvoiceAmount
	}

	if err := ValidateXRPLAddress(merchantAddr); err != nil {
		return nil, fmt.Errorf("invalid merchant address: %w", err)
	}

	if len(metadata) > 20 {
		return nil, fmt.Errorf("%w: max 20 keys allowed", ErrMetadataLimitExceeded)
	}
	for k, v := range metadata {
		if len(k) > 64 || len(v) > 500 {
			return nil, fmt.Errorf("%w: key max 64 chars, value max 500 chars", ErrMetadataLimitExceeded)
		}
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed generating invoice id: %w", err)
	}

	return &Invoice{
		ID:              fmt.Sprintf("inv_%s", hex.EncodeToString(idBytes)),
		OrderID:         orderID,
		MerchantAddress: merchantAddr,
		DestinationTag:  destTag,
		AmountDrops:     amount,
		AmountXRP:       amount.ToXRPString(),
		PaidDrops:       0,
		PaidXRP:         Drops(0).ToXRPString(),
		Status:          StatusPending,
		RedirectURL:     redirectURL,
		WebhookURL:      webhookURL,
		Metadata:        metadata,
		Payments:        make([]PaymentRecord, 0),
		CreatedAt:       now,
		ExpiresAt:       now.Add(expiry),
		Version:         1,
	}, nil
}

// ApplyPayment transitions invoice state on payment receipt.
func (inv *Invoice) ApplyPayment(deliveredDrops Drops, txHash string, receivedAt time.Time) error {
	if inv.Status.IsTerminal() {
		return fmt.Errorf("%w: cannot apply payment to invoice in status %s", ErrInvoiceTerminalState, inv.Status)
	}

	if receivedAt.After(inv.ExpiresAt) {
		inv.Status = StatusExpired
		return ErrInvoiceExpired
	}

	inv.PaidDrops += deliveredDrops
	inv.PaidXRP = inv.PaidDrops.ToXRPString()
	inv.TxHash = txHash

	// Record payment in ledger audit history
	inv.Payments = append(inv.Payments, PaymentRecord{
		TxHash:         txHash,
		DeliveredDrops: deliveredDrops,
		DeliveredXRP:   deliveredDrops.ToXRPString(),
		Timestamp:      receivedAt,
	})

	if inv.PaidDrops >= inv.AmountDrops {
		if inv.PaidDrops > inv.AmountDrops {
			inv.Status = StatusOverpaid
		} else {
			inv.Status = StatusSettled
		}
		inv.SettledAt = &receivedAt
	} else if inv.PaidDrops > 0 {
		inv.Status = StatusPartiallyPaid
	}

	inv.Version++
	return nil
}

// Expire marks an open invoice as expired if current time is past expiry.
func (inv *Invoice) Expire(now time.Time) bool {
	if inv.Status.IsTerminal() {
		return false
	}
	if now.After(inv.ExpiresAt) {
		inv.Status = StatusExpired
		inv.Version++
		return true
	}
	return false
}
