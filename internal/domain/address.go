package domain

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// XRPLBase58Alphabet is the Ripple-specific Base58 dictionary.
const XRPLBase58Alphabet = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"

var (
	ErrAddressInvalidLength   = errors.New("address length must be between 25 and 35 characters")
	ErrAddressInvalidPrefix   = errors.New("xrpl address must begin with 'r'")
	ErrAddressInvalidAlphabet = errors.New("address contains invalid base58 characters")
	ErrAddressInvalidChecksum = errors.New("xrpl address cryptographic checksum mismatch")
)

// ValidateXRPLAddress validates an XRPL classic address using length, prefix,
// Ripple-Base58 alphabet decoding, and 4-byte double-SHA256 checksum verification.
func ValidateXRPLAddress(address string) error {
	address = strings.TrimSpace(address)

	if len(address) < 25 || len(address) > 35 {
		return ErrAddressInvalidLength
	}
	if address[0] != 'r' {
		return ErrAddressInvalidPrefix
	}

	// Decode Ripple-Base58
	decoded, err := decodeBase58(address, XRPLBase58Alphabet)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAddressInvalidAlphabet, err)
	}

	// An XRPL decoded address payload must be exactly 25 bytes:
	// 1 byte (type prefix: 0x00) + 20 bytes (AccountID hash) + 4 bytes (Checksum)
	if len(decoded) != 25 {
		return fmt.Errorf("%w: decoded payload is %d bytes, expected 25", ErrAddressInvalidLength, len(decoded))
	}

	payload := decoded[:21]
	expectedChecksum := decoded[21:]

	// Verify double SHA-256 checksum
	firstHash := sha256.Sum256(payload)
	secondHash := sha256.Sum256(firstHash[:])
	actualChecksum := secondHash[:4]

	for i := 0; i < 4; i++ {
		if actualChecksum[i] != expectedChecksum[i] {
			return ErrAddressInvalidChecksum
		}
	}

	return nil
}

func decodeBase58(input, alphabet string) ([]byte, error) {
	radix := big.NewInt(58)
	result := big.NewInt(0)

	for i := 0; i < len(input); i++ {
		char := input[i]
		idx := strings.IndexByte(alphabet, char)
		if idx == -1 {
			return nil, fmt.Errorf("illegal character '%c'", char)
		}
		result.Mul(result, radix)
		result.Add(result, big.NewInt(int64(idx)))
	}

	decoded := result.Bytes()

	// Handle leading zeros in Base58
	numLeadingZeros := 0
	for i := 0; i < len(input) && input[i] == alphabet[0]; i++ {
		numLeadingZeros++
	}

	if numLeadingZeros > 0 {
		prefix := make([]byte, numLeadingZeros)
		decoded = append(prefix, decoded...)
	}

	// In XRPL, address starts with 'r' (0x00 type prefix), so total length should be 25.
	// If big.Int stripped leading zero byte, re-pad it.
	if len(decoded) < 25 {
		pad := make([]byte, 25-len(decoded))
		decoded = append(pad, decoded...)
	}

	return decoded, nil
}
