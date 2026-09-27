package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

type MemoryStore struct {
	mu       sync.RWMutex
	byID     map[string]*domain.Invoice
	byTag    map[string]string // key: "merchantAddr:tag" -> invoice ID
	nextTags map[string]uint32 // key: merchantAddr -> next tag
}

func NewMemoryStore(startTag uint32) *MemoryStore {
	if startTag == 0 {
		startTag = 1000
	}
	return &MemoryStore{
		byID:     make(map[string]*domain.Invoice),
		byTag:    make(map[string]string),
		nextTags: make(map[string]uint32),
	}
}

func tagKey(merchantAddr string, tag uint32) string {
	return fmt.Sprintf("%s:%d", merchantAddr, tag)
}

func (m *MemoryStore) AllocateTag(ctx context.Context, merchantAddr string) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, exists := m.nextTags[merchantAddr]
	if !exists || tag == 0 {
		tag = 1000
	}

	for {
		candidate := tag
		tag++
		if tag > 4294967295 {
			tag = 1000
		}
		m.nextTags[merchantAddr] = tag

		k := tagKey(merchantAddr, candidate)
		if existingID, exists := m.byTag[k]; exists {
			if existingInv, ok := m.byID[existingID]; ok && !existingInv.IsTerminal() {
				continue
			}
		}
		return candidate, nil
	}
}

func (m *MemoryStore) Save(ctx context.Context, inv *domain.Invoice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byID[inv.ID]; exists {
		return ErrConflict
	}

	k := tagKey(inv.MerchantAddress, inv.DestinationTag)
	if existingID, exists := m.byTag[k]; exists {
		if existingInv, ok := m.byID[existingID]; ok && !existingInv.IsTerminal() {
			return ErrTagInUse
		}
	}

	copied := *inv
	m.byID[inv.ID] = &copied
	m.byTag[k] = inv.ID
	return nil
}

func (m *MemoryStore) GetByID(ctx context.Context, id string) (*domain.Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inv, exists := m.byID[id]
	if !exists {
		return nil, ErrNotFound
	}
	copied := *inv
	return &copied, nil
}

func (m *MemoryStore) GetByTag(ctx context.Context, merchantAddr string, tag uint32) (*domain.Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := tagKey(merchantAddr, tag)
	id, exists := m.byTag[k]
	if !exists {
		return nil, ErrNotFound
	}
	inv, exists := m.byID[id]
	if !exists {
		return nil, ErrNotFound
	}
	copied := *inv
	return &copied, nil
}

func (m *MemoryStore) Update(ctx context.Context, inv *domain.Invoice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.byID[inv.ID]
	if !exists {
		return ErrNotFound
	}

	if existing.Version != inv.Version-1 {
		return ErrVersionMismatch
	}

	copied := *inv
	m.byID[inv.ID] = &copied
	return nil
}

func (m *MemoryStore) ListPending(ctx context.Context) ([]*domain.Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pending := make([]*domain.Invoice, 0)
	for _, inv := range m.byID {
		if !inv.IsTerminal() {
			copied := *inv
			pending = append(pending, &copied)
		}
	}
	return pending, nil
}
