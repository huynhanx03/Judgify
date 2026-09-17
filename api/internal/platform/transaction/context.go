package transaction

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/huynhanx03/judgify/internal/ent/generate"
)

type scopeKey struct{}

type scope struct {
	client    *generate.Client
	valid     atomic.Bool
	mu        sync.Mutex
	callbacks []AfterCompletionFunc
}

func newScope(client *generate.Client) *scope {
	value := &scope{client: client}
	value.valid.Store(true)
	return value
}

func bind(ctx context.Context, value *scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, value)
}

func scopeFrom(ctx context.Context) (*scope, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(scopeKey{}).(*scope)
	return value, ok && value != nil
}

// AfterCommit registers callback on the active outermost transaction. FIFO
// order is preserved. Registration on an ordinary or already-closed context
// fails closed rather than running work at the wrong point in time.
func AfterCommit(ctx context.Context, callback AfterCommitFunc) error {
	if ctx == nil || callback == nil {
		return ErrInvalidUnitOfWork
	}
	return RegisterAfterCompletion(ctx, func(callbackContext context.Context, committed bool) error {
		if !committed {
			return nil
		}
		return callback(callbackContext)
	})
}

// RegisterAfterCompletion registers callback on the active transaction. It is
// the primitive used by AfterCommit and by stateful components that must
// publish on commit or restore an in-memory fence after rollback.
func RegisterAfterCompletion(ctx context.Context, callback AfterCompletionFunc) error {
	if ctx == nil || callback == nil {
		return ErrInvalidUnitOfWork
	}
	current, ok := scopeFrom(ctx)
	if !ok {
		return ErrTransactionRequired
	}
	return current.register(callback)
}

func (current *scope) register(callback AfterCompletionFunc) error {
	if current == nil || callback == nil {
		return ErrInvalidUnitOfWork
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	if !current.valid.Load() {
		return ErrTransactionClosed
	}
	current.callbacks = append(current.callbacks, callback)
	return nil
}

func (current *scope) close() {
	if current == nil {
		return
	}
	current.mu.Lock()
	current.valid.Store(false)
	current.mu.Unlock()
}

// drain snapshots the FIFO before callbacks execute. No scope lock is held
// while application code runs, and callbacks cannot extend a closed unit of
// work by registering more callbacks.
func (current *scope) drain() []AfterCompletionFunc {
	if current == nil {
		return nil
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	callbacks := append([]AfterCompletionFunc(nil), current.callbacks...)
	current.callbacks = nil
	return callbacks
}

func (current *scope) discard() {
	if current == nil {
		return
	}
	current.mu.Lock()
	current.callbacks = nil
	current.mu.Unlock()
}

// Client resolves the transaction-scoped Ent client for database adapters.
// Outside a UnitOfWork it returns fallback. Passing nil fallback makes a
// transaction mandatory. A context that once held a scope never falls back
// after that scope closes, which prevents accidental post-commit writes.
func Client(ctx context.Context, fallback *generate.Client) (*generate.Client, error) {
	if value, ok := scopeFrom(ctx); ok {
		if !value.valid.Load() || value.client == nil {
			return nil, ErrTransactionClosed
		}
		return value.client, nil
	}
	if fallback == nil {
		return nil, ErrTransactionRequired
	}
	return fallback, nil
}

func completionCallbacksError(committed bool, causes []error) error {
	filtered := make([]error, 0, len(causes))
	for _, cause := range causes {
		if cause != nil {
			filtered = append(filtered, cause)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if committed {
		return &PostCommitError{causes: filtered}
	}
	return &CompletionError{causes: filtered}
}

func invokeAfterCompletion(ctx context.Context, committed bool, callback AfterCompletionFunc) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrPostCommitCallbackPanic
		}
	}()
	if callback == nil {
		return ErrInvalidUnitOfWork
	}
	return callback(ctx, committed)
}

func runAfterCompletion(ctx context.Context, committed bool, callbacks []AfterCompletionFunc) error {
	if len(callbacks) == 0 {
		return nil
	}
	callbackContext := context.WithoutCancel(ctx)
	causes := make([]error, 0, len(callbacks))
	for _, callback := range callbacks {
		if err := invokeAfterCompletion(callbackContext, committed, callback); err != nil {
			causes = append(causes, err)
		}
	}
	return completionCallbacksError(committed, causes)
}
