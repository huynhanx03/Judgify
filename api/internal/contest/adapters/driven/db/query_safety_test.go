package db

import (
	"context"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	commondto "github.com/huynhanx03/go-common/pkg/dto"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
)

type capturedContestQueries struct {
	mu      sync.Mutex
	queries []string
}

func (c *capturedContestQueries) matcher(_, actual string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = append(c.queries, actual)
	return nil
}

func (c *capturedContestQueries) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries...)
}

func TestContestFindMapsGenericNameSearchToTitle(t *testing.T) {
	repository, mock, captured := newContestQuerySafetyRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(contest.Columns))

	_, err := repository.Find(context.Background(), &commondto.QueryOptions{
		Filters: []commondto.SearchFilter{{
			Key: "name", Value: "weekly", Type: commonent.SearchTypeSearch,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Contains(t, query, contest.FieldTitle)
		require.NotContains(t, query, `"name"`, "generic admin search is an alias, not a contest column")
	}
}

func TestContestFindPublicCannotBeWidenedByDraftStatusFilter(t *testing.T) {
	repository, mock, captured := newContestQuerySafetyRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(contest.Columns))

	_, err := repository.FindPublic(context.Background(), &commondto.QueryOptions{
		Filters: []commondto.SearchFilter{{
			Key: "status", Value: contest.StatusDraft.String(), Type: commonent.SearchTypeExact,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Contains(t, query, `"status" <>`, "public predicate must exclude drafts independently of input filters")
		require.Contains(t, query, `"status" IN`, "malicious caller filter must only narrow the hard public predicate")
	}
}

func TestContestGetPublicRejectsDirectDraftIDAtDatabaseBoundary(t *testing.T) {
	repository, mock, captured := newContestQuerySafetyRepository(t)
	mock.ExpectQuery("public detail").WillReturnRows(sqlmock.NewRows(contest.Columns))

	result, err := repository.GetPublic(context.Background(), uuid.New())

	require.Nil(t, result)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	queries := captured.snapshot()
	require.Len(t, queries, 1)
	require.Contains(t, queries[0], `"status" <>`, "direct-ID public reads must carry the non-draft predicate")
}

func TestContestFindAdminDoesNotApplyPublicVisibilityPredicate(t *testing.T) {
	repository, mock, captured := newContestQuerySafetyRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(contest.Columns))

	_, err := repository.Find(context.Background(), &commondto.QueryOptions{})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	for _, query := range captured.snapshot() {
		require.NotContains(t, query, `"status" <>`, "authorized administration must include drafts")
	}
}

func newContestQuerySafetyRepository(t *testing.T) (*ContestRepository, sqlmock.Sqlmock, *capturedContestQueries) {
	t.Helper()
	captured := &capturedContestQueries{}
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(captured.matcher)))
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return NewContestRepository(dbent.WrapClient(client, nil)).(*ContestRepository), mock, captured
}
