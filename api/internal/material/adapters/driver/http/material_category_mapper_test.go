package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
)

func TestToUpdateMaterialCategoryCommandParsesHTTPID(t *testing.T) {
	id := uuid.New()
	name := "Algorithms"

	actualID, command, err := toUpdateMaterialCategoryCommand(&UpdateMaterialCategoryRequest{
		ID: id.String(), Name: &name,
	})

	require.NoError(t, err)
	require.Equal(t, id, actualID)
	require.Equal(t, &name, command.Name)
}

func TestToMaterialCategoryIDRejectsInvalidHTTPID(t *testing.T) {
	_, err := toMaterialCategoryID(&materialCategoryIDRequest{ID: "not-a-uuid"})
	require.Error(t, err)
}

func TestToMaterialCategoryResponseFormatsUUID(t *testing.T) {
	id := uuid.New()
	response := toMaterialCategoryResponse(&contract.MaterialCategoryResult{
		ID: id, Name: "Algorithms", ArticleCount: 3,
	})

	require.Equal(t, id.String(), response.ID)
	require.Equal(t, "Algorithms", response.Name)
	require.Equal(t, 3, response.ArticleCount)
}
