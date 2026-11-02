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

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func TestRegistrationRepositoryRegisterSerializesAdmissionAndCommits(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000042")
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "upcoming", now.Add(time.Hour), now.Add(3*time.Hour), 10)
	expectDatabaseClock(mock, now)
	mock.ExpectQuery(`SELECT .* FROM "contest_registrations"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT COUNT\(.* FROM "contest_registrations"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(9))
	mock.ExpectExec(`INSERT INTO "contest_registrations"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repository.Register(context.Background(), contestID, userID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryRejectsInvalidCommandsBeforeOpeningTransaction(t *testing.T) {
	repository, mock := newRegistrationRepository(t)

	require.ErrorIs(
		t,
		repository.Register(context.Background(), uuid.Nil, uuid.New()),
		ErrInvalidRegistrationRepository,
	)
	require.ErrorIs(
		t,
		repository.Unregister(context.Background(), uuid.New(), uuid.Nil),
		ErrInvalidRegistrationRepository,
	)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryRegisterRejectsDuplicateBeforeCapacityCheck(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000042")
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "upcoming", now.Add(time.Hour), now.Add(3*time.Hour), 1)
	expectDatabaseClock(mock, now)
	mock.ExpectQuery(`SELECT .* FROM "contest_registrations"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectRollback()

	err := repository.Register(context.Background(), contestID, userID)
	require.ErrorIs(t, err, entity.ErrContestAlreadyRegistered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryRegisterRejectsFullContestWithoutInsert(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000042")
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "upcoming", now.Add(time.Hour), now.Add(3*time.Hour), 10)
	expectDatabaseClock(mock, now)
	mock.ExpectQuery(`SELECT .* FROM "contest_registrations"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT COUNT\(.* FROM "contest_registrations"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
	mock.ExpectRollback()

	err := repository.Register(context.Background(), contestID, userID)
	require.ErrorIs(t, err, entity.ErrContestRegistrationFull)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryRegisterRejectsDraftContest(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "draft", now.Add(time.Hour), now.Add(3*time.Hour), 0)
	expectDatabaseClock(mock, now)
	mock.ExpectRollback()

	err := repository.Register(context.Background(), contestID, uuid.New())
	require.ErrorIs(t, err, entity.ErrContestRegistrationNotOpen)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryRegisterUsesDatabaseClockToRejectStartedContest(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	now := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "upcoming", now, now.Add(2*time.Hour), 0)
	expectDatabaseClock(mock, now)
	mock.ExpectRollback()

	err := repository.Register(context.Background(), contestID, uuid.New())
	require.ErrorIs(t, err, entity.ErrContestRegistrationStarted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryRegisterMapsMissingContest(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "contests".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "start_time", "end_time", "max_participants"}))
	mock.ExpectRollback()

	err := repository.Register(context.Background(), contestID, uuid.New())
	require.ErrorIs(t, err, entity.ErrContestNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryUnregisterSerializesLifecycleAndCommits(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000042")
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "upcoming", now.Add(time.Hour), now.Add(3*time.Hour), 10)
	expectDatabaseClock(mock, now)
	mock.ExpectExec(`DELETE FROM "contest_registrations"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repository.Unregister(context.Background(), contestID, userID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRegistrationRepositoryUnregisterRejectsLifecycleAfterStart(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		startTime func(time.Time) time.Time
	}{
		{name: "running status", status: "running", startTime: func(now time.Time) time.Time { return now.Add(-time.Hour) }},
		{name: "ended status", status: "ended", startTime: func(now time.Time) time.Time { return now.Add(-2 * time.Hour) }},
		{name: "stale upcoming status", status: "upcoming", startTime: func(now time.Time) time.Time { return now }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository, mock := newRegistrationRepository(t)
			contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
			now := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)

			mock.ExpectBegin()
			expectLockedRegistrationContest(mock, contestID, test.status, test.startTime(now), now.Add(2*time.Hour), 0)
			expectDatabaseClock(mock, now)
			mock.ExpectRollback()

			err := repository.Unregister(context.Background(), contestID, uuid.New())
			require.ErrorIs(t, err, entity.ErrContestRegistrationStarted)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRegistrationRepositoryUnregisterReportsMissingRegistration(t *testing.T) {
	repository, mock := newRegistrationRepository(t)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	now := time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectLockedRegistrationContest(mock, contestID, "upcoming", now.Add(time.Hour), now.Add(3*time.Hour), 0)
	expectDatabaseClock(mock, now)
	mock.ExpectExec(`DELETE FROM "contest_registrations"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repository.Unregister(context.Background(), contestID, uuid.New())
	require.ErrorIs(t, err, entity.ErrContestRegistrationNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectLockedRegistrationContest(
	mock sqlmock.Sqlmock,
	id uuid.UUID,
	status string,
	startTime time.Time,
	endTime time.Time,
	maxParticipants int,
) {
	mock.ExpectQuery(`SELECT .* FROM "contests".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "start_time", "end_time", "max_participants"}).
			AddRow(id, status, startTime, endTime, maxParticipants))
}

func expectDatabaseClock(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
}

func newRegistrationRepository(t *testing.T) (*RegistrationRepository, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository := NewRegistrationRepository(dbent.WrapClient(client, nil)).(*RegistrationRepository)
	return repository, mock
}
