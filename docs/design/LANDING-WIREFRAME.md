# xrpay Wireframes & Layout Specification

## Desktop & Mobile Landing Page (GET /)
+------------------------------------------------------------------------------+
| [x] xrpay * LEDGER 87,432,109 * LIVE |
+------------------------------------------------------------------------------+
| |
| [ NON-CUSTODIAL INFRASTRUCTURE ] |
| |
| Self-host XRP payments in 60 seconds. |
| |
| Direct settlement on the XRP Ledger. Zero gateway fees. |
| Zero custody risk. Built in pure Go for high-scale SaaS. |
| |
| +----------------------------+ +-------------------+ |
| | * Watch it settle (Demo) | | $ Quickstart CLI | |
| +----------------------------+ +-------------------+ |

HOW IT WORKS (ZERO-CUSTODY L1 ARCHITECTURE)
Customer ---[ Tagged Payment ]---> Merchant Cold/Hot Wallet (XRPL L1)
(xrpay Observes)
v
Merchant Backend (Signed Webhook)
* Funds settle directly into merchant-controlled wallets in 3.5s.
* DestinationTag routing eliminates 10 XRP wallet reserve fees per order.
* Cryptographic HMAC-SHA256 webhooks guarantee verifiable event delivery.
------------------------------------------------------------------------
1-LINE DOCKER DEPLOYMENT
$ docker run -d -p 8080:8080 \
-e XRPAY_MERCHANT_ADDR="rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V" \
-e XRPAY_API_KEY="your-secure-secret-key" \
ghcr.io/psiloconvalley/xrpay:latest
+------------------------------------------------------------------------------+
XRPL FINALITY: ~3.5s
+------------------------------------------------------------------------------+
text


## Interactive Sandbox Flow (/demo/invoice -> /checkout/{id})

1. User clicks "Watch it settle (Demo)".
2. System allocates an isolated Destination Tag with metadata.demo = "true".
3. Client is redirected to the dark-mode checkout screen displaying recipient classic address and tag.
4. Client sends Testnet XRP via wallet or XRP Toolkit.
5. Background poller detects transaction on-chain within consensus cycle (3.5s).
6. Screen transitions dynamically to [ SETTLED ] with transaction hash, ledger index, and audit proof.
