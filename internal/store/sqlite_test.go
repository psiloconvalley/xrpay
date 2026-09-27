package store_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

func TestSQLiteStore_CRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xrpay-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_xrpay.db")
	db, err := store.NewSQLiteStore("file:" + dbPath + "?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("failed creating sqlite store: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	merchant := "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

	// 1. Allocate Tag
	tag1, err := db.AllocateTag(ctx, merchant)
	if err != nil {
		t.Fatalf("failed allocating tag: %v", err)
	}
	if tag1 != 1000 {
		t.Errorf("expected tag 1000, got %d", tag1)
	}

	tag2, err := db.AllocateTag(ctx, merchant)
	if err != nil {
		t.Fatalf("failed allocating tag: %v", err)
	}
	if tag2 != 1001 {
		t.Errorf("expected tag 1001, got %d", tag2)
	}

	// 2. Save Invoice
	now := time.Now().UTC().Truncate(time.Second)
	inv, err := domain.NewInvoice("ORD-1", merchant, tag1, domain.Drops(10000000), 15*time.Minute, now, "http://ref.com", "http://web.com", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("failed creating invoice: %v", err)
	}

	if err := db.Save(ctx, inv); err != nil {
		t.Fatalf("failed saving invoice: %v", err)
	}

	// 3. Conflict Save (ID Conflict)
	if err := db.Save(ctx, inv); err == nil {
		t.Errorf("expected ID conflict, got nil")
	}

	// 4. Duplicate Active Tag Check
	invDup, err := domain.NewInvoice("ORD-2", merchant, tag1, domain.Drops(5000000), 15*time.Minute, now, "", "", nil)
	if err != nil {
		t.Fatalf("failed creating duplicate invoice: %v", err)
	}
	if err := db.Save(ctx, invDup); err == nil {
		t.Errorf("expected duplicate active tag conflict, got nil")
	}

	// 5. Get By ID
	retrieved, err := db.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed retrieving invoice: %v", err)
	}
	if retrieved.ID != inv.ID || retrieved.Metadata["foo"] != "bar" {
		t.Errorf("mismatched retrieved data: %+v", retrieved)
	}

	// 6. Get By Tag
	retrievedTag, err := db.GetByTag(ctx, merchant, tag1)
	if err != nil {
		t.Fatalf("failed retrieving invoice by tag: %v", err)
	}
	if retrievedTag.ID != inv.ID {
		t.Errorf("mismatched retrieved tag ID: got %s, want %s", retrievedTag.ID, inv.ID)
	}

	// 7. Update with optimistic lock
	if err := retrieved.ApplyPayment(domain.Drops(5000000), "TXHASH123", now); err != nil {
		t.Fatalf("failed applying payment: %v", err)
	}

	if err := db.Update(ctx, retrieved); err != nil {
		t.Fatalf("failed updating invoice: %v", err)
	}

	// Verify update changes are persisted
	updated, err := db.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("failed retrieving updated invoice: %v", err)
	}
	if updated.PaidDrops != domain.Drops(5000000) || updated.Status != domain.StatusPartiallyPaid {
		t.Errorf("mismatched updated states: %+v", updated)
	}

	// Verify optimistic lock error on stale version update
	staleUpdate := *retrieved
	staleUpdate.Version = 1 // Outdated version directly presented to update
	if err := db.Update(ctx, &staleUpdate); err == nil {
		t.Errorf("expected version mismatch error on stale update, got nil")
	}

	// 8. Ledger bookmark state check
	last, err := db.GetLastProcessedLedger(ctx)
	if err != nil {
		t.Fatalf("failed getting initial last ledger: %v", err)
	}
	if last != -1 {
		t.Errorf("expected default -1, got %d", last)
	}

	if err := db.SetLastProcessedLedger(ctx, 45129938); err != nil {
		t.Fatalf("failed saving last processed ledger: %v", err)
	}

	last, err = db.GetLastProcessedLedger(ctx)
	if err != nil {
		t.Fatalf("failed retrieving updated ledger: %v", err)
	}
	if last != 45129938 {
		t.Errorf("expected 45129938, got %d", last)
	}
}

func TestSQLiteStore_ConcurrentTagAllocation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xrpay-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_concurrent_xrpay.db")
	db, err := store.NewSQLiteStore("file:" + dbPath)
	if err != nil {
		t.Fatalf("failed creating store: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	merchant := "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"

	var wg sync.WaitGroup
	workers := 50
	allocatedTags := make(chan uint32, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tag, err := db.AllocateTag(ctx, merchant)
			if err != nil {
				t.Errorf("error allocating tag concurrently: %v", err)
				return
			}
			allocatedTags <- tag
		}()
	}

	wg.Wait()
	close(allocatedTags)

	seen := make(map[uint32]bool)
	for tag := range allocatedTags {
		if seen[tag] {
			t.Errorf("duplicate tag allocated: %d", tag)
		}
		seen[tag] = true
	}

	if len(seen) != workers {
		t.Errorf("expected %d unique tags allocated, got %d", workers, len(seen))
	}
}
