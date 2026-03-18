package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/trait"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const traitRepoName = "TraitRepository"

type TraitRepository struct {
	client *dbEnt.EntClient
}

func NewTraitRepository(client *dbEnt.EntClient) ports.TraitRepository {
	return &TraitRepository{client: client}
}

func (r *TraitRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Trait], error) {
	query := r.client.DB(ctx).Trait.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}

	entities := make([]*entity.Trait, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToTraitEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.Trait]{Records: &entities, Pagination: meta}, nil
}

func (r *TraitRepository) Get(ctx context.Context, id int) (*entity.Trait, error) {
	rec, err := r.client.DB(ctx).Trait.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}
	return mapper.ToTraitEntity(rec), nil
}

func (r *TraitRepository) Create(ctx context.Context, e *entity.Trait) error {
	rec, err := builder.BuildCreateTrait(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, traitRepoName)
	}
	if created := mapper.ToTraitEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *TraitRepository) Update(ctx context.Context, e *entity.Trait) error {
	rec, err := builder.BuildUpdateTrait(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, traitRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *TraitRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Trait.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, traitRepoName)
	}
	return nil
}

func (r *TraitRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Trait.Query().Where(trait.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, traitRepoName)
	}
	return exists, nil
}
