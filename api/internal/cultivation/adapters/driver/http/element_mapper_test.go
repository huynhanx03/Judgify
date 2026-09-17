package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToUpdateElementCommandParsesHTTPID(t *testing.T) {
	id := uuid.New()
	name := "Fire"

	actualID, command, err := toUpdateElementCommand(&updateElementRequest{ID: id.String(), Name: &name})

	require.NoError(t, err)
	require.Equal(t, id, actualID)
	require.Equal(t, &name, command.Name)
}
