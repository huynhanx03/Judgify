package domain

import (
	"errors"
	"strings"
	"unicode/utf8"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var ErrInvalidProblemSlug = errors.New("problem: invalid stable slug")

// CanonicalProblemSlug normalizes an explicitly supplied, ASCII-only public
// identity. Titles are intentionally not used: they are mutable and may be
// multilingual, while public URLs must remain stable.
func CanonicalProblemSlug(candidate string) (string, error) {
	if candidate == "" || !utf8.ValidString(candidate) ||
		strings.IndexByte(candidate, 0) >= 0 {
		return "", ErrInvalidProblemSlug
	}

	var builder strings.Builder
	builder.Grow(len(candidate))
	pendingSeparator := false

	for _, character := range candidate {
		var ascii byte
		switch {
		case character >= 'a' && character <= 'z':
			ascii = byte(character)
		case character >= 'A' && character <= 'Z':
			ascii = byte(character + ('a' - 'A'))
		case character >= '0' && character <= '9':
			ascii = byte(character)
		case character == ' ' || character == '\t' ||
			character == '\r' || character == '\n' ||
			character == '_' || character == '-':
			if builder.Len() > 0 {
				pendingSeparator = true
			}
			continue
		default:
			return "", ErrInvalidProblemSlug
		}

		if pendingSeparator && builder.Len() > 0 {
			builder.WriteByte('-')
		}
		pendingSeparator = false
		builder.WriteByte(ascii)
	}
	slug := builder.String()
	if !ValidProblemSlug(slug) {
		return "", ErrInvalidProblemSlug
	}
	return slug, nil
}

func ValidProblemSlug(value string) bool {
	if len(value) < 1 || len(value) > problemcontract.MaximumSlugBytes ||
		!utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 ||
		value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	previousDash := false
	for _, character := range []byte(value) {
		valid := (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '-'
		if !valid || (character == '-' && previousDash) {
			return false
		}
		previousDash = character == '-'
	}
	return true
}
