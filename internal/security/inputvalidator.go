package security

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInputTooLong         = errors.New("input exceeds maximum permitted character length")
	ErrInputInvalidUTF8     = errors.New("input contains invalid UTF-8 byte sequences")
	ErrInputControlChars    = errors.New("input contains prohibited control characters")
	ErrMetadataTooManyKeys  = errors.New("metadata exceeds maximum key count limit")
	ErrMetadataKeyTooLong   = errors.New("metadata key exceeds maximum allowed length")
	ErrMetadataValTooLong   = errors.New("metadata value exceeds maximum allowed length")
	ErrMemoContainsHTMLTags = errors.New("memo contains prohibited HTML tags or angle brackets")
)

const (
	MaxMemoLength      = 200
	MaxMetadataKeys    = 20
	MaxMetadataKeyLen  = 64
	MaxMetadataValLen  = 500
)

// ValidateSafeString ensures a string is valid UTF-8, within length boundaries, and free of dangerous control characters.
func ValidateSafeString(s string, minLen, maxLen int) error {
	if !utf8.ValidString(s) {
		return ErrInputInvalidUTF8
	}

	rCount := utf8.RuneCountInString(s)
	if rCount < minLen {
		return fmt.Errorf("input length %d is below minimum %d", rCount, minLen)
	}
	if rCount > maxLen {
		return fmt.Errorf("%w: length %d exceeds max %d", ErrInputTooLong, rCount, maxLen)
	}

	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return ErrInputControlChars
		}
	}

	return nil
}

// ValidateMemoField validates user-visible invoice memos (max 200 chars, no HTML angle brackets).
func ValidateMemoField(memo string) error {
	trimmed := strings.TrimSpace(memo)
	if trimmed == "" {
		return nil
	}

	if err := ValidateSafeString(trimmed, 1, MaxMemoLength); err != nil {
		return err
	}

	if strings.ContainsAny(trimmed, "<>") {
		return ErrMemoContainsHTMLTags
	}

	return nil
}

// ValidateMetadataMap enforces structural size and character set constraints on invoice metadata.
func ValidateMetadataMap(metadata map[string]string) error {
	if len(metadata) == 0 {
		return nil
	}

	if len(metadata) > MaxMetadataKeys {
		return fmt.Errorf("%w: received %d, max is %d", ErrMetadataTooManyKeys, len(metadata), MaxMetadataKeys)
	}

	for k, v := range metadata {
		if strings.TrimSpace(k) == "" {
			return errors.New("metadata key cannot be empty or whitespace")
		}

		if err := ValidateSafeString(k, 1, MaxMetadataKeyLen); err != nil {
			return fmt.Errorf("%w for key '%s'", ErrMetadataKeyTooLong, k)
		}

		if err := ValidateSafeString(v, 0, MaxMetadataValLen); err != nil {
			return fmt.Errorf("%w for key '%s'", ErrMetadataValTooLong, k)
		}
	}

	return nil
}
