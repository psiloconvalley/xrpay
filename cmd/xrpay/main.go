package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/psiloconvalley/xrpay/internal/api"
	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
	"github.com/psiloconvalley/xrpay/internal/webhook"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

type AppConfig struct {
	Port            string
	MerchantAccount string
	RPCURL          string
	WebhookSecret   string
	APIKey          string
	Environment     string
	BaseURL         string
	PollInterval    time.Duration
	StartLedger     int64
}

func loadConfig() AppConfig {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	merchantAddr := os.Getenv("XRPAY_MERCHANT_ADDR")
	if merchantAddr == "" {
		merchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"
	}

	rpcURL := os.Getenv("XRPAY_RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://s.altnet.rippletest.net:51234"
	}

	secret := os.Getenv("XRPAY_WEBHOOK_SECRET")
	if secret == "" {
		secret = "xrpay_default_insecure_development_secret_key"
	}

	apiKey := os.Getenv("XRPAY_API_KEY")

	env := os.Getenv("XRPAY_ENV")
	if env == "" {
		env = "development"
	}

	baseURL := os.Getenv("XRPAY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}

	pollSec := 3
	if s := os.Getenv("XRPAY_POLL_INTERVAL_SECONDS"); s != "" {
		if val, err := strconv.Atoi(s); err == nil && val > 0 {
			pollSec = val
		}
	}

	var startLedger int64 = -1
	if sl := os.Getenv("XRPAY_START_LEDGER"); sl != "" {
		if val, err := strconv.ParseInt(sl, 10, 64); err == nil {
			startLedger = val
		}
	}

	return AppConfig{
		Port:            port,
		MerchantAccount: merchantAddr,
		RPCURL:          rpcURL,
		WebhookSecret:   secret,
		APIKey:          apiKey,
		Environment:     env,
		BaseURL:         baseURL,
		PollInterval:    time.Duration(pollSec) * time.Second,
		StartLedger:     startLedger,
	}
}

func (c AppConfig) Validate() error {
	if err := domain.ValidateXRPLAddress(c.MerchantAccount); err != nil {
		return fmt.Errorf("invalid merchant address: %w", err)
	}
	if c.Environment == "production" {
		if c.WebhookSecret == "xrpay_default_insecure_development_secret_key" {
			return errors.New("XRPAY_WEBHOOK_SECRET must be set to a secure custom value in production")
		}
		if c.APIKey == "" {
			return errors.New("XRPAY_API_KEY is required in production mode")
		}
		if len(c.APIKey) < 16 {
			return errors.New("XRPAY_API_KEY must be at least 16 characters in production")
		}
	}
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()
	logger.Info("starting xrpay gateway",
		"version", "1.0.0",
		"environment", cfg.Environment,
		"port", cfg.Port,
		"merchant_account", cfg.MerchantAccount,
		"rpc_url", cfg.RPCURL,
		"poll_interval", cfg.PollInterval.String(),
	)

	if err := cfg.Validate(); err != nil {
		logger.Error("startup configuration validation failed", "error", err)
		os.Exit(1)
	}

	// 1. Store
	memStore := store.NewMemoryStore(1000)

	// 2. Webhook Dispatcher
	dispatcher := webhook.NewDispatcher(cfg.WebhookSecret, 10*time.Second)

	// 3. XRPL Client & Poller
	xrplClient := xrpl.NewClient(cfg.RPCURL, 10*time.Second)
	poller := xrpl.NewPoller(xrplClient, memStore, dispatcher, cfg.MerchantAccount, cfg.PollInterval, cfg.StartLedger)

	// 4. API Handlers & Routes
	apiHandler := api.NewHandler(memStore, cfg.MerchantAccount, 15*time.Minute)

	mux := http.NewServeMux()

	// Health Check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Checkout UI (Public)
	mux.HandleFunc("/checkout/", apiHandler.CheckoutHandler)

	// Invoice Status (Public Safe Endpoint)
	mux.HandleFunc("/api/v1/invoices/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") && r.Method == http.MethodGet {
			apiHandler.GetInvoicePublicStatusHandler(w, r)
			return
		}
		if r.Method == http.MethodPost {
			api.RequireAuth(cfg.APIKey, apiHandler.CreateInvoiceHandler).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			api.RequireAuth(cfg.APIKey, apiHandler.GetInvoiceHandler).ServeHTTP(w, r)
			return
		}
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// Global HTTP middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				logger.ErrorContext(r.Context(), "panic recovered in HTTP handler", "panic", rec)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		mux.ServeHTTP(w, r)

		logger.InfoContext(r.Context(), "http request served",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})

	securedHandler := api.SecurityHeadersMiddleware(api.CORSMiddleware(handler))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      securedHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start Poller
	pollerCtx, cancelPoller := context.WithCancel(context.Background())
	poller.Start(pollerCtx)

	// Start HTTP Server
	go func() {
		logger.Info("HTTP server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received; commencing graceful shutdown")

	cancelPoller()
	poller.Stop()
	dispatcher.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	} else {
		logger.Info("server exited cleanly")
	}
}

