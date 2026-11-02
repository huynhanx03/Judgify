package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestRegistrationCommandLocksContestBeforeCapacityAndAppendsReceipt(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository, err := NewRegistrationCommandRepository(dbent.WrapClient(client, nil))
	require.NoError(t, err)

	contestID, userID, actorID, commandID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, time.July, 28, 3, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`FROM contests.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{
		"status", "version", "registration_mode", "registration_opens_at", "registration_closes_at", "max_participants",
	}).AddRow("upcoming", 2, "registered", now.Add(-time.Hour), now.Add(time.Hour), 1))
	mock.ExpectQuery(`FROM contest_registration_transitions`).WillReturnRows(sqlmock.NewRows([]string{
		"registration_id", "command_hash", "registration_version", "official_submission_count", "to_status", "source", "occurred_at",
	}))
	mock.ExpectQuery(`FROM contest_registrations.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "status", "source", "version", "official_submission_count",
	}))
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*status = 'registered'`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`INSERT INTO contest_registrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO contest_registration_transitions`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result ports.RegistrationResult
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		var applyErr error
		result, applyErr = repository.Apply(txCtx, ports.RegistrationCommand{
			ContestID: contestID, UserID: userID, ExpectedContestVersion: 2,
			IdempotencyKey: commandID, ActorUserID: actorID, CommandHash: [32]byte{1},
		}, ports.RegistrationRegister)
		return applyErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Registration.Version)
	require.Equal(t, now, result.OccurredAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
