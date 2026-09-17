package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	problemContract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func TestToMaterialResponseFormatsNestedUUIDs(t *testing.T) {
	materialID := uuid.New()
	authorID := uuid.New()
	difficultyID := uuid.New()
	categoryID := uuid.New()
	tagID := uuid.New()

	response := toMaterialResponse(&contract.MaterialResponse{
		ID: materialID, AuthorID: authorID, Title: "Graph basics",
		Difficulty: &problemContract.DifficultyResult{ID: difficultyID, Name: "Easy"},
		Category:   &contract.MaterialCategoryResult{ID: categoryID, Name: "Algorithms"},
		Tags:       []*problemContract.TagResult{{ID: tagID, Name: "graphs"}},
	})

	require.Equal(t, materialID.String(), response.ID)
	require.Equal(t, authorID.String(), response.AuthorID)
	require.Equal(t, difficultyID.String(), response.Difficulty.ID)
	require.Equal(t, categoryID.String(), response.Category.ID)
	require.Equal(t, tagID.String(), response.Tags[0].ID)
}

func TestToMaterialSearchResponseMapsRecords(t *testing.T) {
	id := uuid.New()

	response := toMaterialSearchResponse(&contract.MaterialSearchResponse{
		Records: []*contract.MaterialResponse{{ID: id, Title: "Graph basics"}}, NextCursor: "next",
	})

	require.Equal(t, id.String(), response.Records[0].ID)
	require.Equal(t, "next", response.NextCursor)
}

func TestToMaterialRevisionResponseFormatsUUIDs(t *testing.T) {
	revisionID := uuid.New()
	materialID := uuid.New()
	tagID := uuid.New()

	response := toMaterialRevisionResponse(&contract.MaterialRevisionResponse{
		ID: revisionID, MaterialID: materialID, TagIDs: []uuid.UUID{tagID},
	})

	require.Equal(t, revisionID.String(), response.ID)
	require.Equal(t, materialID.String(), response.MaterialID)
	require.Equal(t, []string{tagID.String()}, response.TagIDs)
}

func TestToMaterialLifecycleReceiptFormatsUUIDs(t *testing.T) {
	materialID := uuid.New()
	commandID := uuid.New()
	eventID := uuid.New()

	response := toMaterialLifecycleReceipt(&contract.MaterialLifecycleReceipt{
		MaterialID: materialID, CommandID: commandID, EventID: eventID,
	})

	require.Equal(t, materialID.String(), response.MaterialID)
	require.Equal(t, commandID.String(), response.CommandID)
	require.Equal(t, eventID.String(), response.EventID)
}
