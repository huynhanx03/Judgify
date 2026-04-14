package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/conteststanding"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/ports"
)

const standingRepoName = "ContestStanding"

// StandingRepository implements ports.StandingRepository.
type StandingRepository struct {
	client *dbEnt.EntClient
}

// NewStandingRepository creates a new StandingRepository.
func NewStandingRepository(client *dbEnt.EntClient) ports.StandingRepository {
	return &StandingRepository{client: client}
}

func (r *StandingRepository) Get(ctx context.Context, contestID, userID int) (*entity.ContestStanding, error) {
	record, err := r.client.DB(ctx).ContestStanding.Query().
		Where(
			conteststanding.ContestIDEQ(contestID),
			conteststanding.UserIDEQ(userID),
		).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, standingRepoName)
	}
	return mapper.ToStandingEntity(record), nil
}

func (r *StandingRepository) Upsert(ctx context.Context, e *entity.ContestStanding) error {
	// Try to find existing standing.
	existing, err := r.client.DB(ctx).ContestStanding.Query().
		Where(
			conteststanding.ContestIDEQ(e.ContestID),
			conteststanding.UserIDEQ(e.UserID),
		).
		Only(ctx)
	if err == nil && existing != nil {
		// Update existing record.
		e.ID = existing.ID
		update := builder.BuildUpdateStanding(ctx, e)
		record, err := update.Save(ctx)
		if err != nil {
			return commonEnt.MapEntError(err, standingRepoName)
		}
		if updated := mapper.ToStandingEntity(record); updated != nil {
			*e = *updated
		}
		return nil
	}

	// Not found — create new record.
	create := builder.BuildCreateStanding(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, standingRepoName)
	}
	if created := mapper.ToStandingEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *StandingRepository) FindByContest(ctx context.Context, contestID int) ([]*entity.ContestStanding, error) {
	records, err := r.client.DB(ctx).ContestStanding.Query().
		Where(conteststanding.ContestIDEQ(contestID)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, standingRepoName)
	}

	entities := make([]*entity.ContestStanding, len(records))
	for i, record := range records {
		entities[i] = mapper.ToStandingEntity(record)
	}
	return entities, nil
}
