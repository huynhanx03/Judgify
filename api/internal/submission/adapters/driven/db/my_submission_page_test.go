package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestFindMyByProblemUsesCompositeSeekAndLimitPlusOne(t *testing.T) {
	repository, mock, captured := newAdminSubmissionRepository(t)
	userID := uuid.MustParse("00000000-0000-7000-8000-000000000031")
	problemID := uuid.MustParse("00000000-0000-7000-8000-000000000021")
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000041")
	after := ports.SubmissionCursor{
		CreatedAt: time.Date(2026, time.July, 29, 9, 0, 0, 0, time.UTC),
		ID:        uuid.MustParse("00000000-0000-7000-8000-000000000019"),
	}
	rows := sqlmock.NewRows(submissionSummaryFields)
	for index := 0; index < 3; index++ {
		rows.AddRow(
			uuid.UUID{15: byte(index + 1)},
			after.CreatedAt.Add(-time.Duration(index+1)*time.Second),
			problemID,
			userID,
			"go",
			"accepted",
			3,
			3,
			10,
			1024,
			contestID,
		)
	}
	mock.ExpectQuery("my submissions").WillReturnRows(rows)

	page, err := repository.FindMyByProblem(
		context.Background(),
		ports.MySubmissionQuery{
			UserID: userID, ProblemID: problemID, ContestID: &contestID,
			After: &after, Limit: 2,
		},
	)
	require.NoError(t, err)
	require.Len(t, page.Submissions, 2)
	require.NotNil(t, page.Next)
	require.Equal(t, page.Submissions[1].ID, page.Next.ID)
	require.Equal(t, page.Submissions[1].CreatedAt, page.Next.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 1)
	query := strings.ToLower(queries[0])
	require.Contains(t, query, `"user_id"`)
	require.Contains(t, query, `"problem_id"`)
	require.Contains(t, query, `"contest_id"`)
	require.Contains(t, query, `"created_at" <`)
	require.Contains(t, query, `"created_at" =`)
	require.Contains(t, query, `"id" <`)
	require.Contains(t, query, `"created_at" desc`)
	require.Contains(t, query, `"id" desc`)
	require.Contains(t, query, "limit 3")
	require.NotContains(t, query, "offset")
	for _, secret := range []string{
		"source_code", "error_message", "judge_bundle_id", "judge_lease_token",
	} {
		require.NotContains(t, query, secret)
	}
}

func TestFindMyByProblemRejectsInvalidQueryBeforeDatabase(t *testing.T) {
	repository, mock, captured := newAdminSubmissionRepository(t)
	_, err := repository.FindMyByProblem(
		context.Background(),
		ports.MySubmissionQuery{Limit: ports.MaximumMySubmissionPageLimit + 1},
	)
	require.ErrorIs(t, err, ports.ErrInvalidMySubmissionPage)
	require.Empty(t, captured.snapshot())
	require.NoError(t, mock.ExpectationsWereMet())
}
