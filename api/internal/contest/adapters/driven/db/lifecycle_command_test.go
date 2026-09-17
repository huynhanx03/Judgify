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

func TestLifecycleRepositoryLocksContestUsesDatabaseTimeAndPersistsTransitionFirst(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository, err := NewLifecycleRepository(dbent.WrapClient(client, nil))
	require.NoError(t, err)

	contestID, commandID, actorID := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, time.July, 28, 2, 0, 0, 0, time.UTC)
	commandHash := [32]byte{1}
	mock.ExpectBegin()
	mock.ExpectQuery(`FROM contests.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "status", "version", "start_time", "end_time", "registration_mode",
		"registration_opens_at", "registration_closes_at", "max_participants",
		"next_submission_sequence", "scoring_policy", "scoring_policy_version",
		"standings_projection_version", "rated", "rating_algorithm_version",
		"freeze_standings_at",
	}).AddRow(contestID, "draft", 1, now.Add(time.Hour), now.Add(2*time.Hour), "open",
		nil, nil, nil, 1, "icpc", 1, 1, true, entity.RatingAlgorithmExpectedRankV1, nil))
	mock.ExpectQuery(`FROM contest_transitions`).WillReturnRows(sqlmock.NewRows([]string{
		"command_hash", "version", "from_state", "to_state", "occurred_at",
	}))
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM contest_problems`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO contest_transitions`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE contests`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var result ports.TransitionResult
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		var transitionErr error
		result, transitionErr = repository.Transition(txCtx, ports.TransitionCommand{
			ContestID: contestID, Target: entity.ContestUpcoming, ExpectedVersion: 1,
			IdempotencyKey: commandID, ActorUserID: actorID,
			Reason: "publish reviewed contest", CommandHash: commandHash,
		})
		return transitionErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Version)
	require.Equal(t, now, result.OccurredAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLifecycleRepositoryFreezesExactContestProblemsBeforePublishing(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repository, err := NewLifecycleRepository(
		dbent.WrapClient(client, nil),
	)
	require.NoError(t, err)

	contestID, commandID, actorID := uuid.New(), uuid.New(), uuid.New()
	problemID, revisionID, testsetID := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, time.July, 28, 2, 0, 0, 0, time.UTC)
	contentChecksum := make([]byte, 32)
	contentChecksum[0] = 1
	runtimes, err := json.Marshal([]string{"go"})
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM contests.*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"status",
			"version",
			"start_time",
			"end_time",
			"registration_mode",
			"registration_opens_at",
			"registration_closes_at",
			"max_participants",
			"next_submission_sequence",
			"scoring_policy",
			"scoring_policy_version",
			"standings_projection_version",
			"rated",
			"rating_algorithm_version",
			"freeze_standings_at",
		}).AddRow(
			contestID,
			"draft",
			1,
			now.Add(time.Hour),
			now.Add(2*time.Hour),
			"open",
			nil,
			nil,
			nil,
			1,
			"icpc",
			1,
			1,
			true,
			entity.RatingAlgorithmExpectedRankV1,
			nil,
		))
	mock.ExpectQuery(`FROM contest_transitions`).
		WillReturnRows(sqlmock.NewRows([]string{
			"command_hash",
			"version",
			"from_state",
			"to_state",
			"occurred_at",
		}))
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).
		WillReturnRows(
			sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now),
		)
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM contest_problems`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM contest_draft_problems`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`FROM contest_draft_problems AS selected`).
		WillReturnRows(sqlmock.NewRows([]string{
			"display_order",
			"alias",
			"points",
			"visible_before_start",
			"problem_id",
			"problem_revision_id",
			"testset_revision_id",
			"checker_key",
			"checker_version",
			"checker_config",
			"allowed_runtime_keys",
			"content_checksum",
		}).AddRow(
			1,
			"A",
			nil,
			true,
			problemID,
			revisionID,
			testsetID,
			"exact",
			"v1",
			[]byte(`{}`),
			runtimes,
			contentChecksum,
		))
	mock.ExpectExec(`INSERT INTO contest_problems`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO contest_transitions`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE contests`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(txCtx context.Context) error {
			_, transitionErr := repository.Transition(
				txCtx,
				ports.TransitionCommand{
					ContestID:       contestID,
					Target:          entity.ContestUpcoming,
					ExpectedVersion: 1,
					IdempotencyKey:  commandID,
					ActorUserID:     actorID,
					Reason:          "publish reviewed contest",
					CommandHash:     [32]byte{1},
				},
			)
			return transitionErr
		},
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
