package api

const landingHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>xrpay • 0% Fee Non-Custodial XRP Payment Gateway</title>
    <style>
        :root {
            --bg-primary: #030712;
            --bg-surface: #0b0f19;
            --bg-card: #111827;
            --border-muted: #1f293d;
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --text-muted: #4b5563;
            --accent-emerald: #10b981;
            --accent-cyan: #38bdf8;
            --accent-amber: #f59e0b;
            --accent-glow: rgba(56, 189, 248, 0.15);
            --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            background-color: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            line-height: 1.6;
        }
        .container {
            max-width: 960px;
            margin: 0 auto;
            padding: 2.5rem 1.5rem;
            width: 100%;
        }
        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid var(--border-muted);
            padding-bottom: 1.5rem;
            margin-bottom: 4rem;
        }
        .logo {
            font-size: 1.35rem;
            font-weight: 800;
            letter-spacing: -0.05em;
            display: flex;
            align-items: center;
            gap: 0.5rem;
            color: var(--text-primary);
            text-decoration: none;
        }
        .logo span { color: var(--accent-cyan); }
        .heartbeat {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            font-size: 0.75rem;
            color: var(--accent-emerald);
            font-family: var(--font-mono);
            font-weight: 600;
            background: rgba(16, 185, 129, 0.08);
            padding: 0.35rem 0.75rem;
            border-radius: 9999px;
            border: 1px solid rgba(16, 185, 129, 0.2);
        }
        .pulse {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: var(--accent-emerald);
            box-shadow: 0 0 8px var(--accent-emerald);
            animation: blink 2s infinite;
        }
        @keyframes blink {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.3; }
        }
        .hero {
            margin-bottom: 4.5rem;
            text-align: center;
        }
        .tagline {
            display: inline-block;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.2em;
            color: var(--accent-cyan);
            margin-bottom: 1.25rem;
            font-family: var(--font-mono);
            font-weight: 700;
            background: var(--accent-glow);
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            border: 1px solid rgba(56, 189, 248, 0.2);
        }
        h1 {
            font-size: 3.25rem;
            font-weight: 900;
            line-height: 1.15;
            letter-spacing: -0.04em;
            margin-bottom: 1.5rem;
            color: #fff;
        }
        h1 span {
            background: linear-gradient(135deg, #fff 30%, var(--accent-cyan) 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .subhead {
            font-size: 1.2rem;
            color: var(--text-secondary);
            max-width: 720px;
            margin: 0 auto 2.5rem auto;
            font-weight: 400;
        }
        .btn-group {
            display: flex;
            gap: 1rem;
            justify-content: center;
            flex-wrap: wrap;
        }
        .btn {
            background: var(--text-primary);
            color: var(--bg-primary);
            border: none;
            padding: 0.85rem 1.75rem;
            border-radius: 6px;
            font-weight: 700;
            text-decoration: none;
            cursor: pointer;
            transition: all 0.15s ease-in-out;
            font-size: 1rem;
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
        }
        .btn:hover {
            transform: translateY(-1px);
            opacity: 0.95;
            box-shadow: 0 4px 12px rgba(255, 255, 255, 0.1);
        }
        .btn-secondary {
            background: transparent;
            color: var(--text-primary);
            border: 1px solid var(--border-muted);
        }
        .btn-secondary:hover {
            background: rgba(255, 255, 255, 0.03);
            border-color: var(--text-secondary);
            box-shadow: none;
        }
        .feature-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 1.5rem;
            margin-bottom: 4.5rem;
        }
        .feature {
            background: var(--bg-surface);
            border: 1px solid var(--border-muted);
            border-radius: 12px;
            padding: 1.75rem;
            transition: border-color 0.2s ease;
        }
        .feature:hover { border-color: rgba(56, 189, 248, 0.3); }
        .feat-title {
            font-size: 1.1rem;
            font-weight: 700;
            margin-bottom: 0.75rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
            color: #fff;
        }
        .feat-desc { font-size: 0.9rem; color: var(--text-secondary); line-height: 1.5; }
        .sandbox-card {
            background: linear-gradient(145deg, var(--bg-surface) 0%, #0d1321 100%);
            border: 1px solid var(--border-muted);
            border-radius: 12px;
            padding: 2.5rem;
            margin-bottom: 4.5rem;
            position: relative;
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.25);
        }
        .sandbox-card::before {
            content: '';
            position: absolute;
            top: 0; left: 0; right: 0; height: 3px;
            background: linear-gradient(90deg, var(--accent-cyan), var(--accent-emerald));
            border-radius: 12px 12px 0 0;
        }
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
            border-bottom: 1px solid var(--border-muted);
            padding-bottom: 1rem;
        }
        .card-title {
            font-family: var(--font-mono);
            font-weight: 700;
            font-size: 0.9rem;
            color: #fff;
            letter-spacing: 0.05em;
        }
        .form-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 1.25rem;
            margin-bottom: 1.25rem;
        }
        @media (max-width: 640px) {
            .form-grid { grid-template-columns: 1fr; }
        }
        .form-group { margin-bottom: 1.25rem; }
        label {
            display: block;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-secondary);
            margin-bottom: 0.5rem;
            font-weight: 700;
        }
        input, select {
            width: 100%;
            background: #020617;
            border: 1px solid var(--border-muted);
            padding: 0.85rem 1.15rem;
            border-radius: 6px;
            color: #fff;
            font-size: 1rem;
            outline: none;
            transition: all 0.15s ease;
            font-family: inherit;
        }
        input:focus, select:focus {
            border-color: var(--accent-cyan);
            box-shadow: 0 0 0 3px var(--accent-glow);
        }
        .table-container {
            background: var(--bg-surface);
            border: 1px solid var(--border-muted);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 4.5rem;
            overflow-x: auto;
        }
        .table-title {
            font-family: var(--font-mono);
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.1em;
            color: var(--accent-cyan);
            margin-bottom: 1rem;
            font-weight: 700;
        }
        table { width: 100%; border-collapse: collapse; text-align: left; font-size: 0.9rem; }
        th, td { padding: 0.75rem 1rem; border-bottom: 1px solid var(--border-muted); }
        th { color: var(--text-secondary); font-weight: 600; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.05em; }
        tr:last-child td { border-bottom: none; }
        .text-green { color: var(--accent-emerald); font-weight: 600; }
        .text-red { color: #f43f5e; }
        .code-box {
            background: #020617;
            border: 1px solid var(--border-muted);
            border-radius: 12px;
            padding: 1.75rem;
            font-family: var(--font-mono);
            font-size: 0.85rem;
            color: #e2e8f0;
            overflow-x: auto;
            margin-bottom: 4.5rem;
            line-height: 1.5;
        }
        .code-header {
            color: var(--text-muted);
            margin-bottom: 0.75rem;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.15em;
        }
        footer {
            border-top: 1px solid var(--border-muted);
            padding-top: 2.5rem;
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
            <a href="/" class="logo">⏣ xr<span>pay</span></a>
            <div class="heartbeat">
                <span class="pulse"></span>
                <span>XRPL TESTNET · ACTIVE</span>
            </div>
        </header>

        <section class="hero">
            <div class="tagline">SOVEREIGN WEALTH GATEWAY</div>
            <h1>Accept XRP Payments With<br><span>0% Fees & No Third-Party.</span></h1>
            <p class="subhead">The self-hosted, non-custodial L1 payment gateway in pure Go. Your keys, your consensus settlement, zero intermediary holds, zero regulatory overhead.</p>
            <div class="btn-group">
                <a href="#checkout-tool" class="btn">Launch Point-of-Sale / Tip &rarr;</a>
                <a href="https://github.com/psiloconvalley/xrpay" target="_blank" class="btn btn-secondary">Inspect Source Code</a>
            </div>
        </section>

        <section class="feature-grid">
            <div class="feature">
                <div class="feat-title"><span style="color:var(--accent-emerald)">✓</span> Sovereign & Non-Custodial</div>
                <div class="feat-desc">Payments settle instantly into your own ledger keys. xrpay never holds, screens, or routes your merchant balance.</div>
            </div>
            <div class="feature">
                <div class="feat-title"><span style="color:var(--accent-cyan)">⚡</span> 3.5s Settlement Finality</div>
                <div class="feat-desc">Powered by native Ripple consensus. Transactions are guaranteed mathematically settled and irreversible in under four seconds.</div>
            </div>
            <div class="feature">
                <div class="feat-title"><span style="color:var(--accent-cyan)">⏣</span> Zero Reserve Bloat</div>
                <div class="feat-desc">Assign infinite customers to a single master classic wallet using per-invoice Destination Tags. No 10-XRP wallet reserves required.</div>
            </div>
        </section>

        <!-- Dynamic POS Generator / Donation & Tipping Hub -->
        <section id="checkout-tool" class="sandbox-card">
            <div class="card-header">
                <div class="card-title">⏣ INSTANT POINT-OF-SALE &amp; DONATION CHECKOUT</div>
                <span class="heartbeat" style="color:var(--accent-cyan); background:rgba(56,189,248,0.08); border-color:rgba(56,189,248,0.2);">0% PLATFORM FEE</span>
            </div>
            <form action="/demo/invoice" method="POST">
                <div class="form-grid">
                    <div class="form-group">
                        <label for="currency">Settlement Currency</label>
                        <select id="currency" name="currency">
                            <option value="XRP">XRP (Native L1 • 3.5s Settlement)</option>
                            <option value="RLUSD" disabled>RLUSD (Ripple USD • Coming with Mainnet)</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="amount">Custom Amount (Enter Any XRP Value)</label>
                        <input type="text" id="amount" name="amount" value="5.00" placeholder="e.g. 2.50, 10, 100" required>
                    </div>
                </div>
                <div class="form-group">
                    <label for="memo">Order Description / Tip Message (Optional)</label>
                    <input type="text" id="memo" name="memo" value="Support Sovereign Open-Source xrpay" placeholder="e.g. Donation from @twitterhandle, Coffee tip, or API invoice">
                </div>
                <button type="submit" class="btn" style="width: 100%; background: var(--accent-cyan); color: var(--bg-primary); font-weight: 800;">Generate Point-of-Sale Checkout &rarr;</button>
            </form>
        </section>

        <section class="table-container">
            <div class="table-title">Comparing Merchant Settlement Infrastructure</div>
            <table>
                <thead>
                    <tr>
                        <th>Gateway Provider</th>
                        <th>Transaction Fee</th>
                        <th>Settlement Delay</th>
                        <th>Platform Risk (Holds/Freezes)</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td>Traditional (Stripe/PayPal)</td>
                        <td class="text-red">2.9% + $0.30</td>
                        <td>2 - 7 Business Days</td>
                        <td class="text-red">High (Reserve balances, chargebacks)</td>
                    </tr>
                    <tr>
                        <td>BitPay (Custodial Crypto)</td>
                        <td class="text-red">1.0% - 2.0%</td>
                        <td>24 Hours (Batch payout)</td>
                        <td class="text-red">Moderate (KYC, compliance freezes)</td>
                    </tr>
                    <tr>
                        <td><strong>xrpay (Sovereign)</strong></td>
                        <td class="text-green">0% (Network fee only)</td>
                        <td class="text-green">3.5 Seconds</td>
                        <td class="text-green">Zero (P2P Direct-to-Key)</td>
                    </tr>
                </tbody>
            </table>
        </section>

        <section class="code-box">
            <div class="code-header">Spin Up Your Gateway Locally In 3 Seconds</div>
            <code>$ docker run -d -p 8080:8080 \
  -e XRPAY_MERCHANT_ADDR="{{.MerchantAddress}}" \
  -e XRPAY_API_KEY="your-highly-secure-gateway-token" \
  ghcr.io/psiloconvalley/xrpay:latest</code>
        </section>

        <footer>
            <p>xrpay &bull; MIT Licensed Sovereign Payment Software Engine</p>
            <p style="margin-top: 0.75rem;">
                <a href="https://github.com/psiloconvalley/xrpay" target="_blank">GitHub Repository</a> |
                <a href="/api/v1/telemetry">Live Telemetry JSON</a> |
                <a href="/healthz">Gateway Healthcheck</a>
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
    <title>Settle Invoice • xrpay</title>
    <style>
        :root {
            --bg-color: #030712;
            --card-bg: #0b0f19;
            --text-color: #f8fafc;
            --text-muted: #94a3b8;
            --border-color: #1f293d;
            --accent-emerald: #10b981;
            --accent-amber: #f59e0b;
            --accent-cyan: #38bdf8;
            --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 1.5rem;
        }
        .card {
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            max-width: 460px;
            width: 100%;
            padding: 2rem;
            position: relative;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
        }
        .card::before {
            content: '';
            position: absolute;
            top: 0; left: 0; right: 0; height: 4px;
            background: linear-gradient(90deg, var(--accent-cyan), var(--accent-emerald));
            border-radius: 16px 16px 0 0;
        }
        .header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 2rem;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 1rem;
        }
        .logo { font-size: 1.25rem; font-weight: 800; letter-spacing: -0.05em; }
        .logo span { color: var(--accent-cyan); }
        .badge {
            font-size: 0.7rem;
            font-weight: 700;
            padding: 0.3rem 0.6rem;
            border-radius: 6px;
            text-transform: uppercase;
            font-family: var(--font-mono);
            letter-spacing: 0.05em;
        }
        .badge-pending { background: rgba(245, 158, 11, 0.08); color: var(--accent-amber); border: 1px solid rgba(245, 158, 11, 0.2); }
        .badge-settled { background: rgba(16, 185, 129, 0.08); color: var(--accent-emerald); border: 1px solid rgba(16, 185, 129, 0.2); }
        .amount-box {
            text-align: center;
            margin-bottom: 1.75rem;
            background: #020617;
            padding: 1.5rem;
            border-radius: 10px;
            border: 1px solid var(--border-color);
        }
        .amount-xrp { font-size: 2.25rem; font-weight: 900; color: var(--accent-emerald); letter-spacing: -0.02em; }
        .field { margin-bottom: 1.25rem; }
        .field-label {
            font-size: 0.75rem;
            color: var(--text-muted);
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.08em;
            font-weight: 700;
        }
        .value-box {
            display: flex;
            align-items: center;
            background: #020617;
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 0.85rem;
        }
        .value-text {
            font-family: var(--font-mono);
            font-size: 0.88rem;
            flex-grow: 1;
            word-break: break-all;
            color: #f1f5f9;
        }
        .highlight { border-color: rgba(245, 158, 11, 0.4); background: rgba(245, 158, 11, 0.02); }
        .tag-text { font-size: 1.25rem; font-weight: 800; color: var(--accent-amber); }
        .btn-copy {
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid var(--border-color);
            color: var(--text-muted);
            padding: 0.35rem 0.65rem;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.75rem;
            font-weight: 600;
            margin-left: 0.75rem;
            transition: all 0.1s ease;
        }
        .btn-copy:hover { color: #fff; border-color: var(--text-muted); background: rgba(255, 255, 255, 0.08); }
        .warning {
            background: rgba(245, 158, 11, 0.04);
            border-left: 4px solid var(--accent-amber);
            padding: 1rem;
            font-size: 0.8rem;
            color: #fbbf24;
            margin-bottom: 1.75rem;
            border-radius: 0 8px 8px 0;
            line-height: 1.5;
        }
        .qr-section {
            display: flex;
            justify-content: center;
            margin-bottom: 1.75rem;
        }
        .qr-wrapper {
            background: #fff;
            padding: 12px;
            border-radius: 12px;
            width: 154px;
            height: 154px;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 4px 12px rgba(0,0,0,0.25);
        }
        .simulation-faucet-box {
            background: rgba(56, 189, 248, 0.04);
            border: 1px dashed rgba(56, 189, 248, 0.25);
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1.75rem;
            text-align: center;
        }
        .simulation-faucet-box p {
            font-size: 0.75rem;
            color: var(--text-muted);
            margin-bottom: 0.75rem;
        }
        .btn-simulate {
            background: var(--accent-cyan);
            color: var(--bg-color);
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            font-weight: 700;
            font-size: 0.8rem;
            cursor: pointer;
            width: 100%;
            transition: all 0.1s ease;
        }
        .btn-simulate:hover { opacity: 0.9; }
        .btn-simulate:disabled {
            background: var(--border-color);
            color: var(--text-muted);
            cursor: not-allowed;
        }
        .status-footer {
            text-align: center;
            padding-top: 1.25rem;
            border-top: 1px solid var(--border-color);
            font-size: 0.85rem;
            color: var(--text-muted);
            font-family: var(--font-mono);
        }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <a href="/" class="logo" style="color:inherit; text-decoration:none;">⏣ xr<span>pay</span></a>
            <span id="badge" class="badge badge-pending">{{.Invoice.Status}}</span>
        </div>

        <div class="amount-box">
            <div class="amount-xrp">{{.Invoice.AmountXRP}} XRP</div>
            {{if .Memo}}<div style="font-size:0.85rem;color:var(--text-muted);margin-top:0.4rem;font-weight:500;">{{.Memo}}</div>{{end}}
        </div>

        <div class="qr-section">
            <div class="qr-wrapper">
                <img src="https://api.qrserver.com/v1/create-qr-code/?size=130x130&data={{.QRPayload}}" alt="QR Code" width="130" height="130">
            </div>
        </div>

        <!-- Simulation Tool for immediate feedback on Testnet -->
        <div class="simulation-faucet-box">
            <p>Evaluating? Trigger an immediate simulated on-chain payment on the Ripple Testnet consensus network.</p>
            <button id="sim-btn" class="btn-simulate" onclick="simulatePayment()">⚡ Simulate Testnet Consensus Payment</button>
        </div>

        <div class="field">
            <div class="field-label">Merchant Classic Wallet Address</div>
            <div class="value-box">
                <span class="value-text" id="addr">{{.Invoice.MerchantAddress}}</span>
                <button class="btn-copy" onclick="copy('addr')">Copy</button>
            </div>
        </div>

        <div class="field">
            <div class="field-label">Required Destination Tag</div>
            <div class="value-box highlight">
                <span class="value-text tag-text" id="tag">{{.FormattedTag}}</span>
                <button class="btn-copy" onclick="copy('tag')">Copy</button>
            </div>
        </div>

        <div class="warning">
            <strong>CRITICAL:</strong> You must include the Destination Tag <strong>{{.FormattedTag}}</strong> precisely. If your wallet leaves this field blank, your funds cannot be routed to this invoice.
        </div>

        <div class="status-footer" id="status-box">
            <span>● Awaiting L1 Consensus ledger event...</span>
        </div>
    </div>

    <script>
        function copy(id) {
            const text = document.getElementById(id).innerText;
            navigator.clipboard.writeText(text).then(() => {
                const btn = event.target;
                const orig = btn.innerText;
                btn.innerText = "Copied!";
                setTimeout(() => { btn.innerText = orig; }, 1500);
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
                        document.getElementById("status-box").innerHTML = "✅ <span style='color:var(--accent-emerald)'><strong>Direct-to-Key Settlement Complete!</strong></span>";
                        document.getElementById("sim-btn").disabled = true;
                        document.getElementById("sim-btn").innerText = "Invoice Settled";
                        if (redirectUrl) {
                            setTimeout(() => { window.location.href = redirectUrl; }, 1500);
                        }
                    } else if (data.expired) {
                        document.getElementById("status-box").innerText = "❌ Invoice Expired";
                        document.getElementById("sim-btn").disabled = true;
                    } else {
                        setTimeout(poll, 1500);
                    }
                })
                .catch(() => setTimeout(poll, 3000));
        }

        function simulatePayment() {
            const btn = document.getElementById("sim-btn");
            btn.disabled = true;
            btn.innerText = "Querying Faucet Network...";

            fetch("https://faucet.altnet.rippletest.net/accounts", {
                method: "POST",
                headers: { "Content-Type": "application/json" }
            })
            .then(r => r.json())
            .then(faucetData => {
                btn.innerText = "Signing Simulated L1 Tx...";
                
                const txBody = {
                    method: "submit",
                    params: [{
                        tx_json: {
                            TransactionType: "Payment",
                            Account: faucetData.account.address,
                            Destination: "{{.Invoice.MerchantAddress}}",
                            DestinationTag: parseInt("{{.FormattedTag}}"),
                            Amount: "{{.Invoice.AmountDrops}}"
                        },
                        secret: faucetData.account.secret
                    }]
                };

                return fetch("https://s.altnet.rippletest.net:51234", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(txBody)
                });
            })
            .then(r => r.json())
            .then(rpcResp => {
                if (rpcResp.result && rpcResp.result.engine_result === "tesSUCCESS") {
                    btn.innerText = "Consensus Validating (3.5s)...";
                } else {
                    btn.innerText = "Retrying Tx Submission...";
                    btn.disabled = false;
                }
            })
            .catch(err => {
                console.error("Simulation error:", err);
                btn.innerText = "Simulation Timeout. Try again.";
                btn.disabled = false;
            });
        }

        poll();
    </script>
</body>
</html>`
