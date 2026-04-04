package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const elementRepoName = "Element"

type ElementRepository struct {
	client *dbEnt.EntClient
}

func NewElementRepository(client *dbEnt.EntClient) ports.ElementRepository {
	return &ElementRepository{client: client}
}

func (r *ElementRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Element], error) {
	query := r.client.DB(ctx).Element.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}

	entities := make([]*entity.Element, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToElementEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.Element]{Records: &entities, Pagination: meta}, nil
}

func (r *ElementRepository) Get(ctx context.Context, id int) (*entity.Element, error) {
	rec, err := r.client.DB(ctx).Element.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}
	return mapper.ToElementEntity(rec), nil
}

func (r *ElementRepository) Create(ctx context.Context, e *entity.Element) error {
	rec, err := builder.BuildCreateElement(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, elementRepoName)
	}
	if created := mapper.ToElementEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *ElementRepository) Update(ctx context.Context, e *entity.Element) error {
	rec, err := builder.BuildUpdateElement(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, elementRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *ElementRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Element.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, elementRepoName)
	}
	return nil
}

func (r *ElementRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Element.Query().Where(element.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, elementRepoName)
	}
	return exists, nil
}

func (r *ElementRepository) FindAll(ctx context.Context) ([]*entity.Element, error) {
	records, err := r.client.DB(ctx).Element.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}
	entities := make([]*entity.Element, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToElementEntity(rec)
	}
	return entities, nil
}
