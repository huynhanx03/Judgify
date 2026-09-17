// Package transaction owns Judgify's transaction-scoped UnitOfWork boundary.
// Application and domain code receive only context.Context; Ent stays private
// to database adapters and platform persistence packages.
package transaction

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrInvalidUnitOfWork reports a nil dependency, context, or callback.
	ErrInvalidUnitOfWork = errors.New("transaction: invalid unit of work")
	// ErrNestedTransaction reports an attempt to start a transaction from an
	// already active transaction scope.
	ErrNestedTransaction = errors.New("transaction: nested transaction")
	// ErrTransactionRequired reports a persistence operation that must be part
	// of a UnitOfWork but received an ordinary context.
	ErrTransactionRequired = errors.New("transaction: transaction required")
	// ErrTransactionClosed reports use of a scoped context after its callback
	// returned or panicked.
	ErrTransactionClosed = errors.New("transaction: transaction scope closed")
	// ErrPostCommit reports that the database commit succeeded but one or more
	// post-commit callbacks failed. Callers must not retry the database mutation
	// as though it had rolled back.
	ErrPostCommit = errors.New("transaction: database committed but post-commit work failed")
	// ErrPostCommitCallbackPanic reports a recovered callback panic without
	// exposing the recovered value, which may contain sensitive data.
	ErrPostCommitCallbackPanic = errors.New("transaction: post-commit callback panicked")
	// ErrAfterCompletion reports callback failure after a transaction that did
	// not commit (callback error, rollback, or commit failure).
	ErrAfterCompletion = errors.New("transaction: after-completion work failed")
)

// AfterCommitFunc is bounded, non-transactional work that runs synchronously
// after the outermost database commit. It must not retain the supplied context
// or start unowned goroutines.
type AfterCommitFunc func(context.Context) error

// AfterCompletionFunc runs exactly once after the outermost transaction has
// physically completed. committed is true only after Commit returned nil.
type AfterCompletionFunc func(context.Context, bool) error

// PostCommitError is deliberately distinct from a transaction/rollback
// failure: the database mutation is durable when this error is returned.
type PostCommitError struct {
	causes []error
}

func (err *PostCommitError) Error() string {
	if err == nil {
		return ErrPostCommit.Error()
	}
	return fmt.Sprintf("%s (%d callback failures)", ErrPostCommit.Error(), len(err.causes))
}

// Unwrap keeps both the stable classification and individual callback errors
// available to errors.Is/errors.As without rendering their text in Error().
func (err *PostCommitError) Unwrap() []error {
	if err == nil {
		return []error{ErrPostCommit}
	}
	result := make([]error, 0, len(err.causes)+1)
	result = append(result, ErrPostCommit)
	result = append(result, err.causes...)
	return result
}

// Committed lets application boundaries distinguish this state from a failed
// database transaction without importing this package's concrete error type.
func (*PostCommitError) Committed() bool { return true }

// CompletionError reports callback failure after a transaction that did not
// commit. The original transaction error is returned alongside this error.
type CompletionError struct {
	causes []error
}

func (err *CompletionError) Error() string {
	if err == nil {
		return ErrAfterCompletion.Error()
	}
	return fmt.Sprintf("%s (%d callback failures)", ErrAfterCompletion.Error(), len(err.causes))
}

func (err *CompletionError) Unwrap() []error {
	if err == nil {
		return []error{ErrAfterCompletion}
	}
	result := make([]error, 0, len(err.causes)+1)
	result = append(result, ErrAfterCompletion)
	result = append(result, err.causes...)
	return result
}

func (*CompletionError) Committed() bool { return false }

// UnitOfWork executes a bounded callback in one database transaction. The
// callback must not start goroutines or perform network, container, or other
// unbounded work. Its context is valid only until the callback returns.
type UnitOfWork interface {
	Within(ctx context.Context, fn func(txCtx context.Context) error) error
}
