package judging

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateRequiredUTF8MeasuresEncodedBytes(t *testing.T) {
	t.Parallel()

	exact := strings.Repeat("界", MaximumSourceCodeBytes/3) +
		strings.Repeat("a", MaximumSourceCodeBytes%3)
	if got := len(exact); got != MaximumSourceCodeBytes || !utf8.ValidString(exact) {
		t.Fatalf("boundary fixture = %d bytes, want %d valid UTF-8 bytes", got, MaximumSourceCodeBytes)
	}
	if err := ValidateSourceCode(exact); err != nil {
		t.Fatalf("ValidateSourceCode(exact boundary): %v", err)
	}

	err := ValidateSourceCode(exact + "界")
	var violation *ContentViolation
	if !errors.As(err, &violation) {
		t.Fatalf("ValidateSourceCode(over boundary) = %v, want ContentViolation", err)
	}
	if violation.Kind != ViolationTooLarge || violation.ActualBytes != MaximumSourceCodeBytes+3 ||
		violation.MaximumBytes != MaximumSourceCodeBytes || violation.Field != FieldSourceCode {
		t.Fatalf("unexpected violation: %+v", violation)
	}
}

func TestRequiredJudgeContentRejectsEmptyInvalidUTF8AndNUL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func() error
		kind ViolationKind
	}{
		{name: "empty source", call: func() error { return ValidateSourceCode("") }, kind: ViolationRequired},
		{name: "invalid UTF-8 input", call: func() error {
			return ValidateTestInput(string([]byte{0xff, 0xfe}))
		}, kind: ViolationInvalidUTF8},
		{name: "NUL output", call: func() error { return ValidateExpectedOutput("ok\x00") }, kind: ViolationContainsNUL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var violation *ContentViolation
			if err := test.call(); !errors.As(err, &violation) || violation.Kind != test.kind {
				t.Fatalf("validation = %v (%+v), want kind %q", err, violation, test.kind)
			}
		})
	}
}

func TestEveryJudgeContentLimitHasOneCanonicalPolicy(t *testing.T) {
	t.Parallel()

	if MaximumSourceCodeBytes != 256<<10 {
		t.Fatalf("MaximumSourceCodeBytes = %d", MaximumSourceCodeBytes)
	}
	if MaximumTestInputBytes != 1<<20 || MaximumExpectedOutputBytes != 1<<20 {
		t.Fatalf("test limits = (%d, %d)", MaximumTestInputBytes, MaximumExpectedOutputBytes)
	}
	if err := ValidateRequiredUTF8("", "value", 1); !errors.Is(err, ErrInvalidContentPolicy) {
		t.Fatalf("invalid field policy = %v", err)
	}
	if err := ValidateRequiredUTF8("field", "value", 0); !errors.Is(err, ErrInvalidContentPolicy) {
		t.Fatalf("invalid byte policy = %v", err)
	}
}
