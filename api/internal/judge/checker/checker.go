// Package checker contains the closed, versioned set of trusted host-side
// output checkers. It never logs or returns contestant/expected output.
package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	KeyExact = "exact"
	KeyToken = "token"
	KeyFloat = "float"

	VersionV1 = "v1"

	defaultMaximumExpectedBytes int64 = 16 << 20
	defaultMaximumActualBytes   int64 = 16 << 20
)

var (
	ErrInvalidRequest        = errors.New("checker: invalid request")
	ErrInvalidPolicy         = errors.New("checker: invalid policy")
	ErrExpectedOutputLimit   = errors.New("checker: expected output exceeds trusted bound")
	ErrActualOutputLimit     = errors.New("checker: actual output exceeds trusted bound")
	ErrInvalidExpectedOutput = errors.New("checker: invalid expected output")
)

type Identity struct {
	Key     string
	Version string
}

type Policy struct {
	MaximumExpectedBytes int64
	MaximumActualBytes   int64
	AbsoluteTolerance    float64
	RelativeTolerance    float64
}

func DefaultPolicy() Policy {
	return Policy{
		MaximumExpectedBytes: defaultMaximumExpectedBytes,
		MaximumActualBytes:   defaultMaximumActualBytes,
	}
}

func (policy Policy) normalized(floatPolicy bool) (Policy, error) {
	if policy.MaximumExpectedBytes == 0 {
		policy.MaximumExpectedBytes = defaultMaximumExpectedBytes
	}
	if policy.MaximumActualBytes == 0 {
		policy.MaximumActualBytes = defaultMaximumActualBytes
	}
	if policy.MaximumExpectedBytes < 1 ||
		policy.MaximumExpectedBytes > defaultMaximumExpectedBytes ||
		policy.MaximumActualBytes < 1 ||
		policy.MaximumActualBytes > defaultMaximumActualBytes {
		return Policy{}, ErrInvalidPolicy
	}
	if !floatPolicy {
		if policy.AbsoluteTolerance != 0 || policy.RelativeTolerance != 0 {
			return Policy{}, ErrInvalidPolicy
		}
		return policy, nil
	}
	if math.IsNaN(policy.AbsoluteTolerance) ||
		math.IsInf(policy.AbsoluteTolerance, 0) ||
		policy.AbsoluteTolerance < 0 ||
		policy.AbsoluteTolerance > 1 ||
		math.IsNaN(policy.RelativeTolerance) ||
		math.IsInf(policy.RelativeTolerance, 0) ||
		policy.RelativeTolerance < 0 ||
		policy.RelativeTolerance > 1 {
		return Policy{}, ErrInvalidPolicy
	}
	return policy, nil
}

type Result struct {
	Accepted   bool
	Code       string
	Diagnostic string
}

func acceptedResult() Result {
	return Result{Accepted: true, Code: "accepted"}
}

func rejectedResult(code string) Result {
	return Result{Accepted: false, Code: code}
}

type Checker interface {
	Check(
		ctx context.Context,
		expected io.Reader,
		actual io.Reader,
		policy Policy,
	) (Result, error)
}

func Resolve(identity Identity) (Checker, bool) {
	switch identity {
	case Identity{Key: KeyExact, Version: VersionV1}:
		return ExactV1{}, true
	case Identity{Key: KeyToken, Version: VersionV1}:
		return TokenV1{}, true
	case Identity{Key: KeyFloat, Version: VersionV1}:
		return FloatV1{}, true
	default:
		return nil, false
	}
}

// Configure resolves the closed checker identity and strictly decodes its
// immutable snapshotted configuration. The returned byte limits are exact
// per-case bounds; no checker can silently widen them.
func Configure(
	identity Identity,
	config json.RawMessage,
	maximumExpectedBytes int64,
	maximumActualBytes int64,
) (Checker, Policy, error) {
	resolved, ok := Resolve(identity)
	if !ok || maximumExpectedBytes < 0 || maximumActualBytes < 1 {
		return nil, Policy{}, ErrInvalidPolicy
	}
	if maximumExpectedBytes == 0 {
		maximumExpectedBytes = 1
	}
	policy := Policy{
		MaximumExpectedBytes: maximumExpectedBytes,
		MaximumActualBytes:   maximumActualBytes,
	}
	switch identity.Key {
	case KeyExact, KeyToken:
		var object map[string]json.RawMessage
		if err := decodeStrictConfig(config, &object); err != nil ||
			object == nil || len(object) != 0 {
			return nil, Policy{}, ErrInvalidPolicy
		}
	case KeyFloat:
		var value struct {
			Absolute *float64 `json:"abs_tol"`
			Relative *float64 `json:"rel_tol"`
		}
		if err := decodeStrictConfig(config, &value); err != nil ||
			value.Absolute == nil || value.Relative == nil {
			return nil, Policy{}, ErrInvalidPolicy
		}
		policy.AbsoluteTolerance = *value.Absolute
		policy.RelativeTolerance = *value.Relative
	default:
		return nil, Policy{}, ErrInvalidPolicy
	}
	normalized, err := policy.normalized(identity.Key == KeyFloat)
	if err != nil {
		return nil, Policy{}, err
	}
	return resolved, normalized, nil
}

func decodeStrictConfig(value json.RawMessage, destination any) error {
	if len(value) == 0 || len(value) > 4<<10 {
		return ErrInvalidPolicy
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrInvalidPolicy
		}
		return err
	}
	return nil
}

func readInputs(
	ctx context.Context,
	expected io.Reader,
	actual io.Reader,
	policy Policy,
	floatPolicy bool,
) ([]byte, []byte, Policy, error) {
	if ctx == nil || expected == nil || actual == nil {
		return nil, nil, Policy{}, ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, Policy{}, err
	}
	normalized, err := policy.normalized(floatPolicy)
	if err != nil {
		return nil, nil, Policy{}, err
	}
	expectedBytes, err := readBounded(expected, normalized.MaximumExpectedBytes)
	if err != nil {
		if errors.Is(err, errReadLimit) {
			return nil, nil, Policy{}, ErrExpectedOutputLimit
		}
		return nil, nil, Policy{}, fmt.Errorf("checker: read expected output: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, Policy{}, err
	}
	actualBytes, err := readBounded(actual, normalized.MaximumActualBytes)
	if err != nil {
		if errors.Is(err, errReadLimit) {
			return nil, nil, Policy{}, ErrActualOutputLimit
		}
		return nil, nil, Policy{}, fmt.Errorf("checker: read actual output: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, Policy{}, err
	}
	return expectedBytes, actualBytes, normalized, nil
}

var errReadLimit = errors.New("checker: bounded reader limit reached")

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	limited := io.LimitReader(reader, maximum+1)
	value, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(value)) > maximum {
		return nil, errReadLimit
	}
	return value, nil
}

func asciiWhitespace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}

type tokenCursor struct {
	value  []byte
	offset int
}

func (cursor *tokenCursor) next() ([]byte, bool) {
	for cursor.offset < len(cursor.value) && asciiWhitespace(cursor.value[cursor.offset]) {
		cursor.offset++
	}
	start := cursor.offset
	for cursor.offset < len(cursor.value) && !asciiWhitespace(cursor.value[cursor.offset]) {
		cursor.offset++
	}
	if start == cursor.offset {
		return nil, false
	}
	return cursor.value[start:cursor.offset], true
}
