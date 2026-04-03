package db

import (
	"context"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/userdifficultystats"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
)

const userDiffStatsRepoName = "UserDifficultyStats"

type UserDifficultyStatsRepository struct {
	client *dbEnt.EntClient
}

func NewUserDifficultyStatsRepository(client *dbEnt.EntClient) ports.UserDifficultyStatsRepository {
	return &UserDifficultyStatsRepository{client: client}
}

func (r *UserDifficultyStatsRepository) GetByUserID(ctx context.Context, userID int) ([]*entity.UserDifficultyStats, error) {
	rows, err := r.client.DB(ctx).UserDifficultyStats.Query().
		Where(userdifficultystats.UserID(userID)).
		WithDifficulty().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userDiffStatsRepoName)
	}

	result := make([]*entity.UserDifficultyStats, len(rows))
	for i, row := range rows {
		s := &entity.UserDifficultyStats{
			UserID:       row.UserID,
			DifficultyID: row.DifficultyID,
			SolvedCount:  row.SolvedCount,
		}
		if row.Edges.Difficulty != nil {
			s.DifficultyName = row.Edges.Difficulty.Name
			s.DifficultyLevel = row.Edges.Difficulty.Level
		}
		result[i] = s
	}
	return result, nil
}

func (r *UserDifficultyStatsRepository) CreateBulk(ctx context.Context, userID int, difficultyIDs []int) error {
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

func (r *UserDifficultyStatsRepository) IncrementSolved(ctx context.Context, userID, difficultyID int) error {
	_, err := r.client.DB(ctx).UserDifficultyStats.Update().
		Where(userdifficultystats.UserID(userID), userdifficultystats.DifficultyID(difficultyID)).
		AddSolvedCount(1).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userDiffStatsRepoName)
	}
	return nil
}
