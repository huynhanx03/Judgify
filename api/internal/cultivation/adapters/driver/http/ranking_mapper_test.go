package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	"github.com/huynhanx03/judgify/internal/platform/operation"
)

func TestToRankingPageResponseSerializesPublicIDs(t *testing.T) {
	t.Parallel()

	publicID := uuid.New()
	response := toRankingPageResponse(&contract.RankingPageResponse{
		Entries: []*contract.RankingEntry{{
			Rank: 1, PublicID: publicID, DisplayMode: "public", Rating: 1200,
		}},
		NextCursor:         "next",
		ProjectionRevision: 7,
	})

	require.Len(t, response.Entries, 1)
	require.Equal(t, publicID.String(), response.Entries[0].PublicID)
	require.Equal(t, "next", response.NextCursor)
	require.Equal(t, int64(7), response.ProjectionRevision)
}

func TestToRankingPrivacyResponsePreservesApplicationResult(t *testing.T) {
	t.Parallel()

	response := toRankingPrivacyResponse(&contract.RankingPrivacyResponse{
		Visibility: "anonymous", Version: 3, ProjectionRevision: 5, Replayed: true,
	})

	require.Equal(t, "anonymous", response.Visibility)
	require.Equal(t, int64(3), response.Version)
	require.True(t, response.Replayed)
}

func TestToRankingReconciliationStartResponseKeepsSharedOperationProjection(t *testing.T) {
	t.Parallel()

	response := toRankingReconciliationStartResponse(&contract.RankingReconciliationStartResponse{
		Operation:          operation.Projection{ID: uuid.New()},
		SourceCount:        10,
		RepairCount:        2,
		ProjectionRevision: 8,
		IdempotentReplay:   true,
	})

	require.Equal(t, int64(10), response.SourceCount)
	require.Equal(t, int64(8), response.ProjectionRevision)
	require.True(t, response.IdempotentReplay)
}
