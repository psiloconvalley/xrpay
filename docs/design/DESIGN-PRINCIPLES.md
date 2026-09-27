# xrpay Design Principles & Front-Door Architecture

## 1. Singular Audience
**The Senior Engineer at a Tech Startup.**
- Evaluating crypto settlement infrastructure to bypass 3% processing fees, chargeback fraud, and geographic merchant account lockouts.
- Skeptical of crypto hype; demands verifiable, deterministic, non-custodial architecture.
- Values: Pure Go stdlib, CGO-free compilation, static distroless containers, exact integer math (Drops), cryptographic HMAC-SHA256 webhooks, and zero custodial liability.

## 2. Positioning & Voice
- **Headline:** "Self-host XRP payments in 60 seconds."
- **Sub-headline:** "Zero custody. Zero gateway fees. Direct L1 settlement on the XRP Ledger in 3.5 seconds."
- **Voice:** Declarative, technical, confident, unembellished. Facts, specifications, and code only.

## 3. Signature Aesthetic: "Financial Telemetry & Monochrome Precision"
- **Visual Tone:**
  - Background: Deep Obsidian Slate (`#070a11` / `#0b0f19`)
  - Surface Cards: Subdued Slate (`#111827`) with 1px border (`#1f293d`)
  - Monospace Typography: For all financial values, drops, addresses, transaction hashes, destination tags, and timestamps.
  - Accent Palette: Electric Emerald (`#10b981`) for settled state, Cyan (`#38bdf8`) for live ledger heartbeat.
- **Zero Asset Bloat:**
  - 100% embedded HTML, CSS, and vanilla JS directly in the Go binary.
  - Zero Google Fonts, zero external CDNs, zero third-party trackers.
  - Sub-15ms page load globally with a total payload weight < 35 KB.

## 4. Security & Operational Invariants
1. **Per-IP Rate Limiting:** In-memory token bucket on public endpoints (`/demo/invoice` capped at 10 req/hr per IP) to defend against destination tag exhaustion.
2. **Strict Content Security Policy (CSP):** `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none';`.
3. **Demo Isolation:** Demo invoices are tagged `metadata.demo = "true"`, auto-expire in 15 minutes, and suppress outbound webhook traffic.
4. **Live Telemetry:** Read-only cached endpoint (`/api/v1/telemetry`) exposing ledger heartbeat, merchant address, network status, and memory stats.
