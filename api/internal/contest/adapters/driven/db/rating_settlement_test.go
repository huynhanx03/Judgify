package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

func TestRatingSettlementFindPendingIsBoundedAndDeterministic(t *testing.T) {
	repository, mock, _ := newRatingSettlementRepository(t)
	firstID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	secondID := uuid.MustParse("00000000-0000-7000-8000-000000000002")

	mock.ExpectQuery(`SELECT .*"id".* FROM "contests".*"status".*"rating_completed_at" IS NULL.*ORDER BY .*"end_time".*"id".*LIMIT`).
		WithArgs("ended").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(firstID).AddRow(secondID))

	ids, err := repository.FindPending(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{firstID, secondID}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingSettlementFindPendingRejectsInvalidLimit(t *testing.T) {
	repository, mock, _ := newRatingSettlementRepository(t)

	_, err := repository.FindPending(context.Background(), 0)
	require.ErrorIs(t, err, ErrInvalidRatingSettlementRepository)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingSettlementCommandsRequireCallerTransaction(t *testing.T) {
	repository, mock, _ := newRatingSettlementRepository(t)
	contestID := uuid.New()

	_, err := repository.LockPending(context.Background(), contestID)
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	err = repository.Complete(context.Background(), contestID, time.Now().UTC())
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingSettlementLocksAndCompletesInsideOneTransaction(t *testing.T) {
	repository, mock, client := newRatingSettlementRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000042")
	completedAt := time.Date(2026, time.July, 22, 6, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*"id".* FROM "contests".*"status".*"rating_completed_at" IS NULL.*FOR UPDATE`).
		WithArgs(contestID, "ended").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(contestID))
	mock.ExpectExec(`UPDATE "contests" SET .*"rating_completed_at".* WHERE .*"id".*"status".*"rating_completed_at" IS NULL`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		pending, err := repository.LockPending(txCtx, contestID)
		require.NoError(t, err)
		require.True(t, pending)
		return repository.Complete(txCtx, contestID, completedAt)
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingSettlementConcurrentWinnerMakesLaterLockIdempotent(t *testing.T) {
	repository, mock, client := newRatingSettlementRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000043")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "contests".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		pending, err := repository.LockPending(txCtx, contestID)
		require.NoError(t, err)
		require.False(t, pending)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingSettlementCompleteFailsClosedWhenGuardMatchesNothing(t *testing.T) {
	repository, mock, client := newRatingSettlementRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000044")

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "contests"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		return repository.Complete(txCtx, contestID, time.Now().UTC())
	})
	require.ErrorIs(t, err, ErrRatingSettlementStateChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newRatingSettlementRepository(t *testing.T) (*RatingSettlementRepository, sqlmock.Sqlmock, *generate.Client) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository := NewRatingSettlementRepository(dbent.WrapClient(client, zap.NewNop())).(*RatingSettlementRepository)
	return repository, mock, client
}
