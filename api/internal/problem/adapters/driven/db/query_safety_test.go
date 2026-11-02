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

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	commondto "github.com/huynhanx03/go-common/pkg/dto"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/tag"
)

type capturedQueries struct {
	mu      sync.Mutex
	queries []string
}

func (c *capturedQueries) matcher(_, actual string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = append(c.queries, actual)
	return nil
}

func (c *capturedQueries) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries...)
}

func TestProblemFindPublishedDropsUnknownColumnsAndAlwaysBoundsRecords(t *testing.T) {
	tests := []struct {
		name    string
		options *commondto.QueryOptions
	}{
		{
			name: "unknown filter",
			options: &commondto.QueryOptions{Filters: []commondto.SearchFilter{{
				Key: "does_not_exist", Value: "boom", Type: commonent.OpEQ,
			}}},
		},
		{name: "empty body", options: &commondto.QueryOptions{}},
		{name: "nil options", options: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, mock, captured := newProblemQuerySafetyRepository(t)
			expectEmptyProblemFind(mock)

			_, err := repository.FindPublished(context.Background(), tt.options)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())

			queries := captured.snapshot()
			require.Len(t, queries, 2)
			for _, query := range queries {
				require.NotContains(t, query, "does_not_exist")
				require.Contains(t, query, problem.FieldLifecycle)
				require.Contains(t, query, string(problem.LifecyclePublished))
				require.Contains(t, query, problem.FieldPublishedRevisionID)
				require.Contains(t, query, problem.FieldPublishedRenderSourceID)
				require.Contains(t, query, problem.FieldPublishedRenderSourceKind)
				require.Contains(t, query, problem.FieldPublishedRenderSourceChecksum)
				require.Contains(t, query, problem.FieldPublishedRenderArtifactID)
			}
			require.NotContains(t, strings.ToUpper(queries[0]), "LIMIT", "count must remain unwindowed")
			require.Contains(t, strings.ToUpper(queries[1]), "LIMIT 10")
		})
	}
}

func TestProblemFindPublishedPreservesDifficultyAndTagFilters(t *testing.T) {
	repository, mock, captured := newProblemQuerySafetyRepository(t)
	expectEmptyProblemFind(mock)
	difficultyID := uuid.New()
	tagID := uuid.New()

	_, err := repository.FindPublished(context.Background(), &commondto.QueryOptions{
		Filters: []commondto.SearchFilter{
			{Key: problem.FieldDifficultyID, Value: difficultyID.String(), Type: commonent.SearchTypeExact},
			{Key: "tag_ids", Value: []any{tagID.String()}, Type: commonent.SearchTypeFilter},
		},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Contains(t, query, problem.FieldDifficultyID)
		require.Contains(t, query, "problem_tags", "tag_ids must become a typed relation predicate")
		require.NotContains(t, query, `"tag_ids"`, "tag_ids is not a physical problem column")
	}
}

func TestProblemFindPublishedSupportsTypedAcceptanceRateSort(t *testing.T) {
	repository, mock, captured := newProblemQuerySafetyRepository(t)
	expectEmptyProblemFind(mock)

	_, err := repository.FindPublished(
		context.Background(),
		&commondto.QueryOptions{
			Sort: []commondto.SortOption{{
				Key:   "acceptance_rate",
				Order: -1,
			}},
		},
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	recordsQuery := strings.ToUpper(queries[1])
	require.Contains(t, recordsQuery, "CASE WHEN")
	require.Contains(t, recordsQuery, strings.ToUpper(problem.FieldAcceptedCount))
	require.Contains(t, recordsQuery, strings.ToUpper(problem.FieldSubmissionCount))
	require.Contains(t, recordsQuery, "NULLIF")
	require.Contains(t, recordsQuery, " DESC")
	require.Contains(t, recordsQuery, strings.ToUpper(problem.FieldID))
	require.NotContains(
		t,
		strings.ToUpper(queries[0]),
		"ORDER BY",
		"count query must remain unwindowed",
	)
}

func TestTagFindPreservesElementRelationFilter(t *testing.T) {
	repository, mock, captured := newProblemQuerySafetyRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(tag.Columns))

	_, err := repositoryTag(repository).Find(context.Background(), &commondto.QueryOptions{
		Filters: []commondto.SearchFilter{{
			Key: "element_id", Value: uuid.NewString(), Type: commonent.SearchTypeExact,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Contains(t, query, "tag_elements", "element_id must become a typed relation predicate")
		require.NotContains(t, query, `"tags"."element_id"`, "element_id is not a physical tag column")
	}
}

func newProblemQuerySafetyRepository(t *testing.T) (*ProblemRepository, sqlmock.Sqlmock, *capturedQueries) {
	t.Helper()
	captured := &capturedQueries{}
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(captured.matcher)))
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return NewProblemRepository(dbent.WrapClient(client, nil)).(*ProblemRepository), mock, captured
}

func repositoryTag(problemRepository *ProblemRepository) *TagRepository {
	return NewTagRepository(problemRepository.client).(*TagRepository)
}

func expectEmptyProblemFind(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(problem.Columns))
}
