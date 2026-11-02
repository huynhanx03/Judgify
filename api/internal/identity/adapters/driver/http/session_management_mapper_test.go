package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func TestToSessionListResponseSerializesIdentityAtHTTPBoundary(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	result := toSessionListResponse(&dto.SessionListResponse{Items: []dto.SessionResponse{{
		ID: id, CreatedAt: now, LastSeenAt: now, LastReauthenticatedAt: now,
		IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
	}}, NextCursor: "next"})

	require.Equal(t, id.String(), result.Items[0].ID)
	body, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(body), `"id":"`+id.String()+`"`)
	require.Contains(t, string(body), `"next_cursor":"next"`)
}
