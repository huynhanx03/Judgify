package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToMaterialViewID(t *testing.T) {
	id := uuid.New()
	actual, err := toMaterialViewID(&RecordMaterialViewRequest{ID: id.String()})

	require.NoError(t, err)
	require.Equal(t, id, actual)
}

func TestToMaterialViewIDRejectsInvalidUUID(t *testing.T) {
	_, err := toMaterialViewID(&RecordMaterialViewRequest{ID: "invalid"})
	require.Error(t, err)
}
