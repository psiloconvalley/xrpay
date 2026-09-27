package xrpl

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/psiloconvalley/xrpay/internal/store"
	"github.com/psiloconvalley/xrpay/internal/webhook"
)

// BoundedTxCache stores up to maxSize transaction hashes with FIFO eviction.
type BoundedTxCache struct {
	mu      sync.RWMutex
	maxSize int
	order   []string
	set     map[string]struct{}
}

// NewBoundedTxCache constructs a bounded in-memory cache for transaction deduplication.
func NewBoundedTxCache(maxSize int) *BoundedTxCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &BoundedTxCache{
		maxSize: maxSize,
		order:   make([]string, 0, maxSize),
		set:     make(map[string]struct{}, maxSize),
	}
}

// Has checks whether the transaction hash has already been processed.
func (c *BoundedTxCache) Has(hash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.set[hash]
	return exists
}

// Add inserts a transaction hash, evicting the oldest entry when at capacity.
func (c *BoundedTxCache) Add(hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.set[hash]; exists {
		return
	}

	if len(c.order) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.set, oldest)
	}

	c.order = append(c.order, hash)
	c.set[hash] = struct{}{}
}

// Poller monitors ledger transactions for a merchant address and reconciles payments.
type Poller struct {
	client          *Client
	store           store.InvoiceStore
	dispatcher      *webhook.Dispatcher
	merchantAddress string
	pollInterval    time.Duration
	lastLedger      int64
	txCache         *BoundedTxCache
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewPoller constructs a new ledger polling worker.
func NewPoller(
	client *Client,
	invoiceStore store.InvoiceStore,
	dispatcher *webhook.Dispatcher,
	merchantAddress string,
	pollInterval time.Duration,
	startLedger int64,
) *Poller {
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	return &Poller{
		client:          client,
		store:           invoiceStore,
		dispatcher:      dispatcher,
		merchantAddress: merchantAddress,
		pollInterval:    pollInterval,
		lastLedger:      startLedger,
		txCache:         NewBoundedTxCache(10000),
		stopCh:          make(chan struct{}),
	}
}

// Start spawns the poller background worker goroutine.
func (p *Poller) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.run(ctx)
}

// Stop signals the poller to terminate and waits for the run loop to finish.
func (p *Poller) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *Poller) run(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.pollInterval)
	sweepTicker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer sweepTicker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.reconcile(ctx); err != nil {
				if errors.Is(err, ErrAccountNotFound) {
					slog.Debug("merchant account unfunded on ledger", "address", p.merchantAddress)
				} else {
					slog.Error("ledger reconciliation error", "err", err)
				}
			}
		case <-sweepTicker.C:
			p.sweepExpired(ctx)
		}
	}
}

func (p *Poller) reconcile(ctx context.Context) error {
	payments, latestLedger, err := p.client.GetAccountPayments(ctx, p.merchantAddress, p.lastLedger, -1)
	if err != nil {
		return err
	}

	if latestLedger > p.lastLedger {
		p.lastLedger = latestLedger
	}

	for _, payment := range payments {
		if payment.DestinationTag == 0 {
			continue
		}

		if p.txCache.Has(payment.TxHash) {
			continue
		}

		inv, err := p.store.GetByTag(ctx, p.merchantAddress, payment.DestinationTag)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			slog.Error("store lookup error for destination tag", "tag", payment.DestinationTag, "err", err)
			continue
		}

		if inv.Status.IsTerminal() {
			p.txCache.Add(payment.TxHash)
			continue
		}

		if err := inv.ApplyPayment(payment.DeliveredDrops, payment.TxHash, payment.Timestamp); err != nil {
			slog.Warn("failed applying payment to invoice", "invoice_id", inv.ID, "err", err)
			continue
		}

		if err := p.store.Update(ctx, inv); err != nil {
			slog.Error("failed updating invoice after payment", "invoice_id", inv.ID, "err", err)
			continue
		}

		p.txCache.Add(payment.TxHash)

		slog.Info("payment reconciled",
			"invoice_id", inv.ID,
			"status", inv.Status,
			"tag", inv.DestinationTag,
			"paid_drops", inv.PaidDrops,
			"tx_hash", payment.TxHash,
		)

		if p.dispatcher != nil && inv.WebhookURL != "" {
			p.dispatcher.DispatchAsync(inv.WebhookURL, webhook.EventInvoiceSettled, inv)
		}
	}

	return nil
}

func (p *Poller) sweepExpired(ctx context.Context) {
	invoices, err := p.store.ListPending(ctx)
	if err != nil {
		return
	}

	now := time.Now().UTC()
	for _, inv := range invoices {
		if inv.Expire(now) {
			if err := p.store.Update(ctx, inv); err == nil {
				slog.Info("invoice expired during sweep", "invoice_id", inv.ID)
				if p.dispatcher != nil && inv.WebhookURL != "" {
					p.dispatcher.DispatchAsync(inv.WebhookURL, webhook.EventInvoiceExpired, inv)
				}
			}
		}
	}
}
