package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

func TestToRewardProfileResponseSerializesIDs(t *testing.T) {
	t.Parallel()

	profileID, userID := uuid.New(), uuid.New()
	response := toRewardProfileResponse(entity.UserRewardProfileRevision{ID: profileID, UserID: userID}, false)

	require.Equal(t, profileID.String(), response.ID)
	require.Equal(t, userID.String(), response.UserID)
}
