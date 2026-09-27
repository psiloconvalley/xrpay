package security_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/psiloconvalley/xrpay/internal/security"
)

func TestIPRateLimiter_BurstAndRefill(t *testing.T) {
	limiter := security.NewIPRateLimiter(3, 3, time.Second)
	defer limiter.Stop()

	ip := "192.0.2.1"

	for i := 0; i < 3; i++ {
		if !limiter.Allow(ip) {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	if limiter.Allow(ip) {
		t.Fatal("4th request should have been rejected (rate limited)")
	}

	otherIP := "192.0.2.2"
	if !limiter.Allow(otherIP) {
		t.Fatal("different IP should have separate quota")
	}
}

func TestIPRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := security.NewIPRateLimiter(50, 50, time.Second)
	defer limiter.Stop()

	var wg sync.WaitGroup
	allowedCount := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("198.51.100.1") {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowedCount != 50 {
		t.Fatalf("expected exactly 50 allowed requests in concurrent burst, got %d", allowedCount)
	}
}

func TestExtractClientIP(t *testing.T) {
	t.Run("trusts X-Forwarded-For when trustProxy is true", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
		ip := security.ExtractClientIP(req, true)
		if ip != "203.0.113.195" {
			t.Fatalf("expected 203.0.113.195, got %s", ip)
		}
	})

	t.Run("ignores X-Forwarded-For when trustProxy is false", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.100:44321"
		req.Header.Set("X-Forwarded-For", "203.0.113.195")
		ip := security.ExtractClientIP(req, false)
		if ip != "192.0.2.100" {
			t.Fatalf("expected RemoteAddr 192.0.2.100, got %s", ip)
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := security.NewIPRateLimiter(1, 1, time.Hour)
	defer limiter.Stop()

	dummyHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	handler := security.RateLimitMiddleware(limiter, false, dummyHandler)

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "198.51.100.5:12345"
	rr1 := httptest.NewRecorder()
	handler(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "198.51.100.5:12345"
	rr2 := httptest.NewRecorder()
	handler(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}
