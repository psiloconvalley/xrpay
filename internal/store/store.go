package store

import (
	"context"
	"errors"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

var (
	ErrNotFound        = errors.New("invoice not found")
	ErrConflict        = errors.New("invoice already exists")
	ErrVersionMismatch = errors.New("optimistic lock failed: version mismatch")
	ErrTagInUse        = errors.New("destination tag is currently in use by an active invoice")
)

// InvoiceStore defines the persistence contract for managing invoices.
// Every method accepts context.Context for timeout and cancellation propagation.
type InvoiceStore interface {
	// Save inserts a new invoice. Returns ErrConflict if an invoice with the same ID already exists.
	Save(ctx context.Context, inv *domain.Invoice) error

	// Update updates an existing invoice with optimistic concurrency control.
	// Returns ErrVersionMismatch if the version in storage does not match inv.Version - 1.
	Update(ctx context.Context, inv *domain.Invoice) error

	// GetByID retrieves an invoice by its unique ID (e.g., "inv_...").
	GetByID(ctx context.Context, id string) (*domain.Invoice, error)

	// GetByTag retrieves an active invoice matching a merchant address and destination tag.
	GetByTag(ctx context.Context, merchantAddr string, tag uint32) (*domain.Invoice, error)

	// AllocateTag reserves the next available destination tag for a merchant address.
	AllocateTag(ctx context.Context, merchantAddr string) (uint32, error)

	// ListPending returns all non-terminal invoices that need active monitoring or expiration checks.
	ListPending(ctx context.Context) ([]*domain.Invoice, error)
}
