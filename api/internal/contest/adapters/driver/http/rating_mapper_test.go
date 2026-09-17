package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	"github.com/huynhanx03/judgify/internal/platform/operation"
)

func TestToRatingReratingStartResponseSerializesTransportIDs(t *testing.T) {
	t.Parallel()

	triggerContestID := uuid.New()
	generationID := uuid.New()
	runChainID := uuid.New()
	response := toRatingReratingStartResponse(&contract.RatingReratingStartResponse{
		Operation:          operation.Projection{ID: uuid.New()},
		TriggerContestID:   triggerContestID,
		FromSequence:       1,
		ThroughSequence:    4,
		SourceRevision:     2,
		TargetGenerationID: generationID,
		TargetRunChainID:   runChainID,
		AlgorithmVersion:   "v1",
	})

	require.Equal(t, triggerContestID.String(), response.TriggerContestID)
	require.Equal(t, generationID.String(), response.TargetGenerationID)
	require.Equal(t, runChainID.String(), response.TargetRunChainID)
	require.Equal(t, int64(4), response.ThroughSequence)
}
