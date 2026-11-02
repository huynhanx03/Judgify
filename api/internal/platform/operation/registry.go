package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"sync"
)

var (
	ErrInvalidExecutor   = errors.New("operation: invalid executor")
	ErrDuplicateExecutor = errors.New("operation: duplicate executor")
)

// Registry is immutable after Freeze. Startup composition registers the
// complete executor catalog before any worker can claim an operation.
type Registry struct {
	mu        sync.RWMutex
	executors map[string]Executor
	frozen    bool
}

func NewRegistry() *Registry {
	return &Registry{executors: make(map[string]Executor)}
}

func (registry *Registry) Register(executor Executor) error {
	if registry == nil || isNilExecutor(executor) ||
		!validIdentifier(executor.Kind(), MaxKindBytes) {
		return ErrInvalidExecutor
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.frozen {
		return ErrInvalidExecutor
	}
	if _, exists := registry.executors[executor.Kind()]; exists {
		return ErrDuplicateExecutor
	}
	registry.executors[executor.Kind()] = executor
	return nil
}

func isNilExecutor(executor Executor) bool {
	if executor == nil {
		return true
	}
	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (registry *Registry) Freeze() error {
	if registry == nil {
		return ErrInvalidExecutor
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if len(registry.executors) == 0 {
		return ErrInvalidExecutor
	}
	registry.frozen = true
	return nil
}

func (registry *Registry) Lookup(kind string) (Executor, bool) {
	if registry == nil || !validIdentifier(kind, MaxKindBytes) {
		return nil, false
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if !registry.frozen {
		return nil, false
	}
	executor, exists := registry.executors[kind]
	return executor, exists
}

// Kinds returns a sorted copy of the code-owned executor catalog. It is safe
// for low-cardinality operational metrics and never exposes durable payloads.
func (registry *Registry) Kinds() []string {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	kinds := make([]string, 0, len(registry.executors))
	for kind := range registry.executors {
		kinds = append(kinds, kind)
	}
	registry.mu.RUnlock()
	slices.Sort(kinds)
	return kinds
}

// JSONBatchFunc is the type-safe domain boundary used by RegisterJSON. The
// generic adapter strictly decodes the durable request before invoking it.
type JSONBatchFunc[T any] func(context.Context, T, BatchInput) (BatchResult, error)

func RegisterJSON[T any](
	registry *Registry,
	kind string,
	execute JSONBatchFunc[T],
) error {
	if registry == nil || execute == nil || !validIdentifier(kind, MaxKindBytes) {
		return ErrInvalidExecutor
	}
	return registry.Register(&jsonExecutor[T]{kind: kind, execute: execute})
}

type jsonExecutor[T any] struct {
	kind    string
	execute JSONBatchFunc[T]
}

func (executor *jsonExecutor[T]) Kind() string {
	if executor == nil {
		return ""
	}
	return executor.kind
}

func (executor *jsonExecutor[T]) ExecuteBatch(
	ctx context.Context,
	input BatchInput,
) (BatchResult, error) {
	if executor == nil || executor.execute == nil || ctx == nil ||
		ctx.Err() != nil || len(input.Request) == 0 {
		return BatchResult{}, ErrInvalidExecutor
	}
	decoder := json.NewDecoder(bytes.NewReader(input.Request))
	decoder.DisallowUnknownFields()
	var request T
	if err := decoder.Decode(&request); err != nil {
		return BatchResult{}, errors.Join(ErrInvalidRequest, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return BatchResult{}, ErrInvalidRequest
	}
	return executor.execute(ctx, request, input)
}
