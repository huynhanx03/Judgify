// Package boundedjson canonicalizes untrusted JSON-shaped values under hard
// depth, node, and byte limits. It is internal to Judgify platform adapters.
package boundedjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalid      = errors.New("bounded json: invalid value")
	ErrTooLarge     = errors.New("bounded json: too large")
	ErrTooDeep      = errors.New("bounded json: too deep")
	ErrTooComplex   = errors.New("bounded json: too complex")
	ErrDuplicateKey = errors.New("bounded json: duplicate object key")
)

// Limits bounds both encoded output and work performed before encoding.
type Limits struct {
	MaxBytes int
	MaxDepth int
	MaxNodes int
}

// MarshalMap returns deterministic JSON object bytes. redact is evaluated for
// every nested object key before its value is inspected, so a rejected secret
// cannot leak through an error or consume the remaining traversal budget.
func MarshalMap(value map[string]any, limits Limits, redact func(string) bool) ([]byte, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	if value == nil {
		value = map[string]any{}
	}
	walker := valueWalker{
		limits: limits,
		redact: redact,
		active: make(map[visit]struct{}),
	}
	normalized, err := walker.walk(reflect.ValueOf(value), 1)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("%w: encode", ErrInvalid)
	}
	if len(encoded) > limits.MaxBytes {
		return nil, ErrTooLarge
	}
	return encoded, nil
}

// CanonicalObject parses exactly one JSON object, rejects duplicate keys, and
// returns deterministic bytes with lexicographically ordered object keys.
func CanonicalObject(raw json.RawMessage, limits Limits) ([]byte, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, ErrInvalid
	}
	if len(raw) > limits.MaxBytes {
		return nil, ErrTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	state := decodeState{limits: limits}
	value, err := state.decode(decoder, 1)
	if err != nil {
		return nil, err
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, ErrInvalid
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return nil, ErrInvalid
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: encode", ErrInvalid)
	}
	if len(encoded) > limits.MaxBytes {
		return nil, ErrTooLarge
	}
	return encoded, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxBytes <= 0 || limits.MaxDepth <= 0 || limits.MaxNodes <= 0 {
		return ErrInvalid
	}
	return nil
}

type visit struct {
	kind reflect.Kind
	ptr  uintptr
}

type valueWalker struct {
	limits Limits
	redact func(string) bool
	nodes  int
	bytes  int
	active map[visit]struct{}
}

func (walker *valueWalker) walk(value reflect.Value, depth int) (any, error) {
	if depth > walker.limits.MaxDepth {
		return nil, ErrTooDeep
	}
	if err := walker.addNode(); err != nil {
		return nil, err
	}
	if !value.IsValid() {
		return nil, nil
	}
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		switch typed := value.Interface().(type) {
		case json.Number:
			if _, err := json.Marshal(typed); err != nil {
				return nil, ErrInvalid
			}
			return typed, nil
		case time.Time:
			return typed.UTC().Format(time.RFC3339Nano), nil
		case uuid.UUID:
			return typed.String(), nil
		case json.RawMessage:
			canonical, err := CanonicalObject(typed, walker.limits)
			if err != nil {
				return nil, err
			}
			var decoded map[string]any
			decoder := json.NewDecoder(bytes.NewReader(canonical))
			decoder.UseNumber()
			if err := decoder.Decode(&decoded); err != nil {
				return nil, ErrInvalid
			}
			return walker.walk(reflect.ValueOf(decoded), depth)
		}
	}

	switch value.Kind() {
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.String:
		text := value.String()
		if !utf8.ValidString(text) {
			return nil, ErrInvalid
		}
		if err := walker.addBytes(len(text)); err != nil {
			return nil, err
		}
		return text, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Interface(), nil
	case reflect.Float32, reflect.Float64:
		number := value.Float()
		if math.IsInf(number, 0) || math.IsNaN(number) {
			return nil, ErrInvalid
		}
		return value.Interface(), nil
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return nil, ErrInvalid
		}
		if err := walker.preflightContainer(value.Len()); err != nil {
			return nil, err
		}
		if err := walker.enter(value); err != nil {
			return nil, err
		}
		defer walker.leave(value)
		result := make(map[string]any, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			key := iterator.Key().String()
			if !utf8.ValidString(key) {
				return nil, ErrInvalid
			}
			if err := walker.addNode(); err != nil {
				return nil, err
			}
			if err := walker.addBytes(len(key)); err != nil {
				return nil, err
			}
			if walker.redact != nil && walker.redact(key) {
				result[key] = "[REDACTED]"
				continue
			}
			nested, err := walker.walk(iterator.Value(), depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = nested
		}
		return result, nil
	case reflect.Slice, reflect.Array:
		if err := walker.preflightContainer(value.Len()); err != nil {
			return nil, err
		}
		if value.Kind() == reflect.Slice {
			if value.IsNil() {
				return nil, nil
			}
			if err := walker.enter(value); err != nil {
				return nil, err
			}
			defer walker.leave(value)
		}
		result := make([]any, value.Len())
		for index := range value.Len() {
			nested, err := walker.walk(value.Index(index), depth+1)
			if err != nil {
				return nil, err
			}
			result[index] = nested
		}
		return result, nil
	case reflect.Invalid:
		return nil, nil
	default:
		return nil, ErrInvalid
	}
}

func (walker *valueWalker) addNode() error {
	walker.nodes++
	if walker.nodes > walker.limits.MaxNodes {
		return ErrTooComplex
	}
	return nil
}

func (walker *valueWalker) preflightContainer(length int) error {
	if length < 0 || length > walker.limits.MaxNodes-walker.nodes {
		return ErrTooComplex
	}
	return nil
}

func (walker *valueWalker) addBytes(count int) error {
	if count < 0 || walker.bytes > walker.limits.MaxBytes-count {
		return ErrTooLarge
	}
	walker.bytes += count
	return nil
}

func (walker *valueWalker) enter(value reflect.Value) error {
	pointer := value.Pointer()
	if pointer == 0 {
		return nil
	}
	key := visit{kind: value.Kind(), ptr: pointer}
	if _, exists := walker.active[key]; exists {
		return ErrInvalid
	}
	walker.active[key] = struct{}{}
	return nil
}

func (walker *valueWalker) leave(value reflect.Value) {
	pointer := value.Pointer()
	if pointer != 0 {
		delete(walker.active, visit{kind: value.Kind(), ptr: pointer})
	}
}

type decodeState struct {
	limits Limits
	nodes  int
}

func (state *decodeState) decode(decoder *json.Decoder, depth int) (any, error) {
	if depth > state.limits.MaxDepth {
		return nil, ErrTooDeep
	}
	state.nodes++
	if state.nodes > state.limits.MaxNodes {
		return nil, ErrTooComplex
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, ErrInvalid
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		switch scalar := token.(type) {
		case nil, bool, json.Number:
			return scalar, nil
		case string:
			if !utf8.ValidString(scalar) {
				return nil, ErrInvalid
			}
			return scalar, nil
		default:
			return nil, ErrInvalid
		}
	}

	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return nil, ErrInvalid
			}
			key, ok := keyToken.(string)
			if !ok || !utf8.ValidString(key) {
				return nil, ErrInvalid
			}
			if _, exists := object[key]; exists {
				return nil, ErrDuplicateKey
			}
			value, valueErr := state.decode(decoder, depth+1)
			if valueErr != nil {
				return nil, valueErr
			}
			object[key] = value
		}
		closing, closeErr := decoder.Token()
		if closeErr != nil || closing != json.Delim('}') {
			return nil, ErrInvalid
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, valueErr := state.decode(decoder, depth+1)
			if valueErr != nil {
				return nil, valueErr
			}
			array = append(array, value)
		}
		closing, closeErr := decoder.Token()
		if closeErr != nil || closing != json.Delim(']') {
			return nil, ErrInvalid
		}
		return array, nil
	default:
		return nil, ErrInvalid
	}
}
