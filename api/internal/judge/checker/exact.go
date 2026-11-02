package checker

import (
	"bytes"
	"context"
	"io"
)

// ExactV1 converts CRLF pairs to LF, removes terminal LF bytes, appends one
// terminal LF, and compares bytes. Bare CR, spaces, tabs, Unicode, and internal
// blank lines remain untouched.
type ExactV1 struct{}

func (ExactV1) Check(
	ctx context.Context,
	expected io.Reader,
	actual io.Reader,
	policy Policy,
) (Result, error) {
	expectedBytes, actualBytes, _, err := readInputs(ctx, expected, actual, policy, false)
	if err != nil {
		return Result{}, err
	}
	if bytes.Equal(normalizeExactV1(expectedBytes), normalizeExactV1(actualBytes)) {
		return acceptedResult(), nil
	}
	return rejectedResult("exact_mismatch"), nil
}

func normalizeExactV1(value []byte) []byte {
	normalized := make([]byte, 0, len(value)+1)
	for index := 0; index < len(value); index++ {
		if value[index] == '\r' && index+1 < len(value) && value[index+1] == '\n' {
			normalized = append(normalized, '\n')
			index++
			continue
		}
		normalized = append(normalized, value[index])
	}
	normalized = bytes.TrimRight(normalized, "\n")
	return append(normalized, '\n')
}

var _ Checker = ExactV1{}
