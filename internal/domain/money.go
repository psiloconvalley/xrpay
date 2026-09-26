package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// DropsPerXRP is the fixed conversion constant: 1 XRP = 1,000,000 drops (10^6).
const DropsPerXRP int64 = 1_000_000

var (
	ErrInvalidAmount  = errors.New("invalid monetary amount")
	ErrNegativeAmount = errors.New("amount cannot be negative")
	ErrAmountTooSmall = errors.New("amount must be at least 1 drop (0.000001 XRP)")
)

// Drops represents an exact amount in XRPL drops (integer arithmetic).
type Drops int64

// FromDrops creates a Drops value from an int64.
func FromDrops(d int64) (Drops, error) {
	if d < 0 {
		return 0, ErrNegativeAmount
	}
	return Drops(d), nil
}

// ParseXRP converts an XRP decimal string (e.g., "12.345678", "5", "0.5") into exact Drops.
// It parses without float64 conversion to avoid IEEE 754 precision loss.
func ParseXRP(s string) (Drops, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidAmount
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, ErrInvalidAmount
	}

	wholeStr := parts[0]
	if wholeStr == "" {
		wholeStr = "0"
	}
	whole, err := strconv.ParseInt(wholeStr, 10, 64)
	if err != nil || whole < 0 {
		return 0, ErrInvalidAmount
	}

	var frac int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) > 6 {
			return 0, ErrInvalidAmount
		}
		for len(fracStr) < 6 {
			fracStr += "0"
		}
		frac, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil || frac < 0 {
			return 0, ErrInvalidAmount
		}
	}

	total := (whole * DropsPerXRP) + frac
	if total <= 0 {
		return 0, ErrAmountTooSmall
	}

	return Drops(total), nil
}

// String formats the Drops back into a standard human-readable XRP string.
// Example: Drops(12345678) -> "12.345678 XRP"
func (d Drops) String() string {
	whole := int64(d) / DropsPerXRP
	frac := int64(d) % DropsPerXRP

	if frac == 0 {
		return fmt.Sprintf("%d XRP", whole)
	}

	fracStr := fmt.Sprintf("%06d", frac)
	fracStr = strings.TrimRight(fracStr, "0")

	return fmt.Sprintf("%d.%s XRP", whole, fracStr)
}

// ToXRPString returns just the numeric decimal string without the " XRP" suffix.
func (d Drops) ToXRPString() string {
	whole := int64(d) / DropsPerXRP
	frac := int64(d) % DropsPerXRP

	if frac == 0 {
		return fmt.Sprintf("%d", whole)
	}

	fracStr := fmt.Sprintf("%06d", frac)
	fracStr = strings.TrimRight(fracStr, "0")

	return fmt.Sprintf("%d.%s", whole, fracStr)
}

// Int64 returns the raw drops value as int64 for JSON-RPC payloads.
func (d Drops) Int64() int64 {
	return int64(d)
}
