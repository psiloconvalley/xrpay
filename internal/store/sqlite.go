package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/psiloconvalley/xrpay/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
	mu sync.Mutex // Strict lock for tag allocation sequencing
}

// NewSQLiteStore initializes and migrates the SQLite embedded database.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening sqlite db: %w", err)
	}

	// Apply highly-optimized production SQLite PRAGMAs
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed applying sqlite pragma %q: %w", pragma, err)
		}
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed migrating sqlite tables: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS invoices (
		id TEXT PRIMARY KEY,
		order_id TEXT NOT NULL,
		merchant_address TEXT NOT NULL,
		destination_tag INTEGER NOT NULL,
		amount_drops INTEGER NOT NULL,
		amount_xrp TEXT NOT NULL,
		paid_drops INTEGER NOT NULL,
		paid_xrp TEXT NOT NULL,
		status TEXT NOT NULL,
		redirect_url TEXT,
		webhook_url TEXT,
		metadata TEXT,
		tx_hash TEXT,
		payments TEXT,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		settled_at DATETIME,
		version INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_invoices_merchant_tag ON invoices (merchant_address, destination_tag);
	CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices (status);

	CREATE TABLE IF NOT EXISTS gateway_state (
		key TEXT PRIMARY KEY,
		val TEXT NOT NULL
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Save(ctx context.Context, inv *domain.Invoice) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed beginning save transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Conflict check on ID
	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM invoices WHERE id = ?)", inv.ID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed checking invoice existence: %w", err)
	}
	if exists {
		return ErrConflict
	}

	// 2. Active Tag Check to avoid duplicate active tags
	var activeExists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM invoices 
		WHERE merchant_address = ? 
		  AND destination_tag = ? 
		  AND status NOT IN ('SETTLED', 'OVERPAID', 'EXPIRED', 'CANCELLED', 'REFUNDED')
	)`
	err = tx.QueryRowContext(ctx, query, inv.MerchantAddress, inv.DestinationTag).Scan(&activeExists)
	if err != nil {
		return fmt.Errorf("failed checking active tag status: %w", err)
	}
	if activeExists {
		return ErrTagInUse
	}

	// Serialize metadata & payments
	metaBytes, err := json.Marshal(inv.Metadata)
	if err != nil {
		return fmt.Errorf("failed serializing metadata: %w", err)
	}
	payBytes, err := json.Marshal(inv.Payments)
	if err != nil {
		return fmt.Errorf("failed serializing payment history: %w", err)
	}

	var settledAtVal interface{}
	if inv.SettledAt != nil {
		settledAtVal = *inv.SettledAt
	}

	insertQuery := `INSERT INTO invoices (
		id, order_id, merchant_address, destination_tag, amount_drops, amount_xrp,
		paid_drops, paid_xrp, status, redirect_url, webhook_url, metadata,
		tx_hash, payments, created_at, expires_at, settled_at, version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, insertQuery,
		inv.ID, inv.OrderID, inv.MerchantAddress, inv.DestinationTag, int64(inv.AmountDrops), inv.AmountXRP,
		int64(inv.PaidDrops), inv.PaidXRP, string(inv.Status), inv.RedirectURL, inv.WebhookURL, string(metaBytes),
		inv.TxHash, string(payBytes), inv.CreatedAt, inv.ExpiresAt, settledAtVal, inv.Version,
	)
	if err != nil {
		return fmt.Errorf("failed executing save insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed committing save transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Update(ctx context.Context, inv *domain.Invoice) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed beginning update transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var dbVersion uint64
	err = tx.QueryRowContext(ctx, "SELECT version FROM invoices WHERE id = ?", inv.ID).Scan(&dbVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed reading invoice version: %w", err)
	}

	if dbVersion != inv.Version-1 {
		return ErrVersionMismatch
	}

	metaBytes, err := json.Marshal(inv.Metadata)
	if err != nil {
		return fmt.Errorf("failed serializing metadata: %w", err)
	}
	payBytes, err := json.Marshal(inv.Payments)
	if err != nil {
		return fmt.Errorf("failed serializing payment records: %w", err)
	}

	var settledAtVal interface{}
	if inv.SettledAt != nil {
		settledAtVal = *inv.SettledAt
	}

	updateQuery := `UPDATE invoices SET
		paid_drops = ?,
		paid_xrp = ?,
		status = ?,
		metadata = ?,
		tx_hash = ?,
		payments = ?,
		settled_at = ?,
		version = ?
	WHERE id = ? AND version = ?`

	res, err := tx.ExecContext(ctx, updateQuery,
		int64(inv.PaidDrops), inv.PaidXRP, string(inv.Status), string(metaBytes), inv.TxHash, string(payBytes),
		settledAtVal, inv.Version, inv.ID, dbVersion,
	)
	if err != nil {
		return fmt.Errorf("failed executing invoice update query: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed verifying database rows affected: %w", err)
	}
	if affected == 0 {
		return ErrVersionMismatch
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed committing update transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetByID(ctx context.Context, id string) (*domain.Invoice, error) {
	query := `SELECT 
		id, order_id, merchant_address, destination_tag, amount_drops, amount_xrp,
		paid_drops, paid_xrp, status, redirect_url, webhook_url, metadata,
		tx_hash, payments, created_at, expires_at, settled_at, version
	FROM invoices WHERE id = ?`

	var inv domain.Invoice
	var amountDropsVal, paidDropsVal int64
	var statusStr, metaStr, paymentsStr string
	var settledAtVal sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&inv.ID, &inv.OrderID, &inv.MerchantAddress, &inv.DestinationTag, &amountDropsVal, &inv.AmountXRP,
		&paidDropsVal, &inv.PaidXRP, &statusStr, &inv.RedirectURL, &inv.WebhookURL, &metaStr,
		&inv.TxHash, &paymentsStr, &inv.CreatedAt, &inv.ExpiresAt, &settledAtVal, &inv.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed querying invoice by ID: %w", err)
	}

	inv.AmountDrops = domain.Drops(amountDropsVal)
	inv.PaidDrops = domain.Drops(paidDropsVal)
	inv.Status = domain.InvoiceStatus(statusStr)

	if settledAtVal.Valid {
		t := settledAtVal.Time
		inv.SettledAt = &t
	}

	if err := json.Unmarshal([]byte(metaStr), &inv.Metadata); err != nil {
		return nil, fmt.Errorf("failed decoding database metadata json: %w", err)
	}
	if err := json.Unmarshal([]byte(paymentsStr), &inv.Payments); err != nil {
		return nil, fmt.Errorf("failed decoding database payment audit history json: %w", err)
	}

	return &inv, nil
}

func (s *SQLiteStore) GetByTag(ctx context.Context, merchantAddr string, tag uint32) (*domain.Invoice, error) {
	query := `SELECT 
		id, order_id, merchant_address, destination_tag, amount_drops, amount_xrp,
		paid_drops, paid_xrp, status, redirect_url, webhook_url, metadata,
		tx_hash, payments, created_at, expires_at, settled_at, version
	FROM invoices 
	WHERE merchant_address = ? AND destination_tag = ?
	ORDER BY created_at DESC LIMIT 1`

	var inv domain.Invoice
	var amountDropsVal, paidDropsVal int64
	var statusStr, metaStr, paymentsStr string
	var settledAtVal sql.NullTime

	err := s.db.QueryRowContext(ctx, query, merchantAddr, tag).Scan(
		&inv.ID, &inv.OrderID, &inv.MerchantAddress, &inv.DestinationTag, &amountDropsVal, &inv.AmountXRP,
		&paidDropsVal, &inv.PaidXRP, &statusStr, &inv.RedirectURL, &inv.WebhookURL, &metaStr,
		&inv.TxHash, &paymentsStr, &inv.CreatedAt, &inv.ExpiresAt, &settledAtVal, &inv.Version,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed querying invoice by tag: %w", err)
	}

	inv.AmountDrops = domain.Drops(amountDropsVal)
	inv.PaidDrops = domain.Drops(paidDropsVal)
	inv.Status = domain.InvoiceStatus(statusStr)

	if settledAtVal.Valid {
		t := settledAtVal.Time
		inv.SettledAt = &t
	}

	if err := json.Unmarshal([]byte(metaStr), &inv.Metadata); err != nil {
		return nil, fmt.Errorf("failed decoding metadata json: %w", err)
	}
	if err := json.Unmarshal([]byte(paymentsStr), &inv.Payments); err != nil {
		return nil, fmt.Errorf("failed decoding payments history json: %w", err)
	}

	return &inv, nil
}

func (s *SQLiteStore) AllocateTag(ctx context.Context, merchantAddr string) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed beginning tag allocation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	keyName := fmt.Sprintf("%s_next_tag", merchantAddr)

	var currentTag uint32 = 1000
	var tagStr string
	err = tx.QueryRowContext(ctx, "SELECT val FROM gateway_state WHERE key = ?", keyName).Scan(&tagStr)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("failed fetching current state bookmark sequence: %w", err)
		}
		// First invoice on gateway, initialize state entry
		_, err = tx.ExecContext(ctx, "INSERT INTO gateway_state (key, val) VALUES (?, '1000')", keyName)
		if err != nil {
			return 0, fmt.Errorf("failed saving initial sequence state entry: %w", err)
		}
	} else {
		var parsed uint64
		if _, err := fmt.Sscanf(tagStr, "%d", &parsed); err == nil {
			currentTag = uint32(parsed)
		}
	}

	for {
		candidate := currentTag
		currentTag++
		if currentTag > 4294967295 {
			currentTag = 1000
		}

		// Update the sequence bookmark in DB
		_, err = tx.ExecContext(ctx, "UPDATE gateway_state SET val = ? WHERE key = ?", fmt.Sprintf("%d", currentTag), keyName)
		if err != nil {
			return 0, fmt.Errorf("failed persisting updated sequence sequence: %w", err)
		}

		// Verify this candidate is not currently bound to any active invoice
		var activeExists bool
		query := `SELECT EXISTS(
			SELECT 1 FROM invoices 
			WHERE merchant_address = ? 
			  AND destination_tag = ? 
			  AND status NOT IN ('SETTLED', 'OVERPAID', 'EXPIRED', 'CANCELLED', 'REFUNDED')
		)`
		err = tx.QueryRowContext(ctx, query, merchantAddr, candidate).Scan(&activeExists)
		if err != nil {
			return 0, fmt.Errorf("failed testing candidate active status: %w", err)
		}

		if activeExists {
			continue
		}

		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("failed committing state tag sequence transaction: %w", err)
		}
		return candidate, nil
	}
}

func (s *SQLiteStore) ListPending(ctx context.Context) ([]*domain.Invoice, error) {
	query := `SELECT 
		id, order_id, merchant_address, destination_tag, amount_drops, amount_xrp,
		paid_drops, paid_xrp, status, redirect_url, webhook_url, metadata,
		tx_hash, payments, created_at, expires_at, settled_at, version
	FROM invoices 
	WHERE status NOT IN ('SETTLED', 'OVERPAID', 'EXPIRED', 'CANCELLED', 'REFUNDED')`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed executing pending listing query: %w", err)
	}
	defer rows.Close()

	var pending []*domain.Invoice
	for rows.Next() {
		var inv domain.Invoice
		var amountDropsVal, paidDropsVal int64
		var statusStr, metaStr, paymentsStr string
		var settledAtVal sql.NullTime

		err := rows.Scan(
			&inv.ID, &inv.OrderID, &inv.MerchantAddress, &inv.DestinationTag, &amountDropsVal, &inv.AmountXRP,
			&paidDropsVal, &inv.PaidXRP, &statusStr, &inv.RedirectURL, &inv.WebhookURL, &metaStr,
			&inv.TxHash, &paymentsStr, &inv.CreatedAt, &inv.ExpiresAt, &settledAtVal, &inv.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning pending listing rows: %w", err)
		}

		inv.AmountDrops = domain.Drops(amountDropsVal)
		inv.PaidDrops = domain.Drops(paidDropsVal)
		inv.Status = domain.InvoiceStatus(statusStr)

		if settledAtVal.Valid {
			t := settledAtVal.Time
			inv.SettledAt = &t
		}

		if err := json.Unmarshal([]byte(metaStr), &inv.Metadata); err != nil {
			return nil, fmt.Errorf("failed decoding row metadata JSON: %w", err)
		}
		if err := json.Unmarshal([]byte(paymentsStr), &inv.Payments); err != nil {
			return nil, fmt.Errorf("failed decoding row payments JSON: %w", err)
		}

		pending = append(pending, &inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed scanning results cleanly: %w", err)
	}

	return pending, nil
}

// Persistent state handlers for Poller ledger index tracking (from ADR-001)
func (s *SQLiteStore) GetLastProcessedLedger(ctx context.Context) (int64, error) {
	var valStr string
	err := s.db.QueryRowContext(ctx, "SELECT val FROM gateway_state WHERE key = 'last_processed_ledger'").Scan(&valStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, nil // Not found
		}
		return -1, fmt.Errorf("failed querying last processed ledger bookmark: %w", err)
	}

	var val int64
	if _, err := fmt.Sscanf(valStr, "%d", &val); err != nil {
		return -1, fmt.Errorf("malformed ledger index bookmark storage: %w", err)
	}
	return val, nil
}

func (s *SQLiteStore) SetLastProcessedLedger(ctx context.Context, index int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gateway_state (key, val) VALUES ('last_processed_ledger', ?)
		ON CONFLICT(key) DO UPDATE SET val = excluded.val
	`, fmt.Sprintf("%d", index))
	if err != nil {
		return fmt.Errorf("failed setting last processed ledger bookmark: %w", err)
	}
	return nil
}
