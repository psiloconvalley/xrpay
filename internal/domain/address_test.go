package domain_test

import (
	"errors"
	"testing"

	"github.com/psiloconvalley/xrpay/internal/domain"
)

func TestValidateXRPLAddress(t *testing.T) {
	testCases := []struct {
		name        string
		address     string
		expectError error
	}{
		{
			name:        "valid testnet faucet address",
			address:     "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V",
			expectError: nil,
		},
		{
			name:        "valid genesis ripple address",
			address:     "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
			expectError: nil,
		},
		{
			name:        "invalid checksum (one character altered)",
			address:     "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q92W",
			expectError: domain.ErrAddressInvalidChecksum,
		},
		{
			name:        "invalid prefix (starts with 1 instead of r)",
			address:     "1wD1bRFNqjyxPqcSkje5UuBYttqLf7Q92V",
			expectError: domain.ErrAddressInvalidPrefix,
		},
		{
			name:        "too short",
			address:     "rShort",
			expectError: domain.ErrAddressInvalidLength,
		},
		{
			name:        "illegal characters",
			address:     "rwD1bRFNqjyxPqcSkje5UuBYttqLf7Q9IO", // I and O are illegal in Base58
			expectError: domain.ErrAddressInvalidAlphabet,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateXRPLAddress(tc.address)
			if tc.expectError == nil && err != nil {
				t.Fatalf("expected valid address, got error: %v", err)
			}
			if tc.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.expectError)
				}
				if !errors.Is(err, tc.expectError) {
					t.Fatalf("expected error %v, got %v", tc.expectError, err)
				}
			}
		})
	}
}
