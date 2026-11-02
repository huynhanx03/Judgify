// Package authz contains the fail-closed route policy catalog consumed by the
// Envoy Check adapter. It intentionally has no Gin dependency so policy
// matching is deterministic at the proxy boundary.
package authz

import (
	"net/url"
	"strings"
)

type Class string

const (
	Public        Class = "public"
	Authenticated Class = "authenticated"
	Protected     Class = "protected"
)

type RoutePolicy struct {
	Method   string
	Pattern  string
	Class    Class
	Resource string
	Action   string
}

// Resolve returns a policy only when exactly one normalized pattern matches.
// Ambiguity and encoded separators fail closed rather than choosing by order.
func Resolve(policies []RoutePolicy, method, path string) (RoutePolicy, bool) {
	if method == "" || !safePath(path) {
		return RoutePolicy{}, false
	}
	var found RoutePolicy
	bestScore := -1
	ambiguous := false
	for _, policy := range policies {
		if (policy.Method != "*" && policy.Method != method) || !patternMatches(policy.Pattern, path) {
			continue
		}
		score := staticSegments(policy.Pattern)
		if score > bestScore {
			found, bestScore, ambiguous = policy, score, false
		} else if score == bestScore {
			ambiguous = true
		}
	}
	if bestScore < 0 || ambiguous {
		return RoutePolicy{}, false
	}
	return found, true
}

func staticSegments(pattern string) int {
	score := 0
	for _, part := range strings.Split(strings.TrimPrefix(pattern, "/"), "/") {
		if part == "**" {
			continue
		}
		if !strings.HasPrefix(part, ":") {
			score++
		}
	}
	return score
}

func safePath(path string) bool {
	if path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "//") {
		return false
	}
	decoded, err := url.PathUnescape(path)
	return err == nil && decoded == path && !strings.Contains(path, "\\")
}

func patternMatches(pattern, path string) bool {
	left := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	right := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(left) > 0 && left[len(left)-1] == "**" {
		left = left[:len(left)-1]
		if len(right) < len(left) {
			return false
		}
	} else if len(left) != len(right) {
		return false
	}
	for index, part := range left {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			if right[index] == "" {
				return false
			}
			continue
		}
		if part != right[index] {
			return false
		}
	}
	return true
}
