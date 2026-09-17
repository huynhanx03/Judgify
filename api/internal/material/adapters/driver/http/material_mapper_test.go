package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
)

func TestToCreateMaterialCommandParsesHTTPIDs(t *testing.T) {
	commandID := uuid.New()
	categoryID := uuid.New()
	difficultyID := uuid.New()
	tagID := uuid.New()
	actual, err := toCreateMaterialCommand(&createMaterialRequest{
		CommandID: commandID.String(), Slug: "graph-basics", Title: "Graph basics",
		CategoryID: categoryID.String(), DifficultyID: difficultyID.String(), TagIDs: []string{tagID.String()},
		ChangeSummary: "initial draft",
	})

	require.NoError(t, err)
	require.Equal(t, commandID, actual.CommandID)
	require.Equal(t, categoryID, actual.CategoryID)
	require.Equal(t, difficultyID, actual.DifficultyID)
	require.Equal(t, []uuid.UUID{tagID}, actual.TagIDs)
}

func TestToMaterialSearchQueryParsesHTTPFilters(t *testing.T) {
	categoryID := uuid.New()
	difficultyID := uuid.New()
	tagID := uuid.New()
	category := categoryID.String()
	difficulty := difficultyID.String()

	actual, err := toMaterialSearchQuery(&materialSearchRequest{
		CategoryID: &category, DifficultyID: &difficulty, TagIDs: []string{tagID.String()},
		Text: "graphs", Sort: "relevance", Limit: 10,
	}, true)

	require.NoError(t, err)
	require.Equal(t, &categoryID, actual.CategoryID)
	require.Equal(t, &difficultyID, actual.DifficultyID)
	require.Equal(t, []uuid.UUID{tagID}, actual.TagIDs)
	require.True(t, actual.Authenticated)
	require.Equal(t, ports.SearchQuery{Text: "graphs", CategoryID: &categoryID, DifficultyID: &difficultyID, TagIDs: []uuid.UUID{tagID}, Sort: "relevance", Limit: 10, Authenticated: true}, actual)
}

func TestToUpdateMaterialCommandParsesHTTPID(t *testing.T) {
	materialID := uuid.New()
	commandID := uuid.New()
	title := "Revised graph basics"

	actualID, actual, err := toUpdateMaterialCommand(&updateMaterialRequest{
		ID: materialID.String(), CommandID: commandID.String(), Title: &title,
		ExpectedVersion: 2, ChangeSummary: "clarify traversal",
	})

	require.NoError(t, err)
	require.Equal(t, materialID, actualID)
	require.Equal(t, commandID, actual.CommandID)
	require.Equal(t, &title, actual.Title)
}
