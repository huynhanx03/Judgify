package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToCreateTraitCommandParsesRarityID(t *testing.T) {
	rarityID := uuid.New()

	command, err := toCreateTraitCommand(&createTraitRequest{RarityID: rarityID.String()})

	require.NoError(t, err)
	require.Equal(t, rarityID, command.RarityID)
}
