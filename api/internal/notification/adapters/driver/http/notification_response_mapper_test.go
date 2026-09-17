package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
)

func TestToNotificationResponseSerializesID(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	response := toNotificationResponse(&contract.NotificationResponse{ID: id})
	require.Equal(t, id.String(), response.ID)
}
