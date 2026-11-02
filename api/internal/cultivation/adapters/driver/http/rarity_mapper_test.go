package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToUpdateRarityCommandParsesHTTPID(t *testing.T) {
	id := uuid.New()
	weight := 10

	actualID, command, err := toUpdateRarityCommand(&updateRarityRequest{ID: id.String(), Weight: &weight})

	require.NoError(t, err)
	require.Equal(t, id, actualID)
	require.Equal(t, &weight, command.Weight)
}
