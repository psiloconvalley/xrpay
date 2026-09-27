package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

var (
	ErrEmptyURL            = errors.New("callback URL cannot be empty")
	ErrInvalidURLScheme    = errors.New("callback URL must use HTTPS scheme")
	ErrMissingHost         = errors.New("callback URL must include a valid host")
	ErrPrivateIPBlocked    = errors.New("callback URL resolves to private or reserved IP address (SSRF protection)")
	ErrURLResolutionFailed = errors.New("failed to resolve callback URL host")
)

var privateIPBlocks []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",       // IPv4 loopback
		"10.0.0.0/8",        // RFC1918
		"172.16.0.0/12",     // RFC1918
		"192.168.0.0/16",    // RFC1918
		"169.254.0.0/16",    // IPv4 link-local / cloud metadata (AWS, GCP, Azure)
		"0.0.0.0/8",         // Current network
		"::1/128",           // IPv6 loopback
		"fe80::/10",         // IPv6 link-local
		"fc00::/7",          // IPv6 unique local
		"::/128",            // IPv6 unspecified
	}

	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// IsPrivateIP checks whether a net.IP is in a loopback, private, or link-local range.
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateCallbackURL enforces SSRF defense against incoming webhook target URLs.
// When allowInsecure is true (e.g. local dev), http:// and loopback addresses are allowed.
func ValidateCallbackURL(rawURL string, allowInsecure bool) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ErrEmptyURL
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if allowInsecure {
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("%w: received %s", ErrInvalidURLScheme, scheme)
		}
	} else {
		if scheme != "https" {
			return ErrInvalidURLScheme
		}
	}

	host := parsed.Hostname()
	if host == "" {
		return ErrMissingHost
	}

	// In dev mode with allowInsecure, skip DNS and private IP checks for localhost
	if allowInsecure && (host == "localhost" || host == "127.0.0.1" || host == "::1") {
		return nil
	}

	// If host is a direct IP address
	if ip := net.ParseIP(host); ip != nil {
		if !allowInsecure && IsPrivateIP(ip) {
			return ErrPrivateIPBlocked
		}
		return nil
	}

	// Resolve hostname with 2-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var r net.Resolver
	ips, err := r.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("%w: %s", ErrURLResolutionFailed, host)
	}

	if !allowInsecure {
		for _, ip := range ips {
			if IsPrivateIP(ip) {
				return ErrPrivateIPBlocked
			}
		}
	}

	return nil
}
