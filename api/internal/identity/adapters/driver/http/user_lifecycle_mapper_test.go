package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func TestToUserLifecyclePreviewResponseKeepsIDsAtHTTPBoundary(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	value := &dto.UserLifecyclePreviewResponse{
		Preview: dto.UserLifecyclePreview{
			Action: "suspend", UserID: userID, ExpectedVersion: 2, ResultingVersion: 3,
			Before: dto.UserLifecycleProjection{ID: userID, Username: "judgify", Status: "active", Version: 2},
			After:  dto.UserLifecycleProjection{ID: userID, Username: "judgify", Status: "suspended", Version: 3},
		},
		ConfirmationToken: "confirmation", ExpiresAt: time.Now().UTC(),
	}

	result := toUserLifecyclePreviewResponse(value)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(body), `"user_id":"`+userID.String()+`"`)
	require.Contains(t, string(body), `"status":"suspended"`)
}

func TestToUserLifecycleCommandRejectsInvalidTransportID(t *testing.T) {
	t.Parallel()
	_, err := toUserLifecycleCommand(&userLifecycleCommandRequest{ID: "not-a-uuid", CommandID: uuid.NewString()})
	require.Error(t, err)
}
