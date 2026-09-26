package domain_test

import (
	"testing"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

func TestParseXRP_Valid(t *testing.T) {
	tests := []struct {
		input       string
		wantDrops   int64
		wantXRPStr  string
		wantDisplay string
	}{
		{input: "1", wantDrops: 1_000_000, wantXRPStr: "1", wantDisplay: "1 XRP"},
		{input: "0.1", wantDrops: 100_000, wantXRPStr: "0.1", wantDisplay: "0.1 XRP"},
		{input: "0.000001", wantDrops: 1, wantXRPStr: "0.000001", wantDisplay: "0.000001 XRP"},
		{input: "25.50", wantDrops: 25_500_000, wantXRPStr: "25.5", wantDisplay: "25.5 XRP"},
		{input: "100.123456", wantDrops: 100_123_456, wantXRPStr: "100.123456", wantDisplay: "100.123456 XRP"},
		{input: "  50  ", wantDrops: 50_000_000, wantXRPStr: "50", wantDisplay: "50 XRP"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := domain.ParseXRP(tt.input)
			if err != nil {
				t.Fatalf("ParseXRP(%q) unexpected error: %v", tt.input, err)
			}
			if got.Int64() != tt.wantDrops {
				t.Errorf("ParseXRP(%q) = %d drops, want %d", tt.input, got.Int64(), tt.wantDrops)
			}
			if got.ToXRPString() != tt.wantXRPStr {
				t.Errorf("ToXRPString() = %q, want %q", got.ToXRPString(), tt.wantXRPStr)
			}
			if got.String() != tt.wantDisplay {
				t.Errorf("String() = %q, want %q", got.String(), tt.wantDisplay)
			}
		})
	}
}

func TestParseXRP_Invalid(t *testing.T) {
	invalidInputs := []string{
		"",
		"-5",
		"0",
		"0.0",
		"0.0000001",
		"abc",
		"12.34.56",
		"10.abc",
	}

	for _, in := range invalidInputs {
		t.Run(in, func(t *testing.T) {
			_, err := domain.ParseXRP(in)
			if err == nil {
				t.Errorf("ParseXRP(%q) expected error, got nil", in)
			}
		})
	}
}
