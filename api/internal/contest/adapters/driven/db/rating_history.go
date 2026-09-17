package db

import (
	"context"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/ratinghistory"
	"github.com/huynhanx03/judgify/internal/ports"
)

const ratingHistoryRepoName = "RatingHistory"

// RatingHistoryRepository implements ports.RatingHistoryRepository.
type RatingHistoryRepository struct {
	client *dbEnt.EntClient
}

// NewRatingHistoryRepository constructs a RatingHistoryRepository.
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
		builders = append(builders, BuildCreateRatingHistory(r.client.DB(ctx), rec))
	}

	saved, err := r.client.DB(ctx).RatingHistory.CreateBulk(builders...).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, ratingHistoryRepoName)
	}

	// Update IDs back into records.
	for i, s := range saved {
		if created := ToRatingHistoryEntity(s); created != nil {
			records[i].ID = created.ID
		}
	}
	return nil
}

// FindByContest returns all rating history records for a contest.
func (r *RatingHistoryRepository) FindByContest(ctx context.Context, contestID uuid.UUID) ([]*entity.RatingHistory, error) {
	records, err := r.client.DB(ctx).RatingHistory.Query().
		Where(ratinghistory.ContestIDEQ(contestID)).
		WithUser().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, ratingHistoryRepoName)
	}

	entities := make([]*entity.RatingHistory, len(records))
	for i, record := range records {
		entities[i] = ToRatingHistoryEntity(record)
	}
	return entities, nil
}

// CountByUser returns the number of rating history records for a user.
func (r *RatingHistoryRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := r.client.DB(ctx).RatingHistory.Query().
		Where(ratinghistory.UserIDEQ(userID)).
		Count(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, ratingHistoryRepoName)
	}
	return count, nil
}

// CountByUsers returns rating history counts keyed by user_id.
func (r *RatingHistoryRepository) CountByUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	counts := make(map[uuid.UUID]int, len(userIDs))
	if len(userIDs) == 0 {
		return counts, nil
	}
	for _, userID := range userIDs {
		counts[userID] = 0
	}

	var rows []struct {
		UserID uuid.UUID `json:"user_id"`
		Count  int       `json:"count"`
	}
	err := r.client.DB(ctx).RatingHistory.Query().
		Where(ratinghistory.UserIDIn(userIDs...)).
		GroupBy(ratinghistory.FieldUserID).
		Aggregate(generate.As(generate.Count(), "count")).
		Scan(ctx, &rows)
	if err != nil {
		return nil, commonEnt.MapEntError(err, ratingHistoryRepoName)
	}

	for _, row := range rows {
		counts[row.UserID] = row.Count
	}
	return counts, nil
}
