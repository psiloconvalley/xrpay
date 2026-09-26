# xrpay

**Self-hosted, zero-fee, single-binary payment gateway for the XRP Ledger, written in vanilla Go.**

Accept XRP directly to your non-custodial wallet with 3-5 second settlement, no middleman, no processing fees.

---

## Status

🚧 **Alpha — under active development.** Not yet suitable for Mainnet.

---

## Why xrpay?

| Metric | Stripe | BitPay | xrpay |
|---|---|---|---|
| Fee | 2.9% + $0.30 | 1.0% | **0%** |
| Settlement | 2–7 days | 1–24 hours | **3–5 seconds** |
| Custody | Bank-held | Middleman-held | **Direct to your wallet** |
| Global | 46 countries | KYC-gated | **Universal** |

---

## Engineering Standards

This project follows a strict engineering charter. See **[ENGINEERING.md](ENGINEERING.md)** before contributing or reading the code.

Key rules:
- Zero third-party dependencies (Go stdlib only, exception: `Peersyst/xrpl-go`)
- Integer-only monetary math (never `float64` for money)
- 100% test coverage on core domain
- Structured logging via `log/slog`
- Clean architecture — dependencies point inward

---

## Quickstart

*(Coming in Phase 4.)*

---

## License

MIT
