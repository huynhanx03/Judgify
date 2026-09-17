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
	"github.com/huynhanx03/judgify/internal/ent/generate/material"
)

type capturedMaterialQueries struct {
	mu      sync.Mutex
	queries []string
}

func (c *capturedMaterialQueries) matcher(_, actual string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = append(c.queries, actual)
	return nil
}

func (c *capturedMaterialQueries) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries...)
}

func TestMaterialFindPublicPreservesSearchAndCategoryWithoutVirtualColumn(t *testing.T) {
	repository, mock, captured := newMaterialQuerySafetyRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(material.Columns))
	categoryID := uuid.New()

	_, err := repository.FindPublic(context.Background(), &commondto.QueryOptions{
		Filters: []commondto.SearchFilter{
			{Key: "search", Value: "binary tree", Type: commonent.SearchTypeSearch},
			{Key: material.FieldCategoryID, Value: categoryID.String(), Type: commonent.SearchTypeExact},
		},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Contains(t, query, material.FieldCategoryID)
		require.Contains(t, query, material.FieldTitle)
		require.Contains(t, query, material.FieldDescription)
		require.NotContains(t, query, `"search"`, "search is a virtual material filter")
	}
	require.NotContains(t, strings.ToUpper(queries[0]), "LIMIT")
	require.Contains(t, strings.ToUpper(queries[1]), "LIMIT 10")
}

func TestMaterialFindAdminMapsGenericNameSearchToArticleText(t *testing.T) {
	repository, mock, captured := newMaterialQuerySafetyRepository(t)
	mock.ExpectQuery("count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("records").WillReturnRows(sqlmock.NewRows(material.Columns))

	_, err := repository.Find(context.Background(), &commondto.QueryOptions{
		Filters: []commondto.SearchFilter{{
			Key: "name", Value: "dynamic programming", Type: commonent.SearchTypeSearch,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	queries := captured.snapshot()
	require.Len(t, queries, 2)
	for _, query := range queries {
		require.Contains(t, query, material.FieldTitle)
		require.Contains(t, query, material.FieldDescription)
		require.NotContains(t, query, `"name"`, "generic admin search is an alias, not a material column")
	}
}

func newMaterialQuerySafetyRepository(t *testing.T) (*MaterialRepository, sqlmock.Sqlmock, *capturedMaterialQueries) {
	t.Helper()
	captured := &capturedMaterialQueries{}
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(captured.matcher)))
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return NewMaterialRepository(dbent.WrapClient(client, nil)).(*MaterialRepository), mock, captured
}
