package security_test

import (
	"errors"
	"net"
	"testing"

	"github.com/psiloconvalley/xrpay/internal/security"
)

func TestValidateCallbackURL(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		allowInsecure bool
		wantErr       error
	}{
		{
			name:          "empty url fails",
			rawURL:        "",
			allowInsecure: false,
			wantErr:       security.ErrEmptyURL,
		},
		{
			name:          "http fails in secure mode",
			rawURL:        "http://example.com/webhook",
			allowInsecure: false,
			wantErr:       security.ErrInvalidURLScheme,
		},
		{
			name:          "http allowed in insecure dev mode",
			rawURL:        "http://localhost:8080/webhook",
			allowInsecure: true,
			wantErr:       nil,
		},
		{
			name:          "aws metadata IP blocked (SSRF defense)",
			rawURL:        "https://169.254.169.254/latest/meta-data",
			allowInsecure: false,
			wantErr:       security.ErrPrivateIPBlocked,
		},
		{
			name:          "rfc1918 private ip blocked",
			rawURL:        "https://10.0.0.1/webhook",
			allowInsecure: false,
			wantErr:       security.ErrPrivateIPBlocked,
		},
		{
			name:          "loopback ip blocked in secure mode",
			rawURL:        "https://127.0.0.1:8443/webhook",
			allowInsecure: false,
			wantErr:       security.ErrPrivateIPBlocked,
		},
		{
			name:          "non-http schemes rejected",
			rawURL:        "ftp://example.com/webhook",
			allowInsecure: true,
			wantErr:       security.ErrInvalidURLScheme,
		},
		{
			name:          "valid public https URL passes",
			rawURL:        "https://api.github.com/webhook",
			allowInsecure: false,
			wantErr:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := security.ValidateCallbackURL(tt.rawURL, tt.allowInsecure)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error type %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	privateCases := []string{
		"127.0.0.1",
		"10.254.0.1",
		"172.16.0.5",
		"192.168.1.1",
		"169.254.169.254",
		"::1",
		"fe80::1",
		"fc00::1",
	}

	for _, ipStr := range privateCases {
		ip := net.ParseIP(ipStr)
		if !security.IsPrivateIP(ip) {
			t.Errorf("expected %s to be flagged as private/reserved", ipStr)
		}
	}

	publicCases := []string{
		"8.8.8.8",
		"1.1.1.1",
		"140.82.121.3",
		"2606:4700:4700::1111",
	}

	for _, ipStr := range publicCases {
		ip := net.ParseIP(ipStr)
		if security.IsPrivateIP(ip) {
			t.Errorf("expected %s to be recognized as public", ipStr)
		}
	}
}
