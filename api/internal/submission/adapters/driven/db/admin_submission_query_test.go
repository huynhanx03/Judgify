package db

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

type adminSubmissionQueries struct {
	mu      sync.Mutex
	queries []string
}

func (capture *adminSubmissionQueries) matcher(_, actual string) error {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.queries = append(capture.queries, actual)
	return nil
}

func (capture *adminSubmissionQueries) snapshot() []string {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]string(nil), capture.queries...)
}

func TestFindAdminUsesTypedFiltersDeterministicBoundAndSafeProjection(t *testing.T) {
	repository, mock, captured := newAdminSubmissionRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(adminSubmissionSummaryFields))
	problemID, userID, contestID := uuid.New(), uuid.New(), uuid.New()

	page, err := repository.FindAdmin(context.Background(), entity.AdminSubmissionQuery{
		Page: 2, PageSize: 25, Status: "accepted", Language: "go",
		ProblemID: &problemID, UserID: &userID, ContestID: &contestID,
	})
	require.NoError(t, err)
	require.Empty(t, *page.Records)
	require.Equal(t, 2, page.Pagination.CurrentPage)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		for _, field := range []string{"status", "language", "problem_id", "user_id", "contest_id"} {
			require.Contains(t, query, field)
		}
	}
	countSQL := strings.ToUpper(queries[0])
	require.NotContains(t, countSQL, "LIMIT")
	require.NotContains(t, countSQL, "OFFSET")

	recordSQL := strings.ToLower(queries[1])
	require.Contains(t, recordSQL, "order by")
	require.Contains(t, recordSQL, `"created_at" desc`)
	require.Contains(t, recordSQL, `"id" desc`)
	require.Contains(t, recordSQL, "limit 25")
	require.Contains(t, recordSQL, "offset 25")
	for _, secret := range []string{
		"source_code", "error_message", "judge_bundle_id", "judge_lease_token",
		"expected_output", "test_cases",
	} {
		require.NotContains(t, recordSQL, secret)
	}
}

func TestFindAdminRejectsInvalidWindowBeforeDatabaseAccess(t *testing.T) {
	repository, mock, captured := newAdminSubmissionRepository(t)
	_, err := repository.FindAdmin(context.Background(), entity.AdminSubmissionQuery{
		Page: 1, PageSize: entity.MaximumAdminSubmissionPageSize + 1,
	})
	require.ErrorIs(t, err, entity.ErrInvalidAdminSubmissionQuery)
	require.Empty(t, captured.snapshot())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindAdminClampsDeletedLastPageBeforeSelecting(t *testing.T) {
	repository, mock, captured := newAdminSubmissionRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(adminSubmissionSummaryFields))

	page, err := repository.FindAdmin(context.Background(), entity.AdminSubmissionQuery{
		Page: 3, PageSize: 25,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Pagination.CurrentPage)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	require.NotContains(t, strings.ToUpper(queries[1]), "OFFSET 50")
}

func newAdminSubmissionRepository(
	t *testing.T,
) (*SubmissionRepository, sqlmock.Sqlmock, *adminSubmissionQueries) {
	t.Helper()
	captured := &adminSubmissionQueries{}
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(
		sqlmock.QueryMatcherFunc(captured.matcher),
	))
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() {
		_ = client.Close()
	})
	return NewSubmissionRepository(dbent.WrapClient(client, nil)), mock, captured
}
