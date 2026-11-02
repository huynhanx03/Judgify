package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAfterCommitOrNowRunsInvalidationImmediatelyOutsideTransaction(t *testing.T) {
	invalidations := 0
	require.NoError(t, AfterCommitOrNow(context.Background(), func(context.Context) error {
		invalidations++
		return nil
	}))
	require.Equal(t, 1, invalidations)
}

func TestAfterCommitOrNowDefersInvalidationAndSkipsRollback(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()
	invalidations := 0
	require.NoError(t, NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, AfterCommitOrNow(txCtx, func(context.Context) error {
			invalidations++
			return nil
		}))
		require.Zero(t, invalidations, "cache invalidation must wait for outer commit")
		return nil
	}))
	require.Equal(t, 1, invalidations)
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectBegin()
	mock.ExpectRollback()
	rollback := errors.New("rollback")
	invalidations = 0
	err := NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, AfterCommitOrNow(txCtx, func(context.Context) error {
			invalidations++
			return nil
		}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.Zero(t, invalidations, "rollback must leave cache untouched")
	require.NoError(t, mock.ExpectationsWereMet())
}
