package api

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

type CreateInvoiceRequest struct {
	OrderID     string            `json:"order_id"`
	Amount      string            `json:"amount"`       // Either XRP (e.g. "5.0") or Drops
	AmountDrops int64             `json:"amount_drops"` // Explicit drops
	ExpirySecs  int64             `json:"expiry_seconds"`
	RedirectURL string            `json:"redirect_url"`
	WebhookURL  string            `json:"webhook_url"`
	Metadata    map[string]string `json:"metadata"`
}

type InvoiceStatusResponse struct {
	ID          string               `json:"id"`
	OrderID     string               `json:"order_id"`
	Status      domain.InvoiceStatus `json:"status"`
	AmountDrops domain.Drops         `json:"amount_drops"`
	AmountXRP   string               `json:"amount_xrp"`
	PaidDrops   domain.Drops         `json:"paid_drops"`
	PaidXRP     string               `json:"paid_xrp"`
	TxHash      string               `json:"tx_hash,omitempty"`
	Settled     bool                 `json:"settled"`
	Expired     bool                 `json:"expired"`
}

type Handler struct {
	store           store.InvoiceStore
	merchantAddress string
	defaultExpiry   time.Duration
	checkoutTmpl    *template.Template
}

func NewHandler(s store.InvoiceStore, merchantAddress string, defaultExpiry time.Duration) *Handler {
	if defaultExpiry <= 0 {
		defaultExpiry = 15 * time.Minute
	}

	tmpl := template.Must(template.New("checkout").Parse(checkoutHTMLTemplate))

	return &Handler{
		store:           s,
		merchantAddress: merchantAddress,
		defaultExpiry:   defaultExpiry,
		checkoutTmpl:    tmpl,
	}
}

// CreateInvoiceHandler handles POST /api/v1/invoices
func (h *Handler) CreateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Restrict request body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateInvoiceRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	var drops domain.Drops
	if req.AmountDrops > 0 {
		drops = domain.Drops(req.AmountDrops)
	} else if req.Amount != "" {
		parsed, err := domain.ParseXRP(req.Amount)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount: " + err.Error()})
			return
		}
		drops = parsed
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "either amount or amount_drops is required"})
		return
	}

	expiry := h.defaultExpiry
	if req.ExpirySecs > 0 {
		expiry = time.Duration(req.ExpirySecs) * time.Second
	}

	destTag, err := h.store.AllocateTag(r.Context(), h.merchantAddress)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed allocating tag"})
		return
	}

	inv, err := domain.NewInvoice(
		req.OrderID,
		h.merchantAddress,
		destTag,
		drops,
		expiry,
		time.Now().UTC(),
		req.RedirectURL,
		req.WebhookURL,
		req.Metadata,
	)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.store.Save(r.Context(), inv); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed saving invoice"})
		return
	}

	writeJSON(w, http.StatusCreated, inv)
}

// GetInvoiceHandler handles GET /api/v1/invoices/{id} (Protected Private Details)
func (h *Handler) GetInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/invoices/")

	inv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "invoice not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}

	writeJSON(w, http.StatusOK, inv)
}

// GetInvoicePublicStatusHandler handles GET /api/v1/invoices/{id}/status (Public Safe Endpoint)
func (h *Handler) GetInvoicePublicStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/invoices/")
	id = strings.TrimSuffix(id, "/status")

	inv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "invoice not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}

	resp := InvoiceStatusResponse{
		ID:          inv.ID,
		OrderID:     inv.OrderID,
		Status:      inv.Status,
		AmountDrops: inv.AmountDrops,
		AmountXRP:   inv.AmountXRP,
		PaidDrops:   inv.PaidDrops,
		PaidXRP:     inv.PaidXRP,
		TxHash:      inv.TxHash,
		Settled:     inv.Status == domain.StatusSettled || inv.Status == domain.StatusOverpaid,
		Expired:     inv.Status == domain.StatusExpired,
	}

	writeJSON(w, http.StatusOK, resp)
}

// CheckoutHandler serves the HTML UI for the customer checkout process
func (h *Handler) CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/checkout/")

	inv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}

	data := struct {
		Invoice     *domain.Invoice
		FormattedTag string
	}{
		Invoice:      inv,
		FormattedTag: strconv.FormatUint(uint64(inv.DestinationTag), 10),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.checkoutTmpl.Execute(w, data)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

const checkoutHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pay with XRP</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: #1e293b;
            --text-color: #f8fafc;
            --text-muted: #94a3b8;
            --primary: #3b82f6;
            --primary-hover: #2563eb;
            --success: #10b981;
            --danger: #ef4444;
            --border-color: #334155;
        }
        body {
            background-color: var(--bg-color);
            color: var(--text-color);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            margin: 0;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
        }
        .container {
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 32px;
            width: 100%;
            max-width: 420px;
            box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.3);
            box-sizing: border-box;
            text-align: center;
        }
        h2 { margin: 0 0 8px 0; font-size: 24px; font-weight: 700; }
        .subtitle { color: var(--text-muted); font-size: 14px; margin-bottom: 24px; }
        .amount-box {
            background-color: rgba(59, 130, 246, 0.1);
            border: 1px dashed var(--primary);
            border-radius: 8px;
            padding: 16px;
            margin-bottom: 24px;
        }
        .amount-val { font-size: 32px; font-weight: 800; color: var(--primary); }
        .amount-lbl { font-size: 12px; color: var(--text-muted); text-transform: uppercase; margin-top: 4px; }
        .instruction { text-align: left; background: #0f172a; border-radius: 8px; padding: 16px; margin-bottom: 24px; border: 1px solid var(--border-color); }
        .inst-row { display: flex; justify-content: space-between; margin-bottom: 12px; font-size: 14px; }
        .inst-row:last-child { margin-bottom: 0; }
        .lbl { color: var(--text-muted); }
        .val { font-family: monospace; font-weight: 600; cursor: pointer; color: #fff; background: #1e293b; padding: 2px 6px; border-radius: 4px; word-break: break-all; }
        .val:hover { background: #334155; }
        .status-badge {
            display: inline-block;
            padding: 8px 16px;
            border-radius: 9999px;
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 8px;
            text-transform: uppercase;
        }
        .status-PENDING { background-color: rgba(59, 130, 246, 0.2); color: var(--primary); }
        .status-PARTIALLY_PAID { background-color: rgba(245, 158, 11, 0.2); color: #f59e0b; }
        .status-SETTLED { background-color: rgba(16, 185, 129, 0.2); color: var(--success); }
        .status-OVERPAID { background-color: rgba(16, 185, 129, 0.2); color: var(--success); }
        .status-EXPIRED { background-color: rgba(239, 68, 68, 0.2); color: var(--danger); }
        .timer { font-size: 13px; color: var(--text-muted); margin-top: 8px; }
        .footer { font-size: 11px; color: var(--text-muted); margin-top: 32px; display: flex; align-items: center; justify-content: center; gap: 4px; }
        .footer a { color: var(--primary); text-decoration: none; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Pay with XRP</h2>
        <div class="subtitle">Order #{{ .Invoice.OrderID }}</div>

        <div class="amount-box">
            <div class="amount-val">{{ .Invoice.AmountXRP }} XRP</div>
            <div class="amount-lbl">Total Amount Due</div>
        </div>

        <div class="instruction">
            <div class="inst-row">
                <span class="lbl">Recipient Address:</span>
                <span class="val" onclick="copyText('{{ .Invoice.MerchantAddress }}')">{{ .Invoice.MerchantAddress }}</span>
            </div>
            <div class="inst-row">
                <span class="lbl">Destination Tag:</span>
                <span class="val" onclick="copyText('{{ .FormattedTag }}')" style="font-size: 16px; color: #f59e0b;">{{ .FormattedTag }}</span>
            </div>
        </div>

        <div style="margin-top: 16px;">
            <div id="statusBadge" class="status-badge status-{{ .Invoice.Status }}">{{ .Invoice.Status }}</div>
            <div id="countdown" class="timer">Checking payment status...</div>
        </div>

        <div class="footer">
            Powered by <a href="https://github.com/psiloconvalley/xrpay" target="_blank">xrpay</a> payment gateway
        </div>
    </div>

    <script>
        function copyText(text) {
            navigator.clipboard.writeText(text).then(() => {
                alert("Copied to clipboard: " + text);
            });
        }

        const invoiceId = "{{ .Invoice.ID }}";
        const expiresAt = new Date("{{ .Invoice.ExpiresAt.Format "2006-01-02T15:04:05Z07:00" }}").getTime();

        function updateTimer() {
            const now = new Date().getTime();
            const distance = expiresAt - now;

            if (distance < 0) {
                document.getElementById("countdown").innerHTML = "Invoice expired";
                return;
            }

            const minutes = Math.floor((distance % (1000 * 64 * 60)) / (1000 * 60));
            const seconds = Math.floor((distance % (1000 * 60)) / 1000);
            document.getElementById("countdown").innerHTML = "Payment window closes in " + minutes + "m " + seconds + "s";
        }

        setInterval(updateTimer, 1000);
        updateTimer();

        function pollStatus() {
            fetch("/api/v1/invoices/" + invoiceId + "/status")
                .then(r => r.json())
                .then(data => {
                    const badge = document.getElementById("statusBadge");
                    badge.innerHTML = data.status;
                    badge.className = "status-badge status-" + data.status;

                    if (data.settled) {
                        badge.innerHTML = "Payment Received! Thank you.";
                        document.getElementById("countdown").innerHTML = "Redirecting...";
                        setTimeout(() => {
                            if (data.redirect_url) {
                                window.location.href = data.redirect_url;
                            }
                        }, 2000);
                        return;
                    }

                    if (data.status !== "EXPIRED") {
                        setTimeout(pollStatus, 2000);
                    }
                })
                .catch(() => setTimeout(pollStatus, 4000));
        }
        pollStatus();
    </script>
</body>
</html>`
