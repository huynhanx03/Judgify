package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
)

func TestToStandingsResponseSerializesBoundaryIDs(t *testing.T) {
	t.Parallel()

	contestID := uuid.New()
	userID := uuid.New()
	frozenAt := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	response := toStandingsResponse(&contract.StandingsResponse{
		ContestID:         contestID,
		View:              "public",
		ProjectionVersion: 3,
		LastCursor:        "cursor",
		FrozenAt:          &frozenAt,
		Items: []*contract.StandingResponse{{
			Rank:             1,
			UserID:           userID,
			Username:         "alice",
			SolvedCount:      2,
			Penalty:          42,
			ProblemResults:   map[string]any{"a": "accepted"},
			PendingAttempts:  1,
			PendingByProblem: map[string]int{"b": 1},
		}},
	})

	require.Equal(t, contestID.String(), response.ContestID)
	require.Equal(t, "public", response.View)
	require.Equal(t, int64(3), response.ProjectionVersion)
	require.Equal(t, &frozenAt, response.FrozenAt)
	require.Len(t, response.Items, 1)
	require.Equal(t, userID.String(), response.Items[0].UserID)
	require.Equal(t, "accepted", response.Items[0].ProblemResults["a"])
	require.Equal(t, 1, response.Items[0].PendingByProblem["b"])
}

func TestToSelfStandingResponseKeepsAbsentStandingAbsent(t *testing.T) {
	t.Parallel()

	contestID := uuid.New()
	response := toSelfStandingResponse(&contract.SelfStandingResponse{
		ContestID:         contestID,
		ProjectionVersion: 4,
		LastCursor:        "cursor",
	})

	require.Equal(t, contestID.String(), response.ContestID)
	require.Nil(t, response.Item)
}
