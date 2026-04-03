package db

import (
	"context"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/usertagstats"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
)

const userTagStatsRepoName = "UserTagStats"

type UserTagStatsRepository struct {
	client *dbEnt.EntClient
}

func NewUserTagStatsRepository(client *dbEnt.EntClient) ports.UserTagStatsRepository {
	return &UserTagStatsRepository{client: client}
}

func (r *UserTagStatsRepository) GetByUserID(ctx context.Context, userID int) ([]*entity.UserTagStats, error) {
	rows, err := r.client.DB(ctx).UserTagStats.Query().
		Where(usertagstats.UserID(userID)).
		WithTag(func(tq *generate.TagQuery) {
			tq.WithElements()
		}).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTagStatsRepoName)
	}

	result := make([]*entity.UserTagStats, len(rows))
	for i, row := range rows {
		s := &entity.UserTagStats{
			UserID:      row.UserID,
			TagID:       row.TagID,
			SolvedCount: row.SolvedCount,
			Elements:    []entity.ElementBrief{},
		}
		if row.Edges.Tag != nil {
			s.TagName = row.Edges.Tag.Name
			for _, elem := range row.Edges.Tag.Edges.Elements {
				s.Elements = append(s.Elements, entity.ElementBrief{
					Code: elem.Code,
					Name: elem.Name,
				})
			}
		}
		result[i] = s
	}
	return result, nil
}

func (r *UserTagStatsRepository) IncrementSolved(ctx context.Context, userID, tagID int) error {
	err := r.client.DB(ctx).UserTagStats.Create().
		SetUserID(userID).
		SetTagID(tagID).
		SetSolvedCount(1).
		OnConflictColumns("user_id", "tag_id").
		AddSolvedCount(1).
		Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userTagStatsRepoName)
	}
	return nil
}
