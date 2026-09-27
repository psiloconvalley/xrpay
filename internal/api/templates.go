package api

const landingHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>xrpay • Self-Host XRP Payments</title>
    <style>
        :root {
            --bg-primary: #070a11;
            --bg-surface: #111827;
            --border-muted: #1f293d;
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --text-muted: #4b5563;
            --accent-green: #10b981;
            --accent-cyan: #38bdf8;
            --font-mono: "Courier New", Courier, monospace;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            line-height: 1.5;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            padding: 2rem 1.5rem;
            width: 100%;
        }
        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid var(--border-muted);
            padding-bottom: 1.5rem;
            margin-bottom: 3rem;
        }
        .logo {
            font-size: 1.25rem;
            font-weight: 700;
            letter-spacing: -0.05em;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .heartbeat {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            font-size: 0.8rem;
            color: var(--accent-cyan);
            font-family: var(--font-mono);
        }
        .pulse {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: var(--accent-cyan);
            box-shadow: 0 0 8px var(--accent-cyan);
            animation: blink 2s infinite;
        }
        @keyframes blink {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.3; }
        }
        .hero {
            margin-bottom: 4rem;
        }
        .tagline {
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 0.15em;
            color: var(--text-secondary);
            margin-bottom: 0.5rem;
            font-family: var(--font-mono);
        }
        h1 {
            font-size: 2.75rem;
            font-weight: 800;
            line-height: 1.1;
            letter-spacing: -0.03em;
            margin-bottom: 1rem;
        }
        .subhead {
            font-size: 1.15rem;
            color: var(--text-secondary);
            max-width: 600px;
            margin-bottom: 2rem;
        }
        .btn-group {
            display: flex;
            gap: 1rem;
            flex-wrap: wrap;
        }
        .btn {
            background: var(--text-primary);
            color: var(--bg-primary);
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 6px;
            font-weight: 600;
            text-decoration: none;
            cursor: pointer;
            transition: opacity 0.2s;
            font-size: 0.95rem;
        }
        .btn:hover { opacity: 0.9; }
        .btn-secondary {
            background: transparent;
            color: var(--text-primary);
            border: 1px solid var(--border-muted);
        }
        .btn-secondary:hover { background: rgba(255, 255, 255, 0.05); }
        .sandbox-card {
            background: var(--bg-surface);
            border: 1px solid var(--border-muted);
            border-radius: 8px;
            padding: 2rem;
            margin-bottom: 4rem;
        }
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
            border-bottom: 1px solid var(--border-muted);
            padding-bottom: 0.75rem;
        }
        .card-title {
            font-family: var(--font-mono);
            font-weight: 700;
            font-size: 0.95rem;
        }
        .form-group {
            margin-bottom: 1.25rem;
        }
        label {
            display: block;
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-secondary);
            margin-bottom: 0.4rem;
            font-weight: 600;
        }
        input {
            width: 100%;
            background: #030712;
            border: 1px solid var(--border-muted);
            padding: 0.75rem 1rem;
            border-radius: 6px;
            color: #fff;
            font-size: 1rem;
            outline: none;
        }
        input:focus { border-color: var(--accent-cyan); }
        .feature-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
            margin-bottom: 4rem;
        }
        .feature {
            background: var(--bg-surface);
            border: 1px solid var(--border-muted);
            border-radius: 8px;
            padding: 1.5rem;
        }
        .feat-title {
            font-size: 1rem;
            font-weight: 700;
            margin-bottom: 0.5rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .feat-desc { font-size: 0.88rem; color: var(--text-secondary); }
        .code-box {
            background: #030712;
            border: 1px solid var(--border-muted);
            border-radius: 8px;
            padding: 1.5rem;
            font-family: var(--font-mono);
            font-size: 0.85rem;
            color: #e2e8f0;
            overflow-x: auto;
            margin-bottom: 4rem;
        }
        footer {
            border-top: 1px solid var(--border-muted);
            padding-top: 2rem;
            text-align: center;
            font-size: 0.8rem;
            color: var(--text-muted);
        }
        footer a { color: var(--text-secondary); text-decoration: none; margin: 0 0.5rem; }
        footer a:hover { color: #fff; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div class="logo">⏣ xrpay</div>
            <div class="heartbeat">
                <span class="pulse"></span>
                <span>XRPL TESTNET · LIVE</span>
            </div>
        </header>

        <section class="hero">
            <div class="tagline">Non-Custodial L1 Settle Engine</div>
            <h1>Self-host XRP payments<br>in 60 seconds.</h1>
            <p class="subhead">Direct on-chain settlements. Zero gateway fees. Zero custody risks. Hardened Go engine built for startups requiring sovereign payment infrastructure.</p>
            <div class="btn-group">
                <a href="#demo" class="btn">Watch It Settle (Demo)</a>
                <a href="https://github.com/psiloconvalley/xrpay" target="_blank" class="btn btn-secondary">Read GitHub Docs</a>
            </div>
        </section>

        <section id="demo" class="sandbox-card">
            <div class="card-header">
                <div class="card-title">⏣ INTERACTIVE LIVE SANDBOX</div>
                <span class="heartbeat">1.00 XRP MINIMUM</span>
            </div>
            <form action="/demo/invoice" method="POST">
                <div class="form-group">
                    <label for="amount">Sandbox Amount (XRP)</label>
                    <input type="text" id="amount" name="amount" value="5.00" required>
                </div>
                <div class="form-group">
                    <label for="memo">Memo / Purpose</label>
                    <input type="text" id="memo" name="memo" value="Standard Plan Subscription" required>
                </div>
                <button type="submit" class="btn" style="width: 100%;">Create Sandbox Invoice &rarr;</button>
            </form>
        </section>

        <section class="feature-grid">
            <div class="feature">
                <div class="feat-title"><span style="color:var(--accent-green)">✓</span> Non-Custodial</div>
                <div class="feat-desc">Payments settle instantly into your master wallet on-chain. xrpay has no access to private keys.</div>
            </div>
            <div class="feature">
                <div class="feat-title"><span style="color:var(--accent-cyan)">⚡</span> 3.5s Finality</div>
                <div class="feat-desc">Powered by native XRPL L1 consensus. Fully cleared and settled payments in under four seconds.</div>
            </div>
            <div class="feature">
                <div class="feat-title"><span style="color:var(--accent-cyan)">⏣</span> Destination Tag Routing</div>
                <div class="feat-desc">Route infinite customers through a single master wallet with tag isolation. Zero reserve locks.</div>
            </div>
        </section>

        <section class="code-box">
# Deploy using a single command in Docker
$ docker run -d -p 8080:8080 \
  -e XRPAY_MERCHANT_ADDR="{{.MerchantAddress}}" \
  -e XRPAY_API_KEY="your-secure-api-key" \
  ghcr.io/psiloconvalley/xrpay:latest
        </section>

        <footer>
            <p>xrpay &bull; Open-Source Financial Software Infrastructure</p>
            <p style="margin-top: 0.5rem;">
                <a href="https://github.com/psiloconvalley/xrpay" target="_blank">Repository</a> |
                <a href="/api/v1/telemetry">Telemetry Check</a> |
                <a href="/healthz">Healthz</a>
            </p>
        </footer>
    </div>
</body>
</html>`

const checkoutHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>xrpay • Settle Invoice</title>
    <style>
        :root {
            --bg-color: #070a11;
            --card-bg: #111827;
            --text-color: #f8fafc;
            --text-muted: #94a3b8;
            --border-color: #1f293d;
            --accent-emerald: #10b981;
            --accent-amber: #f59e0b;
            --font-mono: "Courier New", Courier, monospace;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 1rem;
        }
        .card {
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            max-width: 440px;
            width: 100%;
            padding: 2rem;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5);
        }
        .header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 1.5rem;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 0.75rem;
        }
        .logo { font-size: 1.15rem; font-weight: 700; letter-spacing: -0.05em; }
        .badge {
            font-size: 0.75rem;
            font-weight: 600;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            text-transform: uppercase;
            font-family: var(--font-mono);
        }
        .badge-pending { background: rgba(245, 158, 11, 0.1); color: var(--accent-amber); border: 1px solid var(--accent-amber); }
        .badge-settled { background: rgba(16, 185, 129, 0.1); color: var(--accent-emerald); border: 1px solid var(--accent-emerald); }
        .amount-box {
            text-align: center;
            margin-bottom: 1.5rem;
            background: #030712;
            padding: 1.25rem;
            border-radius: 8px;
            border: 1px solid var(--border-color);
        }
        .amount-xrp { font-size: 2rem; font-weight: 800; color: var(--accent-emerald); }
        .field { margin-bottom: 1.25rem; }
        .field-label {
            font-size: 0.8rem;
            color: var(--text-muted);
            margin-bottom: 0.4rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            font-weight: 600;
        }
        .value-box {
            display: flex;
            align-items: center;
            background: #030712;
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 0.75rem;
        }
        .value-text {
            font-family: var(--font-mono);
            font-size: 0.88rem;
            flex-grow: 1;
            word-break: break-all;
        }
        .highlight { border-color: var(--accent-amber); }
        .tag-text { font-size: 1.15rem; font-weight: 700; color: var(--accent-amber); }
        .btn-copy {
            background: transparent;
            border: 1px solid var(--border-color);
            color: var(--text-muted);
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.75rem;
            margin-left: 0.5rem;
        }
        .btn-copy:hover { color: #fff; border-color: var(--text-muted); }
        .warning {
            background: rgba(245, 158, 11, 0.05);
            border-left: 3px solid var(--accent-amber);
            padding: 0.75rem 1rem;
            font-size: 0.8rem;
            color: #fbbf24;
            margin-bottom: 1.5rem;
            border-radius: 0 4px 4px 0;
            line-height: 1.4;
        }
        .qr-section {
            display: flex;
            justify-content: center;
            margin-bottom: 1.5rem;
        }
        .qr-placeholder {
            background: #fff;
            padding: 10px;
            border-radius: 8px;
            width: 140px;
            height: 140px;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .status-footer {
            text-align: center;
            padding-top: 1rem;
            border-top: 1px solid var(--border-color);
            font-size: 0.85rem;
            color: var(--text-muted);
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <div class="logo">⏣ xrpay</div>
            <span id="badge" class="badge badge-pending">{{.Invoice.Status}}</span>
        </div>

        <div class="amount-box">
            <div class="amount-xrp">{{.Invoice.AmountXRP}} XRP</div>
            {{if .Memo}}<div style="font-size:0.85rem;color:var(--text-muted);margin-top:0.4rem;">{{.Memo}}</div>{{end}}
        </div>

        <div class="qr-section">
            <div class="qr-placeholder">
                <img src="https://api.qrserver.com/v1/create-qr-code/?size=120x120&data={{.QRPayload}}" alt="QR Code" width="120" height="120">
            </div>
        </div>

        <div class="field">
            <div class="field-label">Recipient Classic Address</div>
            <div class="value-box">
                <span class="value-text" id="addr">{{.Invoice.MerchantAddress}}</span>
                <button class="btn-copy" onclick="copy('addr')">Copy</button>
            </div>
        </div>

        <div class="field">
            <div class="field-label">Destination Tag</div>
            <div class="value-box highlight">
                <span class="value-text tag-text" id="tag">{{.FormattedTag}}</span>
                <button class="btn-copy" onclick="copy('tag')">Copy</button>
            </div>
        </div>

        <div class="warning">
            <strong>CRITICAL:</strong> You must enter Destination Tag <strong>{{.FormattedTag}}</strong> precisely. If omitted, your funds cannot be matched to this checkout.
        </div>

        <div class="status-footer" id="status-box">
            <span>● Checking XRP Ledger...</span>
        </div>
    </div>

    <script>
        function copy(id) {
            const text = document.getElementById(id).innerText;
            navigator.clipboard.writeText(text).then(() => {
                alert("Copied: " + text);
            });
        }

        const invoiceId = "{{.Invoice.ID}}";
        const redirectUrl = "{{.Invoice.RedirectURL}}";

        function poll() {
            fetch("/api/v1/invoices/" + invoiceId + "/status")
                .then(r => r.json())
                .then(data => {
                    if (data.settled) {
                        document.getElementById("badge").className = "badge badge-settled";
                        document.getElementById("badge").innerText = "SETTLED";
                        document.getElementById("status-box").innerHTML = "✅ <strong>Payment Received! Thank you.</strong>";
                        if (redirectUrl) {
                            setTimeout(() => { window.location.href = redirectUrl; }, 1500);
                        }
                    } else if (data.expired) {
                        document.getElementById("status-box").innerText = "❌ Invoice Expired";
                    } else {
                        setTimeout(poll, 2000);
                    }
                })
                .catch(() => setTimeout(poll, 4000));
        }
        poll();
    </script>
</body>
</html>`
