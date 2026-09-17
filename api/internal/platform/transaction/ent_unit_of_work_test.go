package transaction

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func TestEntUnitOfWorkCommitsOnSuccessAndExposesOnlyScopedContext(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	uow := NewEntUnitOfWork(client)
	var scoped context.Context
	require.NoError(t, uow.Within(context.Background(), func(txCtx context.Context) error {
		scoped = txCtx
		txClient, err := Client(txCtx, nil)
		require.NoError(t, err)
		require.NotSame(t, client, txClient)
		return nil
	}))
	require.NoError(t, mock.ExpectationsWereMet())

	_, err := Client(scoped, client)
	require.ErrorIs(t, err, ErrTransactionClosed)

	// The application-facing contract exposes only context.Context, never Ent.
	var contract interface {
		Within(context.Context, func(context.Context) error) error
	} = uow
	require.NotNil(t, contract)
	var _ UnitOfWork = uow
}

func TestEntUnitOfWorkRunsCompletionCallbacksFIFOAfterCommit(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	var order []int
	uow := NewEntUnitOfWork(client)
	require.NoError(t, uow.Within(context.Background(), func(txCtx context.Context) error {
		for i := 1; i <= 3; i++ {
			value := i
			require.NoError(t, RegisterAfterCompletion(txCtx, func(context.Context, bool) error {
				order = append(order, value)
				return nil
			}))
		}
		require.Empty(t, order, "callbacks must not run before commit")
		return nil
	}))
	require.Equal(t, []int{1, 2, 3}, order)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkRollsBackOnCallbackErrorAndPreservesRollbackFailure(t *testing.T) {
	client, mock := newMockClient(t)
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")
	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(rollbackErr)

	err := NewEntUnitOfWork(client).Within(context.Background(), func(context.Context) error {
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, rollbackErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkRunsCompletionAfterRollbackAndSkipsAfterCommit(t *testing.T) {
	client, mock := newMockClient(t)
	callbackErr := errors.New("callback failed")
	mock.ExpectBegin()
	mock.ExpectRollback()

	var completed []bool
	afterCommitRan := false
	err := NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, RegisterAfterCompletion(txCtx, func(_ context.Context, committed bool) error {
			completed = append(completed, committed)
			return nil
		}))
		require.NoError(t, AfterCommit(txCtx, func(context.Context) error {
			afterCommitRan = true
			return nil
		}))
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)
	require.Equal(t, []bool{false}, completed)
	require.False(t, afterCommitRan)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkRollsBackAndRepanicsAfterInvalidatingContext(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	uow := NewEntUnitOfWork(client)
	panicValue := &struct{ message string }{"boom"}
	var scoped context.Context

	recovered := func() (recovered any) {
		defer func() { recovered = recover() }()
		_ = uow.Within(context.Background(), func(txCtx context.Context) error {
			scoped = txCtx
			panic(panicValue)
		})
		return nil
	}()

	require.Same(t, panicValue, recovered)
	_, err := Client(scoped, nil)
	require.ErrorIs(t, err, ErrTransactionClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkPreservesCommitError(t *testing.T) {
	client, mock := newMockClient(t)
	commitErr := errors.New("commit failed")
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(commitErr)

	err := NewEntUnitOfWork(client).Within(context.Background(), func(context.Context) error {
		return nil
	})
	require.ErrorIs(t, err, commitErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkRunsCompletionAfterCommitFailure(t *testing.T) {
	client, mock := newMockClient(t)
	commitErr := errors.New("commit failed")
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(commitErr)

	var committedValues []bool
	err := NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		return RegisterAfterCompletion(txCtx, func(_ context.Context, committed bool) error {
			committedValues = append(committedValues, committed)
			return nil
		})
	})
	require.ErrorIs(t, err, commitErr)
	require.Equal(t, []bool{false}, committedValues)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkClassifiesPostCommitFailuresAndContinues(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	callbackErr := errors.New("cache fence failed")
	var laterRan bool
	err := NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, AfterCommit(txCtx, func(context.Context) error { return callbackErr }))
		require.NoError(t, AfterCommit(txCtx, func(context.Context) error { panic("sensitive panic value") }))
		require.NoError(t, AfterCommit(txCtx, func(context.Context) error {
			laterRan = true
			return nil
		}))
		return nil
	})
	require.ErrorIs(t, err, ErrPostCommit)
	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, ErrPostCommitCallbackPanic)
	require.True(t, laterRan, "one callback failure must not suppress later callbacks")
	var committed interface{ Committed() bool }
	require.ErrorAs(t, err, &committed)
	require.True(t, committed.Committed())
	require.NotContains(t, err.Error(), "sensitive panic value")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkJoinsRollbackAndCompletionFailures(t *testing.T) {
	client, mock := newMockClient(t)
	transactionErr := errors.New("mutation failed")
	completionErr := errors.New("restore fence failed")
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, RegisterAfterCompletion(txCtx, func(context.Context, bool) error {
			return completionErr
		}))
		return transactionErr
	})
	require.ErrorIs(t, err, transactionErr)
	require.ErrorIs(t, err, ErrAfterCompletion)
	require.ErrorIs(t, err, completionErr)
	var completion interface{ Committed() bool }
	require.ErrorAs(t, err, &completion)
	require.False(t, completion.Committed())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkCompletionRegistrationFailsAfterScopeClosesWithoutDeadlock(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	var scoped context.Context
	nestedRan := false
	require.NoError(t, NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		scoped = txCtx
		return AfterCommit(txCtx, func(context.Context) error {
			err := AfterCommit(scoped, func(context.Context) error {
				nestedRan = true
				return nil
			})
			require.ErrorIs(t, err, ErrTransactionClosed)
			return nil
		})
	}))
	require.False(t, nestedRan)
	require.ErrorIs(t, AfterCommit(scoped, func(context.Context) error { return nil }), ErrTransactionClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkConcurrentCompletionRegistrationRunsEachOnce(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	const callbackCount = 128
	var ran atomic.Int64
	err := NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		var wait sync.WaitGroup
		errorsSeen := make(chan error, callbackCount)
		for range callbackCount {
			wait.Add(1)
			go func() {
				defer wait.Done()
				errorsSeen <- RegisterAfterCompletion(txCtx, func(context.Context, bool) error {
					ran.Add(1)
					return nil
				})
			}()
		}
		wait.Wait()
		close(errorsSeen)
		for registrationErr := range errorsSeen {
			require.NoError(t, registrationErr)
		}
		require.Zero(t, ran.Load())
		return nil
	})
	require.NoError(t, err)
	require.EqualValues(t, callbackCount, ran.Load())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntUnitOfWorkRejectsNestedTransactions(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	uow := NewEntUnitOfWork(client)
	require.NoError(t, uow.Within(context.Background(), func(txCtx context.Context) error {
		err := uow.Within(txCtx, func(context.Context) error {
			t.Fatal("nested callback must not run")
			return nil
		})
		require.ErrorIs(t, err, ErrNestedTransaction)
		return nil
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionClientFallsBackOnlyOutsideAScope(t *testing.T) {
	client, _ := newMockClient(t)

	resolved, err := Client(context.Background(), client)
	require.NoError(t, err)
	require.Same(t, client, resolved)

	_, err = Client(context.Background(), nil)
	require.ErrorIs(t, err, ErrTransactionRequired)
}

func newMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}
