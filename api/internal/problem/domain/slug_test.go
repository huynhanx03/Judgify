package domain

import (
	"strings"
	"testing"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func TestCanonicalProblemSlugNormalizesOnlyExplicitASCIISeparators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		candidate string
		want      string
	}{
		{
			name:      "space and underscore",
			candidate: "  Two__SUM  ",
			want:      "two-sum",
		},
		{
			name:      "repeated hyphen",
			candidate: "two---sum",
			want:      "two-sum",
		},
		{
			name:      "already canonical",
			candidate: "graph-shortest-path",
			want:      "graph-shortest-path",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := CanonicalProblemSlug(test.candidate)
			if err != nil {
				t.Fatalf("CanonicalProblemSlug() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("CanonicalProblemSlug() = %q, want %q", got, test.want)
			}
			if !ValidProblemSlug(got) {
				t.Fatalf("canonical slug %q is not valid", got)
			}
		})
	}
}

func TestCanonicalProblemSlugRejectsInvalidOrAmbiguousCandidates(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"---",
		"C++",
		"café",
		"两数之和",
		"two\x00sum",
		string([]byte{0xff, 'a'}),
		strings.Repeat("a", problemcontract.MaximumSlugBytes+1),
	}
	for _, candidate := range tests {
		candidate := candidate
		t.Run(candidate, func(t *testing.T) {
			t.Parallel()
			if _, err := CanonicalProblemSlug(candidate); err == nil {
				t.Fatalf("CanonicalProblemSlug(%q) error = nil, want invalid slug", candidate)
			}
		})
	}
}

func TestCanonicalProblemSlugAcceptsExactMaximumBytes(t *testing.T) {
	t.Parallel()

	candidate := strings.Repeat("a", problemcontract.MaximumSlugBytes)
	got, err := CanonicalProblemSlug(candidate)
	if err != nil {
		t.Fatalf("CanonicalProblemSlug() error = %v", err)
	}
	if got != candidate {
		t.Fatalf("CanonicalProblemSlug() = %q, want exact boundary candidate", got)
	}
}
