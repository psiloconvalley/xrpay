# ADR-001: Storage Engine and Persistence Strategy

## Status
Accepted

## Date
2026-09-26

## Context
`xrpay` is positioned as the "BTCPay Server for XRP"—a single, self-contained binary that merchants can deploy on a $5/mo VPS, bare metal server, or container host (e.g., Railway, Fly.io, Docker) with zero operational complexity.

In the initial implementation, `xrpay` utilized an in-memory store (`MemoryStore`). While thread-safe and performant for development, container restarts or deployments lead to:
1. Loss of pending, settled, and expired invoice records.
2. Loss of the poller's last processed ledger cursor bookmark (`last_processed_ledger`), requiring full historical scans upon reboot.
3. Reset of the monotonic `destination_tag` sequence counter.

We need a persistent storage architecture that provides:
- ACID transactional guarantees.
- Monotonically increasing Destination Tag reservation with no race conditions.
- Poller cursor bookmarking (`last_processed_ledger`) to eliminate RPC rescan overhead on restarts.
- Zero external infrastructure requirement (no mandatory external database container for self-hosters).
- Compatibility with our Distroless non-root container deployment.

## Decision Drivers
1. **Single-Binary Operational Simplicity:** Self-hosters should not be forced to deploy, manage, and back up a separate database server.
2. **CGO-Free Compilation:** Pure Go compilation is required to keep cross-compilation simple and maintain a lightweight Distroless static container footprint (~15MB).
3. **Data Integrity & Concurrency:** Robust concurrent read performance with atomic writes (WAL mode).
4. **Interface Isolation:** Storage must strictly satisfy the domain-agnostic `store.InvoiceStore` interface defined in `internal/store/store.go`.

## Considered Options

### Option 1: Pure Go SQLite (`modernc.org/sqlite` via `database/sql`)
- **Pros:**
  - Embedded, serverless, zero-config single file (`xrpay.db`).
  - Pure Go (C-to-Go transpiled via CCGO), requiring **zero CGO** (`CGO_ENABLED=0` remains functional).
  - Full ACID transactions, Write-Ahead Logging (`PRAGMA journal_mode=WAL`), and robust concurrency for thousands of requests per second.
  - Standard Go `database/sql` driver.
- **Cons:**
  - Adds one external Go library dependency (`modernc.org/sqlite`).

### Option 2: CGO SQLite (`github.com/mattn/go-sqlite3`)
- **Pros:**
  - Industry-standard C SQLite wrapper.
- **Cons:**
  - Requires CGO, C toolchain (`gcc`/`musl`), complicates cross-compilation, and breaks static distroless binary compilation.

### Option 3: External PostgreSQL (`database/sql` via `pgx` or `lib/pq`)
- **Pros:**
  - Multi-node scalability and clustered high availability.
- **Cons:**
  - Destroys single-binary operational simplicity for small merchants and $5/mo VPS deployments.
  - Requires managing a secondary database service.

### Option 4: Pure Stdlib JSON / Append-Only Log File
- **Pros:**
  - Zero external dependencies (strictly stdlib).
- **Cons:**
  - High complexity to implement crash recovery, atomic index lookups by Destination Tag, pagination, and multi-threaded WAL compaction safely.

## Decision
We approve and adopt **Option 1: Pure Go SQLite (`modernc.org/sqlite`)** as the primary default embedded storage engine for `xrpay`.

To maintain clean architecture:
1. The storage implementation will live in `internal/store/sqlite.go` implementing `store.InvoiceStore`.
2. Standard `database/sql` interfaces will be used exclusively.
3. If no custom database path is specified via `XRPAY_DB_PATH`, it defaults to `xrpay.db` (or in-memory `:memory:` for automated tests).
4. The schema will store:
   - `invoices`: Full invoice state, amounts in integer drops, metadata, and settled timestamps.
   - `invoice_payments`: Append-only audit log of individual on-chain payment hashes and delivered drops.
   - `gateway_state`: Key-value storage for persistent poller bookmarks (`last_processed_ledger`) and destination tag sequence state.

## Consequences
- **Positive:**
  - Invoices, ledger index bookmarks, and destination tag sequences persist seamlessly across container redeployments and restarts.
  - Retains single-binary, static distroless container deployment with `CGO_ENABLED=0`.
  - Zero required database configuration for merchants.
- **Negative:**
  - Introduces `modernc.org/sqlite` to `go.mod`.
