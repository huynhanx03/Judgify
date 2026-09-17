package checker

import (
	"bytes"
	"context"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var finiteDecimalV1 = regexp.MustCompile(
	`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`,
)

// FloatV1 applies the versioned finite-decimal grammar and the inclusive
// absolute-or-relative tolerance rule. Nonnumeric expected tokens remain
// byte-exact literals.
type FloatV1 struct{}

func (FloatV1) Check(
	ctx context.Context,
	expected io.Reader,
	actual io.Reader,
	policy Policy,
) (Result, error) {
	expectedBytes, actualBytes, normalized, err := readInputs(ctx, expected, actual, policy, true)
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
		accepted, invalidExpected := compareFloatTokenV1(
			expectedToken,
			actualToken,
			normalized.AbsoluteTolerance,
			normalized.RelativeTolerance,
		)
		if invalidExpected {
			return Result{}, ErrInvalidExpectedOutput
		}
		if !accepted {
			return rejectedResult("float_mismatch"), nil
		}
	}
}

func compareFloatTokenV1(
	expected []byte,
	actual []byte,
	absoluteTolerance float64,
	relativeTolerance float64,
) (accepted bool, invalidExpected bool) {
	if reservedNonFinite(expected) {
		return false, true
	}
	if !finiteDecimalV1.Match(expected) {
		return bytes.Equal(expected, actual), false
	}
	expectedNumber, err := strconv.ParseFloat(string(expected), 64)
	if err != nil || math.IsNaN(expectedNumber) || math.IsInf(expectedNumber, 0) {
		return false, true
	}
	if reservedNonFinite(actual) || !finiteDecimalV1.Match(actual) {
		return false, false
	}
	actualNumber, err := strconv.ParseFloat(string(actual), 64)
	if err != nil || math.IsNaN(actualNumber) || math.IsInf(actualNumber, 0) {
		return false, false
	}
	difference := math.Abs(actualNumber - expectedNumber)
	if difference <= absoluteTolerance {
		return true, false
	}
	scale := math.Max(math.Abs(expectedNumber), math.Abs(actualNumber))
	return difference <= relativeTolerance*scale, false
}

func reservedNonFinite(token []byte) bool {
	value := strings.ToLower(string(token))
	if strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		value = value[1:]
	}
	switch value {
	case "nan", "inf", "infinity":
		return true
	default:
		return false
	}
}

var _ Checker = FloatV1{}
