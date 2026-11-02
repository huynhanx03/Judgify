package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestApplyVerdictPreservesDecodedWrongAttemptsAndIgnoresPostAC(t *testing.T) {
	problemID := uuid.MustParse("00000000-0000-7000-8000-000000000021")
	standing := &ContestStanding{
		ContestID: uuid.MustParse("00000000-0000-7000-8000-000000000011"),
		UserID:    uuid.MustParse("00000000-0000-7000-8000-000000000031"),
		ProblemResults: map[string]any{
			problemID.String(): map[string]any{"ac": false, "wa_count": float64(2), "time": float64(0)},
		},
	}
	require.NoError(t, ApplyVerdict(standing, problemID, true, 300))
	require.Equal(t, 1, standing.SolvedCount)
	require.Equal(t, 300+2*ICPCWrongAttemptPenaltySeconds, standing.Penalty)

	require.NoError(t, ApplyVerdict(standing, problemID, false, 500))
	require.Equal(t, 1, standing.SolvedCount)
	require.Equal(t, 300+2*ICPCWrongAttemptPenaltySeconds, standing.Penalty)
}
