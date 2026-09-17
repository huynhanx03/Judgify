package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToUpdateLevelCommandParsesHTTPID(t *testing.T) {
	id := uuid.New()
	name := "Novice"

	actualID, command, err := toUpdateLevelCommand(&updateLevelRequest{ID: id.String(), Name: &name})

	require.NoError(t, err)
	require.Equal(t, id, actualID)
	require.Equal(t, &name, command.Name)
}
