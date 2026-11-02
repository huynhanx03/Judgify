package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func TestToSubmissionResponseSerializesIdentifiers(t *testing.T) {
	t.Parallel()
	id, problemID, userID := uuid.New(), uuid.New(), uuid.New()
	response := toSubmissionResponse(&contract.SubmissionResponse{ID: id, ProblemID: problemID, UserID: userID})
	require.Equal(t, id.String(), response.ID)
	require.Equal(t, problemID.String(), response.ProblemID)
	require.Equal(t, userID.String(), response.UserID)
}
