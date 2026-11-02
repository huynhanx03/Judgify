package contract

import (
	"net/url"
	"path"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

var staticActionPaths = map[string]struct{}{
	"/":                  {},
	"/notifications":     {},
	"/profile":           {},
	"/profile/security":  {},
	"/admin":             {},
	"/admin/audit":       {},
	"/admin/judge":       {},
	"/admin/submissions": {},
}

var uuidActionRoots = map[string]struct{}{
	"contests":          {},
	"materials":         {},
	"problems":          {},
	"submissions":       {},
	"admin/submissions": {},
	"admin/users":       {},
}

// ValidateActionPath accepts only a closed family of same-origin product
// routes. It rejects query strings and fragments so notification content can
// never become an open redirect or a credential-bearing URL.
func ValidateActionPath(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > MaximumActionPathBytes || !strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "//") || strings.Contains(value, "\\") {
		return ErrInvalidNotification
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return ErrInvalidNotification
		}
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" ||
		parsed.Fragment != "" || parsed.Path != value || path.Clean(value) != value {
		return ErrInvalidNotification
	}
	if _, allowed := staticActionPaths[value]; allowed {
		return nil
	}
	segments := strings.Split(strings.TrimPrefix(value, "/"), "/")
	if len(segments) == 2 {
		return validateUUIDActionRoot(segments[0], segments[1])
	}
	if len(segments) == 3 {
		return validateUUIDActionRoot(segments[0]+"/"+segments[1], segments[2])
	}
	return ErrInvalidNotification
}

func validateUUIDActionRoot(root, identifier string) error {
	if _, allowed := uuidActionRoots[root]; !allowed {
		return ErrInvalidNotification
	}
	parsed, err := uuid.Parse(identifier)
	if err != nil || parsed == uuid.Nil || parsed.String() != identifier {
		return ErrInvalidNotification
	}
	return nil
}
