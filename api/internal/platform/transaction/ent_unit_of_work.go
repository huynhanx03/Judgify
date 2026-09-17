package transaction

import (
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// EntUnitOfWork implements UnitOfWork with the generated Ent client.
type EntUnitOfWork struct {
	client  *generate.Client
	options *sql.TxOptions
}

// NewEntUnitOfWork creates a transaction boundary. A nil client is retained as
// an invalid value so dependency-injection mistakes fail through Within rather
// than panicking during process construction.
func NewEntUnitOfWork(client *generate.Client) *EntUnitOfWork {
	return &EntUnitOfWork{client: client}
}

// NewEntUnitOfWorkWithOptions creates a transaction boundary with explicit
// database isolation/read-only semantics. The options are copied so callers
// cannot mutate a live unit of work after construction.
func NewEntUnitOfWorkWithOptions(
	client *generate.Client,
	options sql.TxOptions,
) *EntUnitOfWork {
	copy := options
	return &EntUnitOfWork{client: client, options: &copy}
}

var _ UnitOfWork = (*EntUnitOfWork)(nil)

// Within commits only after fn succeeds. Transaction callback failure rolls
// back; panic invalidates the scoped context, rolls back, and is re-panicked by
// the shared Ent contract. Completion callbacks run after the physical outcome.
func (unit *EntUnitOfWork) Within(ctx context.Context, fn func(context.Context) error) (resultErr error) {
	if unit == nil || unit.client == nil || ctx == nil || fn == nil {
		return ErrInvalidUnitOfWork
	}
	if current, ok := scopeFrom(ctx); ok {
		if !current.valid.Load() {
			return ErrTransactionClosed
		}
		return ErrNestedTransaction
	}

	var current *scope
	defer func() {
		recovered := recover()
		if recovered != nil {
			// WithTx has already attempted rollback before re-panicking. Completion
			// callbacks still run exactly once, but their failures never replace
			// the original panic.
			completionErr := finishCompletion(ctx, current, false)
			logCompletionFailure(ctx, completionErr, false)
			panic(recovered)
		}
	}()
	transactionErr := commonent.WithTx(ctx, unit.begin, func(tx *generate.Tx) error {
		scopeValue := newScope(tx.Client())
		// Assign the outer variable so callbacks remain available only after
		// WithTx has reported a successful physical commit.
		current = scopeValue
		txCtx := bind(ctx, scopeValue)
		defer scopeValue.close()
		return fn(txCtx)
	})
	committed := transactionErr == nil
	completionErr := finishCompletion(ctx, current, committed)
	if completionErr != nil {
		logCompletionFailure(ctx, completionErr, committed)
	}
	if transactionErr != nil {
		if completionErr != nil {
			return errors.Join(transactionErr, completionErr)
		}
		return transactionErr
	}
	return completionErr
}

func (unit *EntUnitOfWork) begin(ctx context.Context) (*generate.Tx, error) {
	if unit.options == nil {
		return unit.client.Tx(ctx)
	}
	copy := *unit.options
	return unit.client.BeginTx(ctx, &copy)
}

func finishCompletion(ctx context.Context, current *scope, committed bool) error {
	if current == nil {
		return nil
	}
	callbacks := current.drain()
	current.discard()
	return runAfterCompletion(ctx, committed, callbacks)
}

func logCompletionFailure(ctx context.Context, err error, committed bool) {
	if err == nil {
		return
	}
	classification := ErrAfterCompletion
	message := "transaction completion callbacks failed"
	if committed {
		classification = ErrPostCommit
		message = "database committed but post-commit callbacks failed"
	}
	logger.FromContext(ctx).Error(
		message,
		zap.Bool("database_committed", committed),
		zap.Int("failed_callbacks", completionFailureCount(err)),
		zap.Error(classification),
	)
}

func completionFailureCount(err error) int {
	if failure, ok := err.(*PostCommitError); ok && failure != nil {
		return len(failure.causes)
	}
	if failure, ok := err.(*CompletionError); ok && failure != nil {
		return len(failure.causes)
	}
	return 1
}
