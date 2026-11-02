package db

import (
	"context"
	"time"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

const userStatsRepoName = "User Stats"

type UserStatsRepository struct {
	client *dbEnt.EntClient
}

func NewUserStatsRepository(client *dbEnt.EntClient) ports.UserStatsRepository {
	return &UserStatsRepository{client: client}
}

func (r *UserStatsRepository) Create(ctx context.Context, e *entity.UserStats) error {
	if _, err := BuildCreateUserStats(r.client.DB(ctx), e).Save(ctx); err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	if err := synchronizeProgressionRanking(
		ctx,
		r.client.DB(ctx).UserStats,
		e.UserID,
		time.Now().UTC(),
	); err != nil {
		return commonEnt.MapEntError(err, userStatsRepoName)
	}
	return nil
}
