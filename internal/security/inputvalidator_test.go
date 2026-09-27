package security_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/psiloconvalley/xrpay/internal/security"
)

func TestValidateSafeString(t *testing.T) {
	t.Run("valid ascii and unicode passes", func(t *testing.T) {
		valid := "Invoice #1042 — Payment for SaaS Subscription 🚀"
		if err := security.ValidateSafeString(valid, 1, 100); err != nil {
			t.Fatalf("unexpected error for valid string: %v", err)
		}
	})

	t.Run("rejects null byte and control characters", func(t *testing.T) {
		hostile := "Order\x00123\x1b[31m"
		err := security.ValidateSafeString(hostile, 1, 50)
		if !errors.Is(err, security.ErrInputControlChars) {
			t.Fatalf("expected ErrInputControlChars, got %v", err)
		}
	})

	t.Run("rejects oversized input", func(t *testing.T) {
		longStr := strings.Repeat("A", 101)
		err := security.ValidateSafeString(longStr, 1, 100)
		if !errors.Is(err, security.ErrInputTooLong) {
			t.Fatalf("expected ErrInputTooLong, got %v", err)
		}
	})

	t.Run("rejects invalid utf8 sequences", func(t *testing.T) {
		badUTF8 := string([]byte{0xff, 0xfe, 0xfd})
		err := security.ValidateSafeString(badUTF8, 1, 50)
		if !errors.Is(err, security.ErrInputInvalidUTF8) {
			t.Fatalf("expected ErrInputInvalidUTF8, got %v", err)
		}
	})
}

func TestValidateMemoField(t *testing.T) {
	t.Run("valid memo passes", func(t *testing.T) {
		err := security.ValidateMemoField("Subscription Renewal - Plan Pro")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects script injection attempt", func(t *testing.T) {
		hostile := "<script>alert(1)</script>"
		err := security.ValidateMemoField(hostile)
		if !errors.Is(err, security.ErrMemoContainsHTMLTags) {
			t.Fatalf("expected ErrMemoContainsHTMLTags, got %v", err)
		}
	})

	t.Run("rejects oversized memo", func(t *testing.T) {
		oversized := strings.Repeat("M", security.MaxMemoLength+1)
		err := security.ValidateMemoField(oversized)
		if !errors.Is(err, security.ErrInputTooLong) {
			t.Fatalf("expected ErrInputTooLong, got %v", err)
		}
	})
}

func TestValidateMetadataMap(t *testing.T) {
	t.Run("valid metadata passes", func(t *testing.T) {
		meta := map[string]string{
			"order_id":  "ord_99812",
			"customer":  "alice@domain.com",
			"tier":      "enterprise",
		}
		if err := security.ValidateMetadataMap(meta); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects too many keys", func(t *testing.T) {
		meta := make(map[string]string)
		for i := 0; i < security.MaxMetadataKeys+1; i++ {
			meta[strings.Repeat("k", i+1)] = "v"
		}
		err := security.ValidateMetadataMap(meta)
		if !errors.Is(err, security.ErrMetadataTooManyKeys) {
			t.Fatalf("expected ErrMetadataTooManyKeys, got %v", err)
		}
	})

	t.Run("rejects oversized value", func(t *testing.T) {
		meta := map[string]string{
			"notes": strings.Repeat("X", security.MaxMetadataValLen+1),
		}
		err := security.ValidateMetadataMap(meta)
		if !errors.Is(err, security.ErrMetadataValTooLong) {
			t.Fatalf("expected ErrMetadataValTooLong, got %v", err)
		}
	})
}
