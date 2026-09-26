package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// SecurityHeadersMiddleware injects defensive HTTP security headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware handles preflight OPTIONS requests and sets CORS headers for API consumers.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAuth enforces Bearer token authentication on protected endpoints.
// Uses constant-time comparison to prevent timing side-channel attacks (Charter §4.2).
func RequireAuth(apiKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			// If no API key configured (development mode), bypass check
			next(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "missing or malformed authorization header (expected 'Bearer <api_key>')",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid api key",
			})
			return
		}

		next(w, r)
	}
}
