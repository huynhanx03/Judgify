package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToUpdateRankCommandParsesHTTPID(t *testing.T) {
	id := uuid.New()
	name := "Silver"

	actualID, command, err := toUpdateRankCommand(&updateRankRequest{ID: id.String(), Name: &name})

	require.NoError(t, err)
	require.Equal(t, id, actualID)
	require.Equal(t, &name, command.Name)
}
