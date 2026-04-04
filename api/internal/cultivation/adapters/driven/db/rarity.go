package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/rarity"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
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
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}

	entities := make([]*entity.Rarity, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToRarityEntity(rec)
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

func (r *RarityRepository) Get(ctx context.Context, id int) (*entity.Rarity, error) {
	rec, err := r.client.DB(ctx).Rarity.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, rarityRepoName)
	}
	return mapper.ToRarityEntity(rec), nil
}

func (r *RarityRepository) Create(ctx context.Context, e *entity.Rarity) error {
	rec, err := builder.BuildCreateRarity(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, rarityRepoName)
	}
	if created := mapper.ToRarityEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *RarityRepository) Update(ctx context.Context, e *entity.Rarity) error {
	rec, err := builder.BuildUpdateRarity(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, rarityRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *RarityRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Rarity.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, rarityRepoName)
	}
	return nil
}

func (r *RarityRepository) Exists(ctx context.Context, id int) (bool, error) {
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
		entities[i] = mapper.ToRarityEntity(rec)
	}
	return entities, nil
}
