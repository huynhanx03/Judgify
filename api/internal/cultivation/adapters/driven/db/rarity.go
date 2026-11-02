package db

import (
	"context"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/rarity"
	"github.com/huynhanx03/judgify/internal/ports"
)

const rarityRepoName = "Rarity"

type RarityRepository struct {
	client *dbEnt.EntClient
}

func NewRarityRepository(client *dbEnt.EntClient) ports.RarityRepository {
	return &RarityRepository{client: client}
}

func (r *RarityRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Rarity], error) {
	query := r.client.DB(ctx).Rarity.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, rarityQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, rarityQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}

	entities := make([]*entity.Rarity, len(records))
	for i, rec := range records {
		entities[i] = ToRarityEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.Rarity]{Records: &entities, Pagination: meta}, nil
}

func (r *RarityRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Rarity, error) {
	rec, err := r.client.DB(ctx).Rarity.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}
	return ToRarityEntity(rec), nil
}

func (r *RarityRepository) Create(ctx context.Context, e *entity.Rarity) error {
	rec, err := BuildCreateRarity(r.client.DB(ctx), e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, rarityRepoName)
	}
	if created := ToRarityEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *RarityRepository) Update(ctx context.Context, e *entity.Rarity) error {
	rec, err := BuildUpdateRarity(r.client.DB(ctx), e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, rarityRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *RarityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Rarity.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, rarityRepoName)
	}
	return nil
}

func (r *RarityRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Rarity.Query().Where(rarity.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, rarityRepoName)
	}
	return exists, nil
}

func (r *RarityRepository) FindAll(ctx context.Context) ([]*entity.Rarity, error) {
	records, err := r.client.DB(ctx).Rarity.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}
	entities := make([]*entity.Rarity, len(records))
	for i, rec := range records {
		entities[i] = ToRarityEntity(rec)
	}
	return entities, nil
}
