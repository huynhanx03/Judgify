package transaction

import (
	"context"
	"errors"
)

// AfterCommitOrNow runs callback after the outermost active transaction
// commits. Outside a transaction it runs synchronously, which is safe because
// the caller's repository-owned transaction has already returned successfully.
// All other registration failures fail closed.
func AfterCommitOrNow(ctx context.Context, callback AfterCommitFunc) error {
	err := AfterCommit(ctx, callback)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrTransactionRequired) {
		return err
	}
	return callback(context.WithoutCancel(ctx))
}
