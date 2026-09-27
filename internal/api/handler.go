package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/security"
	"github.com/psiloconvalley/xrpay/internal/store"
)

type CreateInvoiceRequest struct {
	OrderID     string            `json:"order_id"`
	Amount      string            `json:"amount"` // XRP string (e.g. "5.0")
	AmountDrops int64             `json:"amount_drops"`
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
	RedirectURL string               `json:"redirect_url,omitempty"`
}

type TelemetryResponse struct {
	ServerTime      string `json:"server_time"`
	MerchantAddress string `json:"merchant_address"`
	GoVersion       string `json:"go_version"`
	NumGoroutines   int    `json:"num_goroutines"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
}

type Handler struct {
	store           store.InvoiceStore
	merchantAddress string
	defaultExpiry   time.Duration
	checkoutTmpl    *template.Template
	landingTmpl     *template.Template
	startTime       time.Time
}

func NewHandler(s store.InvoiceStore, merchantAddress string, defaultExpiry time.Duration) *Handler {
	if defaultExpiry <= 0 {
		defaultExpiry = 15 * time.Minute
	}

	return &Handler{
		store:           s,
		merchantAddress: merchantAddress,
		defaultExpiry:   defaultExpiry,
		checkoutTmpl:    template.Must(template.New("checkout").Parse(checkoutHTMLTemplate)),
		landingTmpl:     template.Must(template.New("landing").Parse(landingHTMLTemplate)),
		startTime:       time.Now().UTC(),
	}
}

func (h *Handler) LandingHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := struct {
		MerchantAddress string
		DefaultAmount   string
	}{
		MerchantAddress: h.merchantAddress,
		DefaultAmount:   "5.00",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.landingTmpl.Execute(w, data)
}

func (h *Handler) DemoInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	var amountXRP string
	var memo string

	isForm := strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
	if isForm {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}
		amountXRP = strings.TrimSpace(r.FormValue("amount"))
		memo = strings.TrimSpace(r.FormValue("memo"))
	} else {
		var req struct {
			Amount string `json:"amount"`
			Memo   string `json:"memo"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json payload", http.StatusBadRequest)
			return
		}
		amountXRP = strings.TrimSpace(req.Amount)
		memo = strings.TrimSpace(req.Memo)
	}

	if amountXRP == "" {
		amountXRP = "5.00"
	}
	if memo == "" {
		memo = "SaaS Sandbox Payment"
	}

	if err := security.ValidateMemoField(memo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	drops, err := domain.ParseXRP(amountXRP)
	if err != nil || drops <= 0 {
		http.Error(w, "invalid XRP amount: must be positive numeric value", http.StatusBadRequest)
		return
	}

	if drops < 1000000 {
		http.Error(w, "demo amount must be at least 1.00 XRP to prevent spam", http.StatusBadRequest)
		return
	}

	tag, err := h.store.AllocateTag(r.Context(), h.merchantAddress)
	if err != nil {
		http.Error(w, "failed to allocate destination tag", http.StatusInternalServerError)
		return
	}

	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	invID := "inv_" + hex.EncodeToString(idBytes)

	meta := map[string]string{
		"demo": "true",
		"memo": memo,
	}

	inv, err := domain.NewInvoice(
		invID,
		h.merchantAddress,
		tag,
		drops,
		15*time.Minute,
		time.Now().UTC(),
		"",
		"",
		meta,
	)
	if err != nil {
		http.Error(w, "failed to generate sandbox invoice", http.StatusBadRequest)
		return
	}

	if err := h.store.Save(r.Context(), inv); err != nil {
		http.Error(w, "failed saving sandbox invoice", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	checkoutURL := fmt.Sprintf("%s://%s/checkout/%s", scheme, r.Host, inv.ID)

	if isForm {
		http.Redirect(w, r, checkoutURL, http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":           inv.ID,
		"checkout_url": checkoutURL,
	})
}

func (h *Handler) TelemetryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := TelemetryResponse{
		ServerTime:      time.Now().UTC().Format(time.RFC3339),
		MerchantAddress: h.merchantAddress,
		GoVersion:       runtime.Version(),
		NumGoroutines:   runtime.NumGoroutine(),
		UptimeSeconds:   int64(time.Since(h.startTime).Seconds()),
	}

	w.Header().Set("Cache-Control", "public, max-age=3")
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateInvoiceRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	if err := security.ValidateMetadataMap(req.Metadata); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.WebhookURL != "" {
		allowInsecure := strings.Contains(r.Host, "localhost") || strings.Contains(r.Host, "127.0.0.1")
		if err := security.ValidateCallbackURL(req.WebhookURL, allowInsecure); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
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
		RedirectURL: inv.RedirectURL,
	}

	writeJSON(w, http.StatusOK, resp)
}

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

	qrPayload := fmt.Sprintf("xrpl:%s?dt=%d&amount=%d", inv.MerchantAddress, inv.DestinationTag, inv.AmountDrops)

	data := struct {
		Invoice      *domain.Invoice
		FormattedTag string
		QRPayload    string
		Memo         string
	}{
		Invoice:      inv,
		FormattedTag: strconv.FormatUint(uint64(inv.DestinationTag), 10),
		QRPayload:    qrPayload,
		Memo:         inv.Metadata["memo"],
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.checkoutTmpl.Execute(w, data)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
