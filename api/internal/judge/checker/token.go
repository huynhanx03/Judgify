package checker

import (
	"bytes"
	"context"
	"io"
)

// TokenV1 splits only on the six ASCII whitespace bytes. Unicode whitespace
// remains ordinary token data.
type TokenV1 struct{}

func (TokenV1) Check(
	ctx context.Context,
	expected io.Reader,
	actual io.Reader,
	policy Policy,
) (Result, error) {
	expectedBytes, actualBytes, _, err := readInputs(ctx, expected, actual, policy, false)
	if err != nil {
		return Result{}, err
	}
	expectedTokens := tokenCursor{value: expectedBytes}
	actualTokens := tokenCursor{value: actualBytes}
	for {
		expectedToken, hasExpected := expectedTokens.next()
		actualToken, hasActual := actualTokens.next()
		if hasExpected != hasActual {
			return rejectedResult("token_count_mismatch"), nil
		}
		if !hasExpected {
			return acceptedResult(), nil
		}
		if !bytes.Equal(expectedToken, actualToken) {
			return rejectedResult("token_mismatch"), nil
		}
	}
}

var _ Checker = TokenV1{}
