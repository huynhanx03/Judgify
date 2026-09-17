package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToNotificationIDParsesHTTPID(t *testing.T) {
	id := uuid.New()

	actual, err := toNotificationID(&notificationIDRequest{ID: id.String()})

	require.NoError(t, err)
	require.Equal(t, id, actual)
}

func TestToNotificationIDRejectsInvalidHTTPID(t *testing.T) {
	_, err := toNotificationID(&notificationIDRequest{ID: "invalid"})
	require.Error(t, err)
}
