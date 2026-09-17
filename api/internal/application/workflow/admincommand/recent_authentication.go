package admincommand

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/authctx"
)

var ErrRecentAuthenticationRequired = errors.New(
	"admin command: recent authentication required",
)

// RequireRecentAuthentication proves that the reviewed command is being
// applied by the same live database-backed session and that its original
// authentication ceremony is still inside the configured freshness window.
// It grants no permission; callers must authorize the action independently.
func RequireRecentAuthentication(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	now time.Time,
	ttl time.Duration,
) error {
	if ctx == nil || actorID == uuid.Nil || sessionID == uuid.Nil ||
		now.IsZero() || ttl < time.Minute || ttl > time.Hour {
		return ErrRecentAuthenticationRequired
	}
	principal, ok := authctx.Principal(ctx)
	if !ok || principal.Subject != actorID.String() ||
		principal.SessionID != sessionID.String() ||
		!principal.RecentlyAuthenticated(now.UTC(), ttl) {
		return ErrRecentAuthenticationRequired
	}
	return nil
}
