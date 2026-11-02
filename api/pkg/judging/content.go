// Package judging owns the stable, product-wide constraints shared by
// submission intake, problem authoring, immutable judge bundles, and the
// sandbox. These are byte limits, not character limits.
package judging

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	MaximumSourceCodeBytes     = 256 << 10
	MaximumTestInputBytes      = 1 << 20
	MaximumExpectedOutputBytes = 1 << 20
	FieldSourceCode            = "source_code"
	FieldTestInput             = "input"
	FieldExpectedOutput        = "expected_output"
)

var ErrInvalidContentPolicy = errors.New("judging content: invalid validation policy")

type ViolationKind string

const (
	ViolationRequired    ViolationKind = "required"
	ViolationInvalidUTF8 ViolationKind = "invalid_utf8"
	ViolationContainsNUL ViolationKind = "contains_nul"
	ViolationTooLarge    ViolationKind = "too_large"
)

// ContentViolation contains only safe metadata. The submitted content is
// deliberately never retained in or formatted into the error.
type ContentViolation struct {
	Field        string        `json:"field"`
	Kind         ViolationKind `json:"reason"`
	ActualBytes  int           `json:"actual_bytes"`
	MaximumBytes int           `json:"maximum_bytes"`
}

func (violation *ContentViolation) Error() string {
	if violation == nil {
		return "invalid judge content"
	}
	switch violation.Kind {
	case ViolationRequired:
		return fmt.Sprintf("%s is required", violation.Field)
	case ViolationInvalidUTF8:
		return fmt.Sprintf("%s must be valid UTF-8", violation.Field)
	case ViolationContainsNUL:
		return fmt.Sprintf("%s must not contain NUL bytes", violation.Field)
	case ViolationTooLarge:
		return fmt.Sprintf("%s must not exceed %d bytes", violation.Field, violation.MaximumBytes)
	default:
		return fmt.Sprintf("%s is invalid", violation.Field)
	}
}

// ValidateRequiredUTF8 validates the exact string that will be persisted.
// len(string) is the encoded byte count in Go, which is intentionally used
// instead of utf8.RuneCountInString.
func ValidateRequiredUTF8(field, value string, maximumBytes int) error {
	if strings.TrimSpace(field) == "" || maximumBytes <= 0 {
		return ErrInvalidContentPolicy
	}
	actualBytes := len(value)
	if actualBytes == 0 {
		return &ContentViolation{
			Field: field, Kind: ViolationRequired,
			ActualBytes: actualBytes, MaximumBytes: maximumBytes,
		}
	}
	if !utf8.ValidString(value) {
		return &ContentViolation{
			Field: field, Kind: ViolationInvalidUTF8,
			ActualBytes: actualBytes, MaximumBytes: maximumBytes,
		}
	}
	// PostgreSQL text rejects U+0000 even in an otherwise valid UTF-8 string.
	if strings.IndexByte(value, 0) >= 0 {
		return &ContentViolation{
			Field: field, Kind: ViolationContainsNUL,
			ActualBytes: actualBytes, MaximumBytes: maximumBytes,
		}
	}
	if actualBytes > maximumBytes {
		return &ContentViolation{
			Field: field, Kind: ViolationTooLarge,
			ActualBytes: actualBytes, MaximumBytes: maximumBytes,
		}
	}
	return nil
}

func ValidateSourceCode(value string) error {
	return ValidateRequiredUTF8(FieldSourceCode, value, MaximumSourceCodeBytes)
}

func ValidateTestInput(value string) error {
	return ValidateRequiredUTF8(FieldTestInput, value, MaximumTestInputBytes)
}

func ValidateExpectedOutput(value string) error {
	return ValidateRequiredUTF8(FieldExpectedOutput, value, MaximumExpectedOutputBytes)
}
