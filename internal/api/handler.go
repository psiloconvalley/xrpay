package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
)

// InvoiceManager defines the storage and tag allocation capabilities required by the API.
type InvoiceManager interface {
	Save(ctx context.Context, inv *domain.Invoice) error
	GetByID(ctx context.Context, id string) (*domain.Invoice, error)
	AllocateTag(ctx context.Context, merchantAddr string) (uint32, error)
}

// Config holds runtime parameters for the HTTP API.
type Config struct {
	MerchantAccount string
	BaseURL         string
	DefaultDuration time.Duration
	Logger          *slog.Logger
}

// Handler serves REST API endpoints and the checkout frontend.
type Handler struct {
	store           InvoiceManager
	merchantAccount string
	baseURL         string
	defaultDuration time.Duration
	logger          *slog.Logger
	checkoutTmpl    *template.Template
}

// CreateInvoiceRequest is the JSON payload for creating a new invoice.
type CreateInvoiceRequest struct {
	OrderID         string            `json:"order_id"`
	Amount          string            `json:"amount"` // in XRP (e.g., "10.5")
	DurationMinutes int               `json:"duration_minutes,omitempty"`
	WebhookURL      string            `json:"webhook_url,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// InvoiceResponse is the standardized JSON response for an invoice.
type InvoiceResponse struct {
	ID              string            `json:"id"`
	OrderID         string            `json:"order_id"`
	MerchantAddress string            `json:"merchant_address"`
	DestinationTag  uint32            `json:"destination_tag"`
	AmountExpected  string            `json:"amount_expected"`
	AmountPaid      string            `json:"amount_paid"`
	Status          string            `json:"status"`
	PaymentURI      string            `json:"payment_uri"`
	CheckoutURL     string            `json:"checkout_url"`
	TxHash          string            `json:"tx_hash,omitempty"`
	CreatedAt       string            `json:"created_at"`
	ExpiresAt       string            `json:"expires_at"`
	SettledAt       *string           `json:"settled_at,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// NewHandler constructs and validates an API handler.
func NewHandler(sm InvoiceManager, cfg Config) (*Handler, error) {
	if sm == nil {
		return nil, errors.New("store cannot be nil")
	}
	if len(cfg.MerchantAccount) < 25 || cfg.MerchantAccount[0] != 'r' {
		return nil, fmt.Errorf("invalid merchant address: %q", cfg.MerchantAccount)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.DefaultDuration <= 0 {
		cfg.DefaultDuration = 15 * time.Minute
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	tmpl, err := template.New("checkout").Parse(checkoutHTML)
	if err != nil {
		return nil, fmt.Errorf("parse checkout template: %w", err)
	}

	return &Handler{
		store:           sm,
		merchantAccount: cfg.MerchantAccount,
		baseURL:         cfg.BaseURL,
		defaultDuration: cfg.DefaultDuration,
		logger:          cfg.Logger.With("component", "api.handler"),
		checkoutTmpl:    tmpl,
	}, nil
}

// RegisterRoutes mounts API endpoints onto a http.ServeMux using Go 1.22 routing.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.handleHealthCheck)
	mux.HandleFunc("POST /api/v1/invoices", h.handleCreateInvoice)
	mux.HandleFunc("GET /api/v1/invoices/{id}", h.handleGetInvoice)
	mux.HandleFunc("GET /checkout/{id}", h.handleCheckoutPage)
}

func (h *Handler) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.OrderID == "" {
		h.writeError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	amount, err := domain.ParseXRP(req.Amount)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid amount %q: %v", req.Amount, err))
		return
	}

	duration := h.defaultDuration
	if req.DurationMinutes > 0 {
		duration = time.Duration(req.DurationMinutes) * time.Minute
	}

	tag, err := h.store.AllocateTag(r.Context(), h.merchantAccount)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to allocate destination tag", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to allocate payment destination tag")
		return
	}

	inv, err := domain.NewInvoice(req.OrderID, h.merchantAccount, tag, amount, duration, req.WebhookURL, req.Metadata)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to create invoice: "+err.Error())
		return
	}

	if err := h.store.Save(r.Context(), inv); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to save invoice", "invoice_id", inv.ID, "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to persist invoice")
		return
	}

	h.logger.InfoContext(r.Context(), "invoice created",
		"invoice_id", inv.ID,
		"order_id", inv.OrderID,
		"tag", inv.DestinationTag,
		"amount", inv.AmountExpected.String(),
	)

	resp := h.toInvoiceResponse(inv)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "invoice id is required")
		return
	}

	inv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "invoice not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "failed to fetch invoice", "invoice_id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal storage error")
		return
	}

	writeJSON(w, http.StatusOK, h.toInvoiceResponse(inv))
}

func (h *Handler) handleCheckoutPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Invoice ID required", http.StatusBadRequest)
		return
	}

	inv, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}

	resp := h.toInvoiceResponse(inv)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.checkoutTmpl.Execute(w, resp); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to render checkout template", "error", err)
	}
}

func (h *Handler) toInvoiceResponse(inv *domain.Invoice) InvoiceResponse {
	amountXRP := inv.AmountExpected.ToXRPString()
	paymentURI := fmt.Sprintf("xrpl:%s?amount=%s&dt=%d", inv.MerchantAddr, amountXRP, inv.DestinationTag)
	checkoutURL := fmt.Sprintf("%s/checkout/%s", h.baseURL, inv.ID)

	var settledAtStr *string
	if inv.SettledAt != nil {
		s := inv.SettledAt.UTC().Format(time.RFC3339)
		settledAtStr = &s
	}

	return InvoiceResponse{
		ID:              inv.ID,
		OrderID:         inv.OrderID,
		MerchantAddress: inv.MerchantAddr,
		DestinationTag:  inv.DestinationTag,
		AmountExpected:  inv.AmountExpected.String(),
		AmountPaid:      inv.AmountPaid.String(),
		Status:          string(inv.Status),
		PaymentURI:      paymentURI,
		CheckoutURL:     checkoutURL,
		TxHash:          inv.TxHash,
		CreatedAt:       inv.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt:       inv.ExpiresAt.UTC().Format(time.RFC3339),
		SettledAt:       settledAtStr,
		Metadata:        inv.Metadata,
	}
}

func (h *Handler) writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

// Embedded zero-dependency HTML template for the checkout page.
const checkoutHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pay with XRP | xrpay</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: #1e293b;
            --border-color: #334155;
            --text-primary: #f8fafc;
            --text-secondary: #94a3b8;
            --accent: #22d3ee;
            --success: #34d399;
            --warning: #fbbf24;
            --error: #f87171;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-color);
            color: var(--text-primary);
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            padding: 1rem;
        }
        .card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            width: 100%;
            max-width: 440px;
            padding: 2rem;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5);
        }
        .header { text-align: center; margin-bottom: 1.5rem; }
        .header h1 { font-size: 1.25rem; font-weight: 600; color: var(--text-secondary); }
        .amount { font-size: 2.25rem; font-weight: 700; color: var(--accent); margin-top: 0.25rem; }
        .field { margin-bottom: 1.25rem; }
        .field label { display: block; font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-secondary); margin-bottom: 0.25rem; }
        .field-val {
            background: #090d16;
            border: 1px solid var(--border-color);
            padding: 0.75rem;
            border-radius: 6px;
            font-family: "SF Mono", Monaco, Menlo, monospace;
            font-size: 0.875rem;
            word-break: break-all;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .tag-badge { background: #083344; color: var(--accent); font-weight: 700; padding: 0.25rem 0.5rem; border-radius: 4px; }
        .badge-status {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
        }
        .status-pending { background: rgba(251, 191, 36, 0.2); color: var(--warning); border: 1px solid var(--warning); }
        .status-settled { background: rgba(52, 211, 153, 0.2); color: var(--success); border: 1px solid var(--success); }
        .status-expired { background: rgba(248, 113, 113, 0.2); color: var(--error); border: 1px solid var(--error); }
        .warning-box {
            background: rgba(251, 191, 36, 0.1);
            border: 1px solid var(--warning);
            border-radius: 6px;
            padding: 0.75rem;
            font-size: 0.8rem;
            color: var(--warning);
            margin-bottom: 1.25rem;
            line-height: 1.4;
        }
        .btn-wallet {
            display: block;
            width: 100%;
            text-align: center;
            background: var(--accent);
            color: #0f172a;
            font-weight: 700;
            padding: 0.75rem;
            border-radius: 6px;
            text-decoration: none;
            margin-top: 1rem;
            transition: opacity 0.2s;
        }
        .btn-wallet:hover { opacity: 0.9; }
        .footer { text-align: center; margin-top: 1.5rem; font-size: 0.75rem; color: var(--text-secondary); }
    </style>
</head>
<body>
    <div class="card">
        <div class="header">
            <h1>Payment Required</h1>
            <div class="amount">{{.AmountExpected}}</div>
            <div style="margin-top: 0.5rem;">
                <span id="status-badge" class="badge-status status-{{.Status}}">{{.Status}}</span>
            </div>
        </div>

        <div class="warning-box">
            <strong>CRITICAL:</strong> You MUST include the <strong>Destination Tag</strong> when sending. Payments without tags cannot be credited.
        </div>

        <div class="field">
            <label>Recipient XRP Address</label>
            <div class="field-val">{{.MerchantAddress}}</div>
        </div>

        <div class="field">
            <label>Destination Tag (Required)</label>
            <div class="field-val"><span class="tag-badge">{{.DestinationTag}}</span></div>
        </div>

        <div class="field">
            <label>Order ID</label>
            <div class="field-val">{{.OrderID}}</div>
        </div>

        <a href="{{.PaymentURI}}" class="btn-wallet">Open in XRP Wallet</a>

        <div class="footer">
            Powered by <strong>xrpay</strong> &bull; Zero-fee XRP Gateway
        </div>
    </div>

    <script>
        // Auto-refresh status every 2.5 seconds until terminal
        const invoiceId = "{{.ID}}";
        const interval = setInterval(async () => {
            try {
                const res = await fetch("/api/v1/invoices/" + invoiceId);
                if (res.ok) {
                    const data = await res.json();
                    const badge = document.getElementById("status-badge");
                    badge.innerText = data.status;
                    badge.className = "badge-status status-" + data.status;
                    if (data.status === "settled" || data.status === "overpaid" || data.status === "expired") {
                        clearInterval(interval);
                        if (data.status === "settled" || data.status === "overpaid") {
                            setTimeout(() => alert("Payment Received! Thank you."), 500);
                        }
                    }
                }
            } catch (err) {
                console.error("Status check failed", err);
            }
        }, 2500);
    </script>
</body>
</html>`
