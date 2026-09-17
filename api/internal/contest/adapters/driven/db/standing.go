package db

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/conteststanding"
	"github.com/huynhanx03/judgify/internal/ports"
)

const standingRepoName = "ContestStanding"

// StandingRepository implements ports.StandingRepository.
type StandingRepository struct {
	client *dbEnt.EntClient
}

// NewStandingRepository constructs a StandingRepository.
func NewStandingRepository(client *dbEnt.EntClient) ports.StandingRepository {
	return &StandingRepository{client: client}
}

func (r *StandingRepository) Get(ctx context.Context, contestID, userID uuid.UUID) (*entity.ContestStanding, error) {
	record, err := r.client.DB(ctx).ContestStanding.Query().
		Where(
			conteststanding.ContestIDEQ(contestID),
			conteststanding.UserIDEQ(userID),
		).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, standingRepoName)
	}
	return ToStandingEntity(record), nil
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
		update := BuildUpdateStanding(r.client.DB(ctx), e)
		record, err := update.Save(ctx)
		if err != nil {
			return commonEnt.MapEntError(err, standingRepoName)
		}
		if updated := ToStandingEntity(record); updated != nil {
			*e = *updated
		}
		return nil
	}
	if !generate.IsNotFound(err) {
		return commonEnt.MapEntError(err, standingRepoName)
	}

	// Not found — create new record.
	create := BuildCreateStanding(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, standingRepoName)
	}
	if created := ToStandingEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *StandingRepository) FindByContest(ctx context.Context, contestID uuid.UUID) ([]*entity.ContestStanding, error) {
	records, err := r.client.DB(ctx).ContestStanding.Query().
		Where(conteststanding.ContestIDEQ(contestID)).
		WithUser().
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, standingRepoName)
	}

	entities := make([]*entity.ContestStanding, len(records))
	for i, record := range records {
		entities[i] = ToStandingEntity(record)
	}
	return entities, nil
}

// FindByContestLimit is deliberately separate from the domain repository
// contract: public callers can detect it and keep a browser response bounded,
// while internal settlement still obtains the complete final table.
func (r *StandingRepository) FindByContestLimit(
	ctx context.Context,
	contestID uuid.UUID,
	limit int,
) ([]*entity.ContestStanding, error) {
	if limit <= 0 {
		return []*entity.ContestStanding{}, nil
	}
	records, err := r.client.DB(ctx).ContestStanding.Query().
		Where(conteststanding.ContestIDEQ(contestID)).
		WithUser().
		Order(
			conteststanding.BySolvedCount(sql.OrderDesc()),
			conteststanding.ByPenalty(sql.OrderAsc()),
			conteststanding.ByLastAcceptedElapsedMs(sql.OrderAsc()),
			conteststanding.ByUserID(sql.OrderAsc()),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, standingRepoName)
	}

	entities := make([]*entity.ContestStanding, len(records))
	for i, record := range records {
		entities[i] = ToStandingEntity(record)
	}
	return entities, nil
}
