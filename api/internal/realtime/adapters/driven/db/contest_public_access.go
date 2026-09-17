package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/realtime"
)

// ContestPublicAccess owns the realtime form of the same fail-closed
// publication predicate enforced by contest public REST reads.
type ContestPublicAccess struct {
	client *internalent.EntClient
}

func NewContestPublicAccess(
	client *internalent.EntClient,
) (*ContestPublicAccess, error) {
	if client == nil || client.Client() == nil {
		return nil, realtime.ErrInvalidConfig
	}
	return &ContestPublicAccess{client: client}, nil
}

func (access *ContestPublicAccess) IsPubliclyVisible(
	ctx context.Context,
	contestID uuid.UUID,
) (bool, error) {
	if access == nil || access.client == nil || ctx == nil || contestID == uuid.Nil {
		return false, realtime.ErrInvalidConfig
	}
	visible, err := access.client.DB(ctx).Contest.Query().Where(
		contest.IDEQ(contestID),
		contest.StatusNEQ(contest.StatusDraft),
		contest.PublishedAtNotNil(),
	).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("query realtime contest visibility: %w", err)
	}
	return visible, nil
}
