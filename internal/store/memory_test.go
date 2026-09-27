package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

const testMerchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

func TestMemoryStore_BasicCRUD(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(1000)

	tag, err := s.AllocateTag(ctx, testMerchantAddr)
	if err != nil {
		t.Fatalf("failed allocating tag: %v", err)
	}

	now := time.Now().UTC()
	inv, err := domain.NewInvoice("ORD-1", testMerchantAddr, tag, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
	if err != nil {
		t.Fatalf("failed creating invoice: %v", err)
	}

	if err := s.Save(ctx, inv); err != nil {
		t.Fatalf("failed saving invoice: %v", err)
	}

	// Retrieve by ID
	got, err := s.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed retrieving invoice: %v", err)
	}
	if got.ID != inv.ID || got.AmountDrops != domain.Drops(5000000) {
		t.Fatalf("retrieved invoice mismatch: %+v", got)
	}

	// Retrieve by Tag
	gotByTag, err := s.GetByTag(ctx, testMerchantAddr, tag)
	if err != nil {
		t.Fatalf("failed retrieving invoice by tag: %v", err)
	}
	if gotByTag.ID != inv.ID {
		t.Fatalf("retrieved invoice by tag mismatch: got %s, want %s", gotByTag.ID, inv.ID)
	}
}

func TestMemoryStore_OptimisticLocking(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(1000)

	tag, _ := s.AllocateTag(ctx, testMerchantAddr)
	now := time.Now().UTC()
	inv, _ := domain.NewInvoice("ORD-LOCK", testMerchantAddr, tag, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
	_ = s.Save(ctx, inv)

	// Clone 1 updates successfully
	clone1, _ := s.GetByID(ctx, inv.ID)
	_ = clone1.ApplyPayment(domain.Drops(2000000), "TX_1", now.Add(time.Minute))
	if err := s.Update(ctx, clone1); err != nil {
		t.Fatalf("first update should succeed: %v", err)
	}

	// Clone 2 (stale version) tries to update -> conflict
	clone2, _ := s.GetByID(ctx, inv.ID)
	clone2.Version = 1 // Force stale version
	if err := s.Update(ctx, clone2); err != store.ErrVersionMismatch {
		t.Fatalf("expected ErrVersionMismatch, got %v", err)
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(1000)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tag, err := s.AllocateTag(ctx, testMerchantAddr)
			if err != nil {
				return
			}
			now := time.Now().UTC()
			inv, err := domain.NewInvoice("ORD-CONC", testMerchantAddr, tag, domain.Drops(1000000), 10*time.Minute, now, "", "", nil)
			if err == nil {
				_ = s.Save(ctx, inv)
				_, _ = s.GetByID(ctx, inv.ID)
			}
		}(i)
	}
	wg.Wait()
}
