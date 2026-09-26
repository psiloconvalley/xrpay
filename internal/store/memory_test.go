package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

const testMerchant = "rPT1Sjq2YGrBMTttX4GZHjKu9DYfzbpAYe"

func TestMemoryStore_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()

	drops, _ := domain.ParseXRP("10")
	inv, _ := domain.NewInvoice("order_1", testMerchant, 1001, drops, 15*time.Minute, "", nil)

	// Save
	if err := s.Save(ctx, inv); err != nil {
		t.Fatalf("Save unexpected error: %v", err)
	}

	// Duplicate Save should fail
	if err := s.Save(ctx, inv); err != store.ErrConflict {
		t.Fatalf("expected ErrConflict on duplicate save, got %v", err)
	}

	// GetByID
	got, err := s.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetByID unexpected error: %v", err)
	}
	if got.ID != inv.ID || got.AmountExpected != drops {
		t.Errorf("GetByID returned corrupted invoice: %+v", got)
	}

	// GetByTag
	byTag, err := s.GetByTag(ctx, testMerchant, 1001)
	if err != nil {
		t.Fatalf("GetByTag unexpected error: %v", err)
	}
	if byTag.ID != inv.ID {
		t.Errorf("GetByTag returned wrong ID: %s", byTag.ID)
	}

	// Non-existent lookups
	if _, err := s.GetByID(ctx, "inv_nonexistent"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for fake ID, got %v", err)
	}
	if _, err := s.GetByTag(ctx, testMerchant, 9999); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for fake tag, got %v", err)
	}
}

func TestMemoryStore_OptimisticLocking(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()

	drops, _ := domain.ParseXRP("10")
	inv, _ := domain.NewInvoice("order_2", testMerchant, 1002, drops, 15*time.Minute, "", nil)
	_ = s.Save(ctx, inv)

	// Correct update (inv.ApplyPayment increments Version from 1 -> 2)
	_ = inv.ApplyPayment(drops, "tx_hash_123", time.Now().UTC())
	if err := s.Update(ctx, inv); err != nil {
		t.Fatalf("Update with incremented version failed: %v", err)
	}

	// Stale update: trying to update with old version
	staleInv := *inv
	staleInv.Version = 1 // wrong version
	if err := s.Update(ctx, &staleInv); err != store.ErrVersionMismatch {
		t.Errorf("expected ErrVersionMismatch for stale update, got %v", err)
	}
}

func TestMemoryStore_ConcurrentTagAllocation(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()

	concurrency := 50
	var wg sync.WaitGroup
	tags := make(chan uint32, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tag, err := s.AllocateTag(ctx, testMerchant)
			if err != nil {
				t.Errorf("AllocateTag error: %v", err)
				return
			}
			tags <- tag
		}()
	}

	wg.Wait()
	close(tags)

	// Ensure all 50 allocated tags are strictly unique
	seen := make(map[uint32]bool)
	for tag := range tags {
		if seen[tag] {
			t.Fatalf("duplicate tag allocated: %d", tag)
		}
		seen[tag] = true
	}
}
