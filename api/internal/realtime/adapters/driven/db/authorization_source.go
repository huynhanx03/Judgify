package db

import (
	"context"
	"fmt"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/authorizationstate"
	"github.com/huynhanx03/judgify/internal/realtime"
)

const authorizationStateID = "default"

// AuthorizationSource reads the migration-owned singleton revision fence.
// PostgreSQL visibility makes this a post-commit signal; no process-local
// callback or best-effort publication can produce an event for a rollback.
type AuthorizationSource struct {
	client *internalent.EntClient
}

func NewAuthorizationSource(client *internalent.EntClient) (*AuthorizationSource, error) {
	if client == nil || client.Client() == nil {
		return nil, realtime.ErrInvalidConfig
	}
	return &AuthorizationSource{client: client}, nil
}

func (source *AuthorizationSource) Current(
	ctx context.Context,
) (realtime.AuthorizationRevisionChange, error) {
	if source == nil || source.client == nil || ctx == nil {
		return realtime.AuthorizationRevisionChange{}, realtime.ErrInvalidConfig
	}
	record, err := source.client.DB(ctx).AuthorizationState.Query().
		Where(authorizationstate.ID(authorizationStateID)).
		Select(
			authorizationstate.FieldDesiredRevision,
			authorizationstate.FieldUpdatedAt,
		).
		Only(ctx)
	if err != nil {
		return realtime.AuthorizationRevisionChange{}, fmt.Errorf(
			"query authorization realtime revision: %w",
			err,
		)
	}
	change := realtime.AuthorizationRevisionChange{
		Event: realtime.AuthorizationCapabilitiesChangedV1{
			Revision:  record.DesiredRevision,
			ChangedAt: record.UpdatedAt.UTC(),
		},
	}
	if err := change.Validate(); err != nil {
		return realtime.AuthorizationRevisionChange{}, err
	}
	return change, nil
}
