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
	"syscall"
	"time"

	"github.com/psiloconvalley/xrpay/internal/api"
	"github.com/psiloconvalley/xrpay/internal/domain"
	"github.com/psiloconvalley/xrpay/internal/store"
	"github.com/psiloconvalley/xrpay/internal/webhook"
	"github.com/psiloconvalley/xrpay/internal/xrpl"
)

// AppConfig holds environment-driven configuration for xrpay daemon.
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
		// Default public testnet test faucet account for development
		merchantAddr = "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V"
	}

	rpcURL := os.Getenv("XRPAY_RPC_URL")
	if rpcURL == "" {
		rpcURL = xrpl.TestnetRPCURL
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

// Validate checks the runtime configuration against strict production requirements.
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
	// 1. Initialize structured JSON logger (Charter §1.2)
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

	// Validate config before doing any work
	if err := cfg.Validate(); err != nil {
		logger.Error("startup configuration validation failed", "error", err)
		os.Exit(1)
	}

	// 2. Initialize Store
	memStore := store.NewMemoryStore()

	// 3. Initialize Webhook Dispatcher
	dispatcher, err := webhook.NewDispatcher(webhook.Config{
		WebhookSecret: cfg.WebhookSecret,
		MaxRetries:    3,
		InitialDelay:  500 * time.Millisecond,
		Logger:        logger,
	})
	if err != nil {
		logger.Error("failed to initialize webhook dispatcher", "error", err)
		os.Exit(1)
	}

	// 4. Initialize XRPL Client & Poller
	xrplClient := xrpl.NewClient(cfg.RPCURL)

	poller, err := xrpl.NewPoller(xrplClient, memStore, xrpl.PollerConfig{
		MerchantAccount: cfg.MerchantAccount,
		PollInterval:    cfg.PollInterval,
		StartLedger:     cfg.StartLedger,
		Logger:          logger,
		OnPayment: func(ctx context.Context, inv *domain.Invoice, event xrpl.PaymentEvent) {
			eventType := webhook.EventInvoiceSettled
			if inv.Status == domain.StatusPartiallyPaid {
				eventType = webhook.EventInvoicePartiallyPaid
			} else if inv.Status == domain.StatusOverpaid {
				eventType = webhook.EventInvoiceOverpaid
			}

			webhookEvent := webhook.Event{
				ID:        fmt.Sprintf("evt_%s_%d", inv.ID, time.Now().UnixNano()),
				Type:      eventType,
				CreatedAt: time.Now().UTC(),
				Invoice:   inv,
				TxHash:    event.TxHash,
			}

			// Fire webhook asynchronously so poller loop is not blocked
			go func() {
				dispatchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := dispatcher.DispatchSync(dispatchCtx, webhookEvent); err != nil {
					logger.ErrorContext(dispatchCtx, "webhook delivery failed", "error", err, "invoice_id", inv.ID)
				}
			}()
		},
	})
	if err != nil {
		logger.Error("failed to initialize poller", "error", err)
		os.Exit(1)
	}

	// 5. Initialize API Handler & Routes
	apiHandler, err := api.NewHandler(memStore, api.Config{
		MerchantAccount: cfg.MerchantAccount,
		BaseURL:         cfg.BaseURL,
		APIKey:          cfg.APIKey,
		DefaultDuration: 15 * time.Minute,
		Logger:          logger,
	})
	if err != nil {
		logger.Error("failed to initialize api handler", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	apiHandler.RegisterRoutes(mux)

	// Logging & Recovery Middleware
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

	// Wrap handler with CORS and Security Headers (Sweep 1)
	securedHandler := api.SecurityHeadersMiddleware(api.CORSMiddleware(handler))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      securedHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 6. Start Poller Background Worker
	pollerCtx, cancelPoller := context.WithCancel(context.Background())
	defer cancelPoller()

	if err := poller.Start(pollerCtx); err != nil {
		logger.Error("failed to start poller", "error", err)
		os.Exit(1)
	}

	// 7. Start HTTP Server in background goroutine
	go func() {
		logger.Info("HTTP server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 8. Graceful Shutdown on OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received; commencing graceful shutdown")

	// Stop poller worker
	cancelPoller()
	_ = poller.Stop()

	// Shutdown HTTP server with 10s deadline
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	} else {
		logger.Info("server exited cleanly")
	}
}
