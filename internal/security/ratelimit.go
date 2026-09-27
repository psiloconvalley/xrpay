package security

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// clientBucket tracks token availability and last access for an IP.
type clientBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// IPRateLimiter implements a thread-safe token bucket rate limiter per IP.
type IPRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*clientBucket
	capacity   float64
	refillRate float64 // tokens per second
	ttl        time.Duration
	stopCh     chan struct{}
}

// NewIPRateLimiter creates a rate limiter with a maximum token capacity and refill rate.
// Example: NewIPRateLimiter(10, 10, time.Hour) allows 10 requests bursting, refilling 10 tokens per hour.
func NewIPRateLimiter(capacity float64, refillCount float64, refillInterval time.Duration) *IPRateLimiter {
	rate := refillCount / refillInterval.Seconds()
	limiter := &IPRateLimiter{
		buckets:    make(map[string]*clientBucket),
		capacity:   capacity,
		refillRate: rate,
		ttl:        2 * refillInterval,
		stopCh:     make(chan struct{}),
	}

	go limiter.cleanupLoop(refillInterval)
	return limiter
}

// Allow reports whether a request from the given IP address is permitted.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	b, exists := l.buckets[ip]
	if !exists {
		l.buckets[ip] = &clientBucket{
			tokens:     l.capacity - 1.0,
			lastRefill: now,
			lastSeen:   now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * l.refillRate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.lastRefill = now
	b.lastSeen = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}

	return false
}

// Stop terminates the background cleanup goroutine.
func (l *IPRateLimiter) Stop() {
	select {
	case <-l.stopCh:
	default:
		close(l.stopCh)
	}
}

// cleanupLoop periodically evicts inactive IP entries to prevent unbounded memory growth.
func (l *IPRateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case now := <-ticker.C:
			l.mu.Lock()
			cutoff := now.UTC().Add(-l.ttl)
			for ip, b := range l.buckets {
				if b.lastSeen.Before(cutoff) {
					delete(l.buckets, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

// ExtractClientIP retrieves the client's IP. When trustProxy is true, it inspects X-Forwarded-For.
func ExtractClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			parts := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(parts[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return ip
}

// RateLimitMiddleware wraps an http.HandlerFunc with IP-based token bucket rate limiting.
func RateLimitMiddleware(limiter *IPRateLimiter, trustProxy bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := ExtractClientIP(r, trustProxy)
		if !limiter.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded: please try again later",
			})
			return
		}
		next(w, r)
	}
}
