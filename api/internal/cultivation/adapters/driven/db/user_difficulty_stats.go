package db

import (
	"context"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ports"
)

const userDiffStatsRepoName = "UserDifficultyStats"

type UserDifficultyStatsRepository struct {
	client *dbEnt.EntClient
}

func NewUserDifficultyStatsRepository(client *dbEnt.EntClient) ports.UserDifficultyStatsRepository {
	return &UserDifficultyStatsRepository{client: client}
}

func (r *UserDifficultyStatsRepository) CreateBulk(ctx context.Context, userID uuid.UUID, difficultyIDs []uuid.UUID) error {
	builders := make([]*generate.UserDifficultyStatsCreate, len(difficultyIDs))
	for i, diffID := range difficultyIDs {
		builders[i] = r.client.DB(ctx).UserDifficultyStats.Create().
			SetUserID(userID).
			SetDifficultyID(diffID).
			SetSolvedCount(0)
	}
	err := r.client.DB(ctx).UserDifficultyStats.CreateBulk(builders...).
		OnConflictColumns("user_id", "difficulty_id").
		DoNothing().
		Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userDiffStatsRepoName)
	}
	return nil
}
