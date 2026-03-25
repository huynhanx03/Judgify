package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/userelementexp"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const userElementExpRepoName = "UserElementExpRepository"

type UserElementExpRepository struct {
	client *dbEnt.EntClient
}

func NewUserElementExpRepository(client *dbEnt.EntClient) ports.UserElementExpRepository {
	return &UserElementExpRepository{client: client}
}

func (r *UserElementExpRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserElementExp], error) {
	query := r.client.DB(ctx).UserElementExp.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}

	entities := make([]*entity.UserElementExp, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToUserElementExpEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.UserElementExp]{Records: &entities, Pagination: meta}, nil
}

func (r *UserElementExpRepository) Get(ctx context.Context, id int) (*entity.UserElementExp, error) {
	rec, err := r.client.DB(ctx).UserElementExp.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return mapper.ToUserElementExpEntity(rec), nil
}

func (r *UserElementExpRepository) Create(ctx context.Context, e *entity.UserElementExp) error {
	rec, err := builder.BuildCreateUserElementExp(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	if created := mapper.ToUserElementExpEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserElementExpRepository) Update(ctx context.Context, e *entity.UserElementExp) error {
	rec, err := builder.BuildUpdateUserElementExp(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *UserElementExpRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).UserElementExp.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return nil
}

func (r *UserElementExpRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).UserElementExp.Query().Where(userelementexp.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, userElementExpRepoName)
	}
	return exists, nil
}
