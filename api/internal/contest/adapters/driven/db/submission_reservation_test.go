package db

import (
	"context"
	"encoding/json"
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
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestReserveContestSubmissionUsesOneDatabaseTimeAndContiguousCounter(t *testing.T) {
	repository, client, mock := newSubmissionReservationRepository(t)
	contestID, contestProblemID, problemID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	revisionID, testsetID := uuid.New(), uuid.New()
	now := time.Date(2026, time.July, 28, 3, 0, 0, 0, time.UTC)
	checksum := make([]byte, 32)
	checksum[0] = 1
	runtimes, _ := json.Marshal([]string{"cpp17", "go"})

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM contests.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "start_time", "end_time", "registration_mode", "max_participants",
			"next_submission_sequence", "scoring_policy", "scoring_policy_version",
		}).AddRow(contestID, "running", now, now.Add(time.Hour), "registered", 10, 7, "icpc", 1))
	mock.ExpectQuery(`FROM contest_registrations.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "source", "version", "official_submission_count"}).
			AddRow(uuid.New(), "registered", "explicit", 1, 0))
	mock.ExpectQuery(`FROM contest_problems`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contest_id", "problem_id", "problem_revision_id", "testset_revision_id",
			"checker_key", "checker_version", "checker_config", "allowed_runtime_keys", "snapshot_checksum",
		}).AddRow(contestProblemID, contestID, problemID, revisionID, testsetID,
			"exact", "v1", []byte(`{}`), runtimes, checksum))
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
	mock.ExpectExec(`UPDATE contest_registrations.*official_submission_count`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE contests.*next_submission_sequence.*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_sequence"}).AddRow(7))
	mock.ExpectCommit()

	var result ports.ContestSubmissionReservation
	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		var reserveErr error
		result, reserveErr = repository.ReserveContestSubmission(txCtx, ports.ReserveContestSubmissionRequest{
			ContestID: contestID, ContestProblemID: contestProblemID, ProblemID: problemID,
			UserID: userID, RuntimeKey: "go",
		})
		return reserveErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), result.ContestSubmissionSequence)
	require.Equal(t, now, result.SubmittedAt)
	require.Zero(t, result.SubmissionElapsedMillis)
	require.Equal(t, revisionID, result.ProblemRevisionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveContestSubmissionOpenModeConsumesCapacityBeforeCounter(t *testing.T) {
	repository, client, mock := newSubmissionReservationRepository(t)
	contestID, contestProblemID, problemID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, time.July, 28, 3, 30, 0, 0, time.UTC)
	checksum := make([]byte, 32)
	checksum[0] = 1

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM contests.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "start_time", "end_time", "registration_mode", "max_participants",
			"next_submission_sequence", "scoring_policy", "scoring_policy_version",
		}).AddRow(contestID, "running", now.Add(-time.Minute), now.Add(time.Hour), "open", 1, 1, "icpc", 1))
	mock.ExpectQuery(`FROM contest_registrations.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "source", "version", "official_submission_count"}))
	mock.ExpectQuery(`FROM contest_problems`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contest_id", "problem_id", "problem_revision_id", "testset_revision_id",
			"checker_key", "checker_version", "checker_config", "allowed_runtime_keys", "snapshot_checksum",
		}).AddRow(contestProblemID, contestID, problemID, uuid.New(), uuid.New(),
			"exact", "v1", []byte(`{}`), []byte(`["go"]`), checksum))
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*status = 'registered'`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`INSERT INTO contest_registrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO contest_registration_transitions`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE contest_registrations.*official_submission_count`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE contests.*RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_sequence"}).AddRow(1))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		_, reserveErr := repository.ReserveContestSubmission(txCtx, ports.ReserveContestSubmissionRequest{
			ContestID: contestID, ContestProblemID: contestProblemID, ProblemID: problemID,
			UserID: userID, RuntimeKey: "go",
		})
		return reserveErr
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveContestSubmissionRejectsExactEndWithoutConsumingSequence(t *testing.T) {
	repository, client, mock := newSubmissionReservationRepository(t)
	contestID, contestProblemID, problemID, userID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, time.July, 28, 4, 0, 0, 0, time.UTC)
	checksum := make([]byte, 32)
	checksum[0] = 1

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM contests.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "status", "start_time", "end_time", "registration_mode", "max_participants",
		"next_submission_sequence", "scoring_policy", "scoring_policy_version",
	}).AddRow(contestID, "running", now.Add(-time.Hour), now, "registered", nil, 2, "icpc", 1))
	mock.ExpectQuery(`FROM contest_registrations.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "status", "source", "version", "official_submission_count",
	}).AddRow(uuid.New(), "registered", "explicit", 1, 0))
	mock.ExpectQuery(`FROM contest_problems`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "contest_id", "problem_id", "problem_revision_id", "testset_revision_id",
		"checker_key", "checker_version", "checker_config", "allowed_runtime_keys", "snapshot_checksum",
	}).AddRow(contestProblemID, contestID, problemID, uuid.New(), uuid.New(), "exact", "v1", []byte(`{}`), []byte(`["go"]`), checksum))
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
	mock.ExpectRollback()

	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		_, reserveErr := repository.ReserveContestSubmission(txCtx, ports.ReserveContestSubmissionRequest{
			ContestID: contestID, ContestProblemID: contestProblemID, ProblemID: problemID,
			UserID: userID, RuntimeKey: "go",
		})
		return reserveErr
	})
	require.ErrorIs(t, err, entity.ErrContestNotRunning)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newSubmissionReservationRepository(t *testing.T) (*SubmissionReservationRepository, *generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository, err := NewSubmissionReservationRepository(dbent.WrapClient(client, nil))
	require.NoError(t, err)
	return repository, client, mock
}
