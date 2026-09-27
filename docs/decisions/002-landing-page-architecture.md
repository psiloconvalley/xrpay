# ADR-002: Embedded High-Performance Landing Page and Interactive Settlement Sandbox

## Status
Accepted

## Date
2026-09-26

## Context
`xrpay` is deployed to production at `https://xrpay.gopherit.dev`. Visiting the root URL (`/`) currently returns an unstyled 404. We need a landing page and live interactive settlement sandbox that:
1. Speaks directly to Senior Software Engineers evaluating non-custodial payment rails.
2. Demonstrates real 3.5-second XRP Ledger L1 finality on screen.
3. Adheres strictly to the Engineering Charter (Zero third-party runtime dependencies, zero external CDNs, sub-15ms page load, distroless-compatible).
4. Defends against tag exhaustion, faucet abuse, and denial-of-service via strict in-memory rate limiting and Content Security Policies.

## Decision
1. **Pure Go Embedded Template:** Embed semantic HTML, scoped CSS, and vanilla JS directly in the Go binary using `html/template`. No Webpack, npm, or external asset pipelines.
2. **In-Memory Rate Limiter:** Implement a thread-safe token bucket rate limiter in `internal/api/ratelimit.go` keyed by client IP. Public invoice demo generation is capped at 10 requests per hour per IP.
3. **Telemetry & Live Heartbeat Endpoint:** Expose `GET /api/v1/telemetry` returning current server time, merchant address, network mode, and memory footprint, with `Cache-Control: public, max-age=3`.
4. **Security Perimeter:** Apply strict Content Security Policy (`CSP`), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and origin isolation headers on all landing/demo routes.

## Consequences
- **Positive:** Zero external dependencies; instant binary compilation; sub-15ms worldwide TTFB; verified technical credibility with senior engineers.
- **Negative:** UI changes require compiling and deploying the Go binary (which takes < 15s in Railway).
