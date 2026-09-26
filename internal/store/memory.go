package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

// MemoryStore is an in-memory, thread-safe implementation of InvoiceStore.
type MemoryStore struct {
	mu       sync.RWMutex
	invoices map[string]*domain.Invoice // primary key: invoice ID
	tagIndex map[string]string          // composite key: "merchant:tag" -> invoice ID
	nextTag  map[string]uint32          // merchant -> next tag counter (starts at 1000)
}

// NewMemoryStore creates an initialized in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		invoices: make(map[string]*domain.Invoice),
		tagIndex: make(map[string]string),
		nextTag:  make(map[string]uint32),
	}
}

// Save inserts a new invoice into the store.
func (s *MemoryStore) Save(ctx context.Context, inv *domain.Invoice) error {
	if inv == nil {
		return fmt.Errorf("cannot save nil invoice")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.invoices[inv.ID]; exists {
		return ErrConflict
	}

	tagKey := makeTagKey(inv.MerchantAddr, inv.DestinationTag)
	if existingID, inUse := s.tagIndex[tagKey]; inUse {
		// If tag is already indexed to an active invoice, prevent reuse
		if existingInv, ok := s.invoices[existingID]; ok && !existingInv.IsTerminal() {
			return ErrTagInUse
		}
	}

	// Clone to prevent external mutation
	cloned := cloneInvoice(inv)
	s.invoices[inv.ID] = cloned
	s.tagIndex[tagKey] = inv.ID

	return nil
}

// Update updates an existing invoice with optimistic concurrency control.
func (s *MemoryStore) Update(ctx context.Context, inv *domain.Invoice) error {
	if inv == nil {
		return fmt.Errorf("cannot update nil invoice")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.invoices[inv.ID]
	if !exists {
		return ErrNotFound
	}

	// Optimistic locking check: the existing version must be exactly inv.Version - 1
	if existing.Version != inv.Version-1 && existing.Version != inv.Version {
		return ErrVersionMismatch
	}

	s.invoices[inv.ID] = cloneInvoice(inv)
	return nil
}

// GetByID retrieves a cloned invoice by its ID.
func (s *MemoryStore) GetByID(ctx context.Context, id string) (*domain.Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inv, exists := s.invoices[id]
	if !exists {
		return nil, ErrNotFound
	}

	return cloneInvoice(inv), nil
}

// GetByTag retrieves an invoice by merchant address and destination tag.
func (s *MemoryStore) GetByTag(ctx context.Context, merchantAddr string, tag uint32) (*domain.Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagKey := makeTagKey(merchantAddr, tag)
	id, exists := s.tagIndex[tagKey]
	if !exists {
		return nil, ErrNotFound
	}

	inv, exists := s.invoices[id]
	if !exists {
		return nil, ErrNotFound
	}

	return cloneInvoice(inv), nil
}

// AllocateTag generates the next unique destination tag for a merchant.
func (s *MemoryStore) AllocateTag(ctx context.Context, merchantAddr string) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	curr, ok := s.nextTag[merchantAddr]
	if !ok || curr < 1000 {
		curr = 1000 // start destination tags at 1000 for clarity
	}

	curr++
	s.nextTag[merchantAddr] = curr
	return curr, nil
}

// ListPending returns all non-terminal invoices.
func (s *MemoryStore) ListPending(ctx context.Context) ([]*domain.Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []*domain.Invoice
	for _, inv := range s.invoices {
		if !inv.IsTerminal() {
			pending = append(pending, cloneInvoice(inv))
		}
	}
	return pending, nil
}

func makeTagKey(merchantAddr string, tag uint32) string {
	return fmt.Sprintf("%s:%d", merchantAddr, tag)
}

// cloneInvoice creates a deep copy to ensure memory safety across goroutines.
func cloneInvoice(src *domain.Invoice) *domain.Invoice {
	dst := *src
	if src.SettledAt != nil {
		t := *src.SettledAt
		dst.SettledAt = &t
	}
	if src.Metadata != nil {
		dst.Metadata = make(map[string]string, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}
	return &dst
}
