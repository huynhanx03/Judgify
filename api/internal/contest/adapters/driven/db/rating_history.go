package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratinghistory"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/ports"
)

const ratingHistoryRepoName = "RatingHistory"

// RatingHistoryRepository implements ports.RatingHistoryRepository.
type RatingHistoryRepository struct {
	client *dbEnt.EntClient
}

// NewRatingHistoryRepository creates a new RatingHistoryRepository.
func NewRatingHistoryRepository(client *dbEnt.EntClient) ports.RatingHistoryRepository {
	return &RatingHistoryRepository{client: client}
}

// CreateBulk creates multiple rating history records in a single batch.
func (r *RatingHistoryRepository) CreateBulk(ctx context.Context, records []*entity.RatingHistory) error {
	if len(records) == 0 {
		return nil
	}

	// Build bulk create from individual builders.
	builders := make([]*generate.RatingHistoryCreate, 0, len(records))
	for _, rec := range records {
		builders = append(builders, builder.BuildCreateRatingHistory(ctx, rec))
	}

	saved, err := r.client.DB(ctx).RatingHistory.CreateBulk(builders...).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, ratingHistoryRepoName)
	}

	// Update IDs back into records.
	for i, s := range saved {
		if created := mapper.ToRatingHistoryEntity(s); created != nil {
			records[i].ID = created.ID
		}
	}
	return nil
}

// FindByContest returns all rating history records for a contest.
func (r *RatingHistoryRepository) FindByContest(ctx context.Context, contestID int) ([]*entity.RatingHistory, error) {
	records, err := r.client.DB(ctx).RatingHistory.Query().
		Where(ratinghistory.ContestIDEQ(contestID)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, ratingHistoryRepoName)
	}

	entities := make([]*entity.RatingHistory, len(records))
	for i, record := range records {
		entities[i] = mapper.ToRatingHistoryEntity(record)
	}
	return entities, nil
}

// CountByUser returns the number of rating history records for a user.
func (r *RatingHistoryRepository) CountByUser(ctx context.Context, userID int) (int, error) {
	count, err := r.client.DB(ctx).RatingHistory.Query().
		Where(ratinghistory.UserIDEQ(userID)).
		Count(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, ratingHistoryRepoName)
	}
	return count, nil
}
