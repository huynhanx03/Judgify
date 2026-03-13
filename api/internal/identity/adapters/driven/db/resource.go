package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/resource"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const resourceRepoName = "ResourceRepository"

type ResourceRepository struct {
	client *dbEnt.EntClient
}

func NewResourceRepository(client *dbEnt.EntClient) ports.ResourceRepository {
	return &ResourceRepository{client: client}
}

func (r *ResourceRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Resource], error) {
	client := r.client.DB(ctx)

	query := client.Resource.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, resourceRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, resourceRepoName)
	}

	entities := make([]*entity.Resource, len(records))
	for i, record := range records {
		entities[i] = mapper.ToResourceEntity(record)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(
		paginationOpts.Page,
		paginationOpts.PageSize,
		int64(total),
	)

	return &d.Paginated[*entity.Resource]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *ResourceRepository) Get(ctx context.Context, id int) (*entity.Resource, error) {
	record, err := r.client.DB(ctx).Resource.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, resourceRepoName)
	}
	return mapper.ToResourceEntity(record), nil
}

func (r *ResourceRepository) Create(ctx context.Context, e *entity.Resource) error {
	create := builder.BuildCreateResource(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, resourceRepoName)
	}

	if created := mapper.ToResourceEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *ResourceRepository) Update(ctx context.Context, e *entity.Resource) error {
	update := builder.BuildUpdateResource(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, resourceRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *ResourceRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Resource.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, resourceRepoName)
	}
	return nil
}

func (r *ResourceRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Resource.Query().Where(resource.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, resourceRepoName)
}

func (r *ResourceRepository) FindByIDs(ctx context.Context, ids []int) ([]*entity.Resource, error) {
	records, err := r.client.DB(ctx).Resource.Query().
		Where(resource.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, resourceRepoName)
	}

	entities := make([]*entity.Resource, len(records))
	for i, m := range records {
		entities[i] = mapper.ToResourceEntity(m)
	}
	return entities, nil
}
