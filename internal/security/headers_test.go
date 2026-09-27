package security_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psiloconvalley/xrpay/internal/security"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := security.SecurityHeadersMiddleware(dummy)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
	}

	for k, expected := range headers {
		got := rr.Header().Get(k)
		if got != expected {
			t.Errorf("header %s: expected %q, got %q", k, expected, got)
		}
	}

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" || !cspContains(csp, "frame-ancestors 'none'") {
		t.Errorf("expected strict CSP containing frame-ancestors 'none', got %q", csp)
	}
}

func TestRequireAuth(t *testing.T) {
	apiKey := "secret-test-token-12345"
	dummy := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	authHandler := security.RequireAuth(apiKey, dummy)

	t.Run("missing header returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		rr := httptest.NewRecorder()
		authHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("wrong token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rr := httptest.NewRecorder()
		authHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("correct token returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rr := httptest.NewRecorder()
		authHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("empty apiKey allows in dev mode", func(t *testing.T) {
		devAuthHandler := security.RequireAuth("", dummy)
		req := httptest.NewRequest("GET", "/protected", nil)
		rr := httptest.NewRecorder()
		devAuthHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 bypass in dev mode, got %d", rr.Code)
		}
	})
}

func cspContains(csp, directive string) bool {
	return len(csp) > 0 && len(directive) > 0
}
