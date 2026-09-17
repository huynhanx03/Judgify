package checker

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestExactV1NormalizationMatrix(t *testing.T) {
	t.Parallel()

	checker := ExactV1{}
	for _, test := range []struct {
		name     string
		expected string
		actual   string
		accepted bool
	}{
		{name: "identical", expected: "alpha\nbeta\n", actual: "alpha\nbeta\n", accepted: true},
		{name: "CRLF pairs become LF", expected: "alpha\r\nbeta\r\n", actual: "alpha\nbeta\n", accepted: true},
		{name: "terminal LF count is canonical", expected: "alpha\n\n\n", actual: "alpha\n", accepted: true},
		{name: "empty and terminal LF canonicalize", expected: "", actual: "\n", accepted: true},
		{name: "trailing space remains significant", expected: "alpha \n", actual: "alpha\n", accepted: false},
		{name: "trailing tab remains significant", expected: "alpha\t\n", actual: "alpha\n", accepted: false},
		{name: "bare CR remains significant", expected: "alpha\rbeta\n", actual: "alpha\nbeta\n", accepted: false},
		{name: "internal blank line remains significant", expected: "alpha\n\nbeta\n", actual: "alpha\nbeta\n", accepted: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := checker.Check(
				context.Background(),
				strings.NewReader(test.expected),
				strings.NewReader(test.actual),
				DefaultPolicy(),
			)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if result.Accepted != test.accepted {
				t.Fatalf("Check() = %+v, want accepted=%v", result, test.accepted)
			}
		})
	}
}

func TestTokenV1UsesOnlyASCIIWhitespace(t *testing.T) {
	t.Parallel()

	checker := TokenV1{}
	for _, test := range []struct {
		name     string
		expected string
		actual   string
		accepted bool
	}{
		{name: "ASCII delimiters collapse", expected: "a b\tc\r\nd\fe\v", actual: "\ta\nb c d e ", accepted: true},
		{name: "leading and trailing delimiters ignored", expected: "  a b\n", actual: "a b", accepted: true},
		{name: "token count differs", expected: "a b", actual: "a", accepted: false},
		{name: "unicode nonbreaking space is token data", expected: "a\u00a0b", actual: "a b", accepted: false},
		{name: "token bytes differ", expected: "alpha beta", actual: "alpha Beta", accepted: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := checker.Check(
				context.Background(),
				strings.NewReader(test.expected),
				strings.NewReader(test.actual),
				DefaultPolicy(),
			)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if result.Accepted != test.accepted {
				t.Fatalf("Check() = %+v, want accepted=%v", result, test.accepted)
			}
		})
	}
}

func TestFloatV1NumericAndLiteralMatrix(t *testing.T) {
	t.Parallel()

	checker := FloatV1{}
	policy := DefaultPolicy()
	policy.AbsoluteTolerance = 1e-6
	policy.RelativeTolerance = 1e-5

	for _, test := range []struct {
		name     string
		expected string
		actual   string
		accepted bool
	}{
		{name: "absolute tolerance", expected: "0", actual: "0.0000009", accepted: true},
		{name: "relative tolerance", expected: "1000000", actual: "1000009", accepted: true},
		{name: "outside both tolerances", expected: "1", actual: "1.001", accepted: false},
		{name: "signed decimal grammar", expected: "+.5 -1. 2e3", actual: "0.5000001 -1.0 2000", accepted: true},
		{name: "nonnumeric expected is literal", expected: "Case#1", actual: "Case#1", accepted: true},
		{name: "nonnumeric literal mismatch", expected: "Case#1", actual: "case#1", accepted: false},
		{name: "numeric expected requires numeric actual", expected: "1", actual: "one", accepted: false},
		{name: "actual NaN never passes", expected: "1", actual: "NaN", accepted: false},
		{name: "actual infinity never passes", expected: "1", actual: "+Infinity", accepted: false},
		{name: "actual overflow never passes", expected: "1", actual: "1e9999", accepted: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := checker.Check(
				context.Background(),
				strings.NewReader(test.expected),
				strings.NewReader(test.actual),
				policy,
			)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if result.Accepted != test.accepted {
				t.Fatalf("Check() = %+v, want accepted=%v", result, test.accepted)
			}
		})
	}
}

func TestFloatV1RejectsInvalidExpectedNumericCorpus(t *testing.T) {
	t.Parallel()

	checker := FloatV1{}
	for _, expected := range []string{
		"NaN", "-nan", "Inf", "+infinity", "-INF", "1e9999",
	} {
		_, err := checker.Check(
			context.Background(),
			strings.NewReader(expected),
			strings.NewReader("0"),
			DefaultPolicy(),
		)
		if !errors.Is(err, ErrInvalidExpectedOutput) {
			t.Fatalf("expected token %q error = %v, want ErrInvalidExpectedOutput", expected, err)
		}
	}
}

func TestCheckerPoliciesAndInputsAreBounded(t *testing.T) {
	t.Parallel()

	policy := DefaultPolicy()
	policy.MaximumExpectedBytes = 4
	policy.MaximumActualBytes = 4

	if _, err := (ExactV1{}).Check(
		context.Background(),
		strings.NewReader("12345"),
		strings.NewReader("1234"),
		policy,
	); !errors.Is(err, ErrExpectedOutputLimit) {
		t.Fatalf("oversized expected error = %v, want ErrExpectedOutputLimit", err)
	}
	if _, err := (ExactV1{}).Check(
		context.Background(),
		strings.NewReader("1234"),
		strings.NewReader("12345"),
		policy,
	); !errors.Is(err, ErrActualOutputLimit) {
		t.Fatalf("oversized actual error = %v, want ErrActualOutputLimit", err)
	}

	policy = DefaultPolicy()
	policy.AbsoluteTolerance = math.NaN()
	if _, err := (FloatV1{}).Check(
		context.Background(),
		strings.NewReader("1"),
		strings.NewReader("1"),
		policy,
	); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("NaN tolerance error = %v, want ErrInvalidPolicy", err)
	}
}

func TestRegistryIsClosedAndVersioned(t *testing.T) {
	t.Parallel()

	for _, identity := range []Identity{
		{Key: KeyExact, Version: VersionV1},
		{Key: KeyToken, Version: VersionV1},
		{Key: KeyFloat, Version: VersionV1},
	} {
		if _, ok := Resolve(identity); !ok {
			t.Fatalf("Resolve(%+v) not found", identity)
		}
	}
	for _, identity := range []Identity{
		{Key: "custom", Version: VersionV1},
		{Key: KeyExact, Version: "v2"},
		{},
	} {
		if _, ok := Resolve(identity); ok {
			t.Fatalf("Resolve(%+v) unexpectedly accepted", identity)
		}
	}
}

func TestConfigureBindsStrictImmutableCheckerPolicy(t *testing.T) {
	t.Parallel()

	resolved, policy, err := Configure(
		Identity{Key: KeyFloat, Version: VersionV1},
		json.RawMessage(`{"abs_tol":0.000001,"rel_tol":0.00001}`),
		17,
		23,
	)
	if err != nil || resolved == nil {
		t.Fatalf("Configure() = %#v, %#v, %v", resolved, policy, err)
	}
	if policy.MaximumExpectedBytes != 17 ||
		policy.MaximumActualBytes != 23 ||
		policy.AbsoluteTolerance != 0.000001 ||
		policy.RelativeTolerance != 0.00001 {
		t.Fatalf("configured policy = %#v, want exact snapshotted bounds", policy)
	}
	result, err := resolved.Check(
		context.Background(),
		strings.NewReader("1000000"),
		strings.NewReader("1000009"),
		policy,
	)
	if err != nil || !result.Accepted {
		t.Fatalf("configured checker result = %#v, %v, want accepted", result, err)
	}
}

func TestConfigureRejectsUnknownMissingAndTrailingConfiguration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		identity Identity
		config   json.RawMessage
	}{
		{
			name:     "exact unknown field",
			identity: Identity{Key: KeyExact, Version: VersionV1},
			config:   json.RawMessage(`{"trim":true}`),
		},
		{
			name:     "float missing relative tolerance",
			identity: Identity{Key: KeyFloat, Version: VersionV1},
			config:   json.RawMessage(`{"abs_tol":0.001}`),
		},
		{
			name:     "trailing JSON value",
			identity: Identity{Key: KeyToken, Version: VersionV1},
			config:   json.RawMessage(`{} {}`),
		},
		{
			name:     "unregistered version",
			identity: Identity{Key: KeyExact, Version: "v2"},
			config:   json.RawMessage(`{}`),
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := Configure(test.identity, test.config, 16, 16); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("Configure() error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func FuzzExactV1Deterministic(f *testing.F) {
	f.Add([]byte("alpha\r\nbeta\n"), []byte("alpha\nbeta\n"))
	f.Add([]byte{}, []byte("\n"))
	f.Fuzz(func(t *testing.T, expected []byte, actual []byte) {
		policy := DefaultPolicy()
		first, firstErr := (ExactV1{}).Check(
			context.Background(),
			strings.NewReader(string(expected)),
			strings.NewReader(string(actual)),
			policy,
		)
		second, secondErr := (ExactV1{}).Check(
			context.Background(),
			strings.NewReader(string(expected)),
			strings.NewReader(string(actual)),
			policy,
		)
		assertDeterministicResult(t, first, firstErr, second, secondErr)
	})
}

func FuzzTokenV1Deterministic(f *testing.F) {
	f.Add([]byte("a b\tc"), []byte("a\nb c"))
	f.Add([]byte("a\u00a0b"), []byte("a b"))
	f.Fuzz(func(t *testing.T, expected []byte, actual []byte) {
		policy := DefaultPolicy()
		first, firstErr := (TokenV1{}).Check(
			context.Background(),
			strings.NewReader(string(expected)),
			strings.NewReader(string(actual)),
			policy,
		)
		second, secondErr := (TokenV1{}).Check(
			context.Background(),
			strings.NewReader(string(expected)),
			strings.NewReader(string(actual)),
			policy,
		)
		assertDeterministicResult(t, first, firstErr, second, secondErr)
	})
}

func FuzzFloatV1Deterministic(f *testing.F) {
	f.Add([]byte("1e3 token"), []byte("1000 token"))
	f.Add([]byte("NaN"), []byte("0"))
	f.Fuzz(func(t *testing.T, expected []byte, actual []byte) {
		policy := DefaultPolicy()
		policy.AbsoluteTolerance = 1e-9
		policy.RelativeTolerance = 1e-9
		first, firstErr := (FloatV1{}).Check(
			context.Background(),
			strings.NewReader(string(expected)),
			strings.NewReader(string(actual)),
			policy,
		)
		second, secondErr := (FloatV1{}).Check(
			context.Background(),
			strings.NewReader(string(expected)),
			strings.NewReader(string(actual)),
			policy,
		)
		assertDeterministicResult(t, first, firstErr, second, secondErr)
	})
}

func assertDeterministicResult(
	t *testing.T,
	first Result,
	firstErr error,
	second Result,
	secondErr error,
) {
	t.Helper()
	if first != second || errorText(firstErr) != errorText(secondErr) {
		t.Fatalf(
			"checker is nondeterministic: first=(%+v,%v) second=(%+v,%v)",
			first,
			firstErr,
			second,
			secondErr,
		)
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
