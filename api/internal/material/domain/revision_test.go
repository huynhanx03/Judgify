package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSlugIsBoundedAndCanonical(t *testing.T) {
	slug, err := NormalizeSlug("  Cay-Doan_Co-Ban ")
	require.NoError(t, err)
	require.Equal(t, "cay-doan-co-ban", slug)

	_, err = NormalizeSlug("không-an-toàn")
	require.ErrorIs(t, err, ErrMaterialInvalidCommand)
}

func TestEstimatedReadMinutesRoundsUp(t *testing.T) {
	require.Equal(t, 1, EstimatedReadMinutes([]byte("một hai ba")))
	words := make([]byte, 0, 201*2)
	for range 201 {
		words = append(words, 'a', ' ')
	}
	require.Equal(t, 2, EstimatedReadMinutes(words))
}
