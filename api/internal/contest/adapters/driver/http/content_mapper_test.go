package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
)

func TestToAnnouncementResponseSerializesIDs(t *testing.T) {
	t.Parallel()
	id, contestID := uuid.New(), uuid.New()
	response := toAnnouncementResponse(&contract.AnnouncementResponse{ID: id, ContestID: contestID})
	require.Equal(t, id.String(), response.ID)
	require.Equal(t, contestID.String(), response.ContestID)
}
