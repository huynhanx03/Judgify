package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
)

func TestToContestPageSerializesIDsAtHTTPBoundary(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	page := toContestPage(&d.Paginated[*contract.ContestResponse]{Records: &[]*contract.ContestResponse{{ID: id, AuthorID: id, StartTime: time.Now().UTC(), EndTime: time.Now().UTC()}}})
	body, err := json.Marshal(page)
	require.NoError(t, err)
	require.Contains(t, string(body), `"id":"`+id.String()+`"`)
	require.Contains(t, string(body), `"author_id":"`+id.String()+`"`)
}

func TestToContestProblemDetailResponseSerializesNestedIDs(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	result := toContestProblemDetailResponse(&contract.ContestProblemDetailResponse{ContestID: id, ContestProblemID: id, ProblemID: id, Difficulty: contract.ContestProblemDifficultyResult{ID: id}, Tags: []contract.ContestProblemTagResponse{{ID: id}}})
	require.Equal(t, id.String(), result.Difficulty.ID)
	require.Equal(t, id.String(), result.Tags[0].ID)
}
