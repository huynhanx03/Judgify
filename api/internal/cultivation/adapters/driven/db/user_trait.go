package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/usertrait"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const userTraitRepoName = "UserTraitRepository"

type UserTraitRepository struct {
	client *dbEnt.EntClient
}

func NewUserTraitRepository(client *dbEnt.EntClient) ports.UserTraitRepository {
	return &UserTraitRepository{client: client}
}

func (r *UserTraitRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserTrait], error) {
	query := r.client.DB(ctx).UserTrait.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTraitRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTraitRepoName)
	}

	entities := make([]*entity.UserTrait, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToUserTraitEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.UserTrait]{Records: &entities, Pagination: meta}, nil
}

func (r *UserTraitRepository) Get(ctx context.Context, id int) (*entity.UserTrait, error) {
	rec, err := r.client.DB(ctx).UserTrait.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTraitRepoName)
	}
	return mapper.ToUserTraitEntity(rec), nil
}

func (r *UserTraitRepository) Create(ctx context.Context, e *entity.UserTrait) error {
	rec, err := builder.BuildCreateUserTrait(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userTraitRepoName)
	}
	if created := mapper.ToUserTraitEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserTraitRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).UserTrait.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, userTraitRepoName)
	}
	return nil
}

func (r *UserTraitRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).UserTrait.Query().Where(usertrait.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, userTraitRepoName)
	}
	return exists, nil
}
