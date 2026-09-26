package xrpl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

// Sentinel errors for poller lifecycle and operations.
var (
	ErrPollerAlreadyRunning = errors.New("poller is already running")
	ErrPollerNotRunning     = errors.New("poller is not running")
	ErrInvalidConfig        = errors.New("invalid poller configuration")
)

// PaymentFetcher defines the XRPL client capabilities required by the Poller.
// Following Charter §3.2 (interfaces defined in consumer packages).
type PaymentFetcher interface {
	GetAccountPayments(ctx context.Context, account string, minLedger int64) ([]PaymentEvent, error)
}

// InvoiceRepository defines the persistence capabilities required by the Poller.
type InvoiceRepository interface {
	GetByTag(ctx context.Context, merchantAddr string, tag uint32) (*domain.Invoice, error)
	Update(ctx context.Context, inv *domain.Invoice) error
	ListPending(ctx context.Context) ([]*domain.Invoice, error)
}

// PaymentListener is a callback invoked whenever an invoice changes status due to an on-chain payment.
type PaymentListener func(ctx context.Context, inv *domain.Invoice, event PaymentEvent)

// Clock abstracts time for deterministic testing without time.Sleep (Charter §2.2).
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// PollerConfig holds tunable configuration parameters for the ledger monitor.
type PollerConfig struct {
	MerchantAccount string
	PollInterval    time.Duration
	StartLedger     int64
	Logger          *slog.Logger
	Clock           Clock
	OnPayment       PaymentListener
}

// Poller runs a reconciliation loop between the XRPL ledger and the Invoice store.
type Poller struct {
	fetcher     PaymentFetcher
	store       InvoiceRepository
	cfg         PollerConfig
	logger      *slog.Logger
	clock       Clock
	lastLedger  int64 // accessed atomically
	running     atomic.Bool
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	processedTx map[string]struct{}
}

// NewPoller constructs and validates a new Poller instance.
func NewPoller(fetcher PaymentFetcher, repo InvoiceRepository, cfg PollerConfig) (*Poller, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("%w: fetcher cannot be nil", ErrInvalidConfig)
	}
	if repo == nil {
		return nil, fmt.Errorf("%w: store cannot be nil", ErrInvalidConfig)
	}
	if len(cfg.MerchantAccount) < 25 || len(cfg.MerchantAccount) > 35 || cfg.MerchantAccount[0] != 'r' {
		return nil, fmt.Errorf("%w: invalid merchant address %q", ErrInvalidConfig, cfg.MerchantAccount)
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 3 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = realClock{}
	}

	p := &Poller{
		fetcher:     fetcher,
		store:       repo,
		cfg:         cfg,
		logger:      cfg.Logger.With("component", "xrpl.poller", "merchant", cfg.MerchantAccount),
		clock:       cfg.Clock,
		lastLedger:  cfg.StartLedger,
		stopChan:    make(chan struct{}),
		processedTx: make(map[string]struct{}),
	}

	return p, nil
}

// LastLedger returns the highest validated ledger index processed so far.
func (p *Poller) LastLedger() int64 {
	return atomic.LoadInt64(&p.lastLedger)
}

// PollOnce executes a single reconciliation cycle:
// 1. Fetches on-chain payments since lastLedger.
// 2. Matches destination tags to active invoices and applies payments.
// 3. Sweeps and expires stale pending invoices.
func (p *Poller) PollOnce(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	currentMinLedger := atomic.LoadInt64(&p.lastLedger)

	// 1. Fetch on-chain payments
	payments, err := p.fetcher.GetAccountPayments(ctx, p.cfg.MerchantAccount, currentMinLedger)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to fetch ledger payments", "error", err, "min_ledger", currentMinLedger)
		return fmt.Errorf("fetch payments: %w", err)
	}

	var maxLedgerSeen int64 = currentMinLedger

	// 2. Reconcile payments against invoices
	for _, payment := range payments {
		if int64(payment.LedgerIndex) > maxLedgerSeen {
			maxLedgerSeen = int64(payment.LedgerIndex)
		}

		// Skip if transaction was already processed by this poller instance
		if _, exists := p.processedTx[payment.TxHash]; exists {
			continue
		}

		// Destination tag 0 means untagged payment to merchant root account
		if payment.DestinationTag == 0 {
			p.logger.WarnContext(ctx, "received untagged payment to merchant wallet; skipping invoice match",
				"tx_hash", payment.TxHash,
				"drops", payment.DeliveredDrops,
			)
			p.processedTx[payment.TxHash] = struct{}{}
			continue
		}

		inv, err := p.store.GetByTag(ctx, payment.Destination, payment.DestinationTag)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				p.logger.DebugContext(ctx, "payment tag not matched to active invoice",
					"tag", payment.DestinationTag,
					"tx_hash", payment.TxHash,
				)
				p.processedTx[payment.TxHash] = struct{}{}
				continue
			}
			p.logger.ErrorContext(ctx, "store lookup failed for tag", "tag", payment.DestinationTag, "error", err)
			continue
		}

		// If invoice already reached terminal state before this payment
		if inv.IsTerminal() {
			p.logger.WarnContext(ctx, "payment received for already terminal invoice",
				"invoice_id", inv.ID,
				"status", inv.Status,
				"tx_hash", payment.TxHash,
			)
			p.processedTx[payment.TxHash] = struct{}{}
			continue
		}

		// Apply payment to domain state machine
		now := p.clock.Now()
		if applyErr := inv.ApplyPayment(payment.DeliveredDrops, payment.TxHash, now); applyErr != nil {
			p.logger.ErrorContext(ctx, "failed to apply payment to invoice",
				"invoice_id", inv.ID,
				"error", applyErr,
			)
			continue
		}

		// Persist state change with optimistic concurrency control
		if updateErr := p.store.Update(ctx, inv); updateErr != nil {
			p.logger.ErrorContext(ctx, "failed to persist invoice state update",
				"invoice_id", inv.ID,
				"error", updateErr,
			)
			continue
		}

		p.processedTx[payment.TxHash] = struct{}{}
		p.logger.InfoContext(ctx, "invoice payment processed successfully",
			"invoice_id", inv.ID,
			"order_id", inv.OrderID,
			"new_status", inv.Status,
			"amount_paid", inv.AmountPaid.String(),
			"tx_hash", payment.TxHash,
		)

		// Fire listener (for Webhook Dispatcher in Phase 3)
		if p.cfg.OnPayment != nil {
			p.cfg.OnPayment(ctx, inv, payment)
		}
	}

	// Advance ledger bookmark if we saw new validated ledgers
	if maxLedgerSeen > currentMinLedger {
		atomic.StoreInt64(&p.lastLedger, maxLedgerSeen)
	}

	// 3. Sweep and expire overdue pending invoices
	now := p.clock.Now()
	pendingInvoices, err := p.store.ListPending(ctx)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to list pending invoices for expiration sweep", "error", err)
		return fmt.Errorf("list pending invoices: %w", err)
	}

	for _, inv := range pendingInvoices {
		if inv.IsExpired(now) {
			if expireErr := inv.MarkExpired(now); expireErr != nil {
				p.logger.ErrorContext(ctx, "failed to mark invoice expired", "invoice_id", inv.ID, "error", expireErr)
				continue
			}
			if updateErr := p.store.Update(ctx, inv); updateErr != nil {
				p.logger.ErrorContext(ctx, "failed to update expired invoice", "invoice_id", inv.ID, "error", updateErr)
				continue
			}
			p.logger.InfoContext(ctx, "invoice marked expired",
				"invoice_id", inv.ID,
				"order_id", inv.OrderID,
				"expired_at", now,
			)
		}
	}

	return nil
}

// Start launches the background polling goroutine.
// It returns immediately and runs until Stop is called or ctx is canceled.
func (p *Poller) Start(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		return ErrPollerAlreadyRunning
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.running.Store(false)

		ticker := time.NewTicker(p.cfg.PollInterval)
		defer ticker.Stop()

		p.logger.Info("ledger poller started", "poll_interval", p.cfg.PollInterval)

		for {
			select {
			case <-ctx.Done():
				p.logger.Info("poller stopping due to context cancellation")
				return
			case <-p.stopChan:
				p.logger.Info("poller stopping due to explicit stop signal")
				return
			case <-ticker.C:
				if err := p.PollOnce(ctx); err != nil {
					p.logger.ErrorContext(ctx, "reconciliation pass encountered error", "error", err)
				}
			}
		}
	}()

	return nil
}

// Stop gracefully signals the poller to terminate and waits for the worker goroutine to exit.
func (p *Poller) Stop() error {
	if !p.running.Load() {
		return ErrPollerNotRunning
	}

	close(p.stopChan)
	p.wg.Wait()
	p.logger.Info("ledger poller stopped cleanly")
	return nil
}
