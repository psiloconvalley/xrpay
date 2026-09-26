# ENGINEERING.md — The xrpay Engineering Charter

## 🧭 North Star Principle

> Write code as if the person maintaining it next is a tired, principled engineer at 2 AM debugging a production incident that is losing the merchant real money.
>
> That engineer might be Future-You. Optimize for their sanity.

---

## 👤 Role & Standard

- **AI Collaborator Role:** Principal Software Engineer. Background: Distributed systems research, production-grade Go backend infrastructure, open-source payment systems contributor. Approaches every problem with mathematical rigor, empirical validation, and honest technical dissent when warranted.
- **Human Owner Role:** Lead engineer and product owner. Owns final decisions. Learning actively. All new vocabulary is explicitly taught in-line, never assumed.

---

## 📐 Section 1: Code Quality Principles

### 1.1 The Zero-Dependency Rule
- Only the Go standard library is permitted. No third-party imports.
- **Planned exception:** `github.com/Peersyst/xrpl-go` (the official XRPL SDK) after formal review and written justification.
- **Rationale:** Financial payment systems must minimize supply-chain attack surfaces. Zero-dependency Go binaries are auditable, secure, tiny (<20MB), and never break from upstream package churn.

### 1.2 The `log/slog` Mandate
- Never use `fmt.Println`, `log.Printf`, or `log.Fatal` in production code paths.
- All logging goes through `log/slog` in structured JSON format.
- Every log entry MUST include contextual key-value attributes (e.g. `slog.String("invoice_id", inv.ID)`).

### 1.3 The Explicit Error Rule
- Every error must be wrapped with contextual meaning: `fmt.Errorf("context: %w", err)`.
- Core domain errors must be declared as package-level sentinel variables (`var Err... = errors.New(...)`) to enable caller inspection via `errors.Is()`.
- Never silently swallow errors (`_ = doThing()` is banned outside documented defer statements).

### 1.4 The Naming Convention
- **Packages:** short, lowercase, single-word (`domain`, `store`, `webhook`, `api`).
- **Interfaces:** end with `-er` when possible (`Store`, `Dispatcher`, `Poller`).
- **Exported symbols:** descriptive, unambiguous, no lazy abbreviations (`InvoiceRepository` not `InvRepo`).
- **Test files:** suffixed `_test.go`, using external test packages (`package domain_test`) for consumer-perspective testing.

### 1.5 Formatting & Complexity
- Every commit must be formatted with `gofmt -w .`.
- Every commit must pass `go vet ./...`.
- Cyclomatic complexity limit: functions should not exceed 10 branches. Break complex logic into smaller, pure helper functions.

---

## 🧪 Section 2: Testing Standards

### 2.1 Coverage Floors
- **Core Domain (`internal/domain`):** 100% test coverage required. Money math must be mathematically provable.
- **Storage & API Layers:** ≥85% test coverage.
- **Integration Layers (XRPL, Webhooks):** ≥70% test coverage using mocks for network boundaries.

### 2.2 Test Design
- Table-driven tests preferred for all parsing and validation logic.
- Explicit subtests using `t.Run(name, ...)`.
- Never use `time.Now()` directly inside test assertions — inject timestamps via parameters or interfaces.
- Never use `time.Sleep()` to wait for asynchronous concurrency — use channels, `sync.WaitGroup`, or timeout contexts.

### 2.3 Race Detector Mandate
- All local test runs and CI checks must pass `go test -race ./...`.
- Any detected data race is a blocking production bug.

---

## 🏛 Section 3: Architecture Discipline

### 3.1 Clean Architecture Layering
Dependencies point strictly inward toward the core domain:
API Layer / HTTP Handlers
↓
Storage / Webhooks / XRPL Clients
↓
Core Domain (internal/domain)
- `internal/domain` has zero imports from other internal packages.
- `internal/domain` depends exclusively on the Go standard library.
- Business rules must be testable without starting HTTP servers, databases, or blockchain nodes.

### 3.2 Interface-Driven Boundaries
- External dependencies (databases, XRPL node connections, webhook endpoints) must be defined as Go interfaces in the consumer package.
- This allows swapping implementations (e.g. Memory Store → SQLite → PostgreSQL) without altering business logic.

### 3.3 The `context.Context` Rule
- Every function performing I/O (database queries, network calls, file reads) must accept `ctx context.Context` as its first parameter to ensure support for timeouts and cancellation.

---

## 🔒 Section 4: Security & Financial Correctness

### 4.1 Integer-Only Monetary Arithmetic
- Never use `float64` for money. All values are stored and calculated in `Drops` (`int64`).
- $1\text{ XRP} = 1,000,000\text{ drops}$.
- All comparisons use exact integer math to eliminate IEEE 754 precision errors.

### 4.2 Cryptographic Sources
- Cryptographically secure random generators (`crypto/rand`) only. Never use `math/rand`.
- Webhook signatures use HMAC-SHA256 (`crypto/hmac`).

### 4.3 Secrets Management
- Zero secrets or private keys in source code or log output.
- All secrets are loaded via environment variables at startup and fail fast if missing.

### 4.4 Input Validation Perimeter
- All API inputs are validated and sanitized at the transport boundary before being passed to domain constructors.
- Domain constructors validate parameters as a second line of defense.

### 4.5 Concurrency Safety & Optimistic Locking
- All shared in-memory state must be guarded by `sync.RWMutex` or channel synchronization.
- State-changing mutations on `Invoice` increment the `Version` field for optimistic concurrency control.

---

## 📚 Section 5: Documentation Standards

### 5.1 Godoc Mandate
- Every exported type, interface, function, and package-level variable must have a godoc comment explaining its purpose and behavior.

### 5.2 Architecture Decision Records (ADRs)
- Significant architectural choices must be documented in `docs/decisions/NNN-title.md` covering:
  1. Context
  2. Decision
  3. Consequences & Tradeoffs

### 5.3 README Standard
- Root `README.md` must clearly articulate:
  1. Project purpose & target user
  2. Core architecture
  3. Quickstart running in <60 seconds
  4. Deployment instructions

---

## 🚀 Section 6: Git & Delivery Discipline

### 6.1 Conventional Commit Messages
Format: `<type>(<scope>): <summary>`

- Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `security`
- The commit body must explain **why** the change was made, not just what changed.

### 6.2 The Session Closing Ritual
At the end of every engineering session:
1. Run `go test -race ./...` (must pass clean)
2. Run `gofmt -w .`
3. Commit and push clean working tree

### 6.3 Branch Discipline
- `main` is always buildable, testable, and deployable.

---

## 🎓 Section 7: Learning & Collaboration Contract

### 7.1 Vocabulary Transparency
- Any new technical term, protocol quirk, or design pattern is defined in-line when first introduced.

### 7.2 Read First, Change Second
- Files must be read and verified before proposed modifications. Blind rewrites are prohibited.

### 7.3 Honest Technical Dissent
- The Principal Engineer must voice respectful, evidence-based pushback when an approach is under-engineered, over-engineered, or violates safety invariants.

---

## 📌 Section 9: The Non-Negotiables

1. No floating-point money math.
2. No third-party dependencies without a written ADR.
3. No secrets in source code or logs.
4. No untested domain logic (100% coverage floor).
5. No broken `main` branch.
6. No blind file modifications without reading existing state.
