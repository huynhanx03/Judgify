package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestregistration"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/contest/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/ports"
)

const registrationRepoName = "ContestRegistration"

// RegistrationRepository implements ports.RegistrationRepository.
type RegistrationRepository struct {
	client *dbEnt.EntClient
}

// NewRegistrationRepository creates a new RegistrationRepository.
func NewRegistrationRepository(client *dbEnt.EntClient) ports.RegistrationRepository {
	return &RegistrationRepository{client: client}
}

func (r *RegistrationRepository) Create(ctx context.Context, contestID, userID int) error {
	create := builder.BuildCreateRegistration(ctx, contestID, userID)
	if _, err := create.Save(ctx); err != nil {
		return commonEnt.MapEntError(err, registrationRepoName)
	}
	return nil
}

func (r *RegistrationRepository) Delete(ctx context.Context, contestID, userID int) error {
	_, err := r.client.DB(ctx).ContestRegistration.Delete().
		Where(
			contestregistration.ContestIDEQ(contestID),
			contestregistration.UserIDEQ(userID),
		).
		Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, registrationRepoName)
	}
	return nil
}

func (r *RegistrationRepository) Exists(ctx context.Context, contestID, userID int) (bool, error) {
	exists, err := r.client.DB(ctx).ContestRegistration.Query().
		Where(
			contestregistration.ContestIDEQ(contestID),
			contestregistration.UserIDEQ(userID),
		).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, registrationRepoName)
	}
	return exists, nil
}

func (r *RegistrationRepository) CountByContest(ctx context.Context, contestID int) (int, error) {
	count, err := r.client.DB(ctx).ContestRegistration.Query().
		Where(contestregistration.ContestIDEQ(contestID)).
		Count(ctx)
	if err != nil {
		return 0, commonEnt.MapEntError(err, registrationRepoName)
	}
	return count, nil
}

func (r *RegistrationRepository) FindByContest(ctx context.Context, contestID int) ([]*entity.ContestRegistration, error) {
	records, err := r.client.DB(ctx).ContestRegistration.Query().
		Where(contestregistration.ContestIDEQ(contestID)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, registrationRepoName)
	}

	entities := make([]*entity.ContestRegistration, len(records))
	for i, record := range records {
		entities[i] = mapper.ToRegistrationEntity(record)
	}
	return entities, nil
}
