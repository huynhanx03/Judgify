package db

import (
	"context"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ports"
)

const userTraitRepoName = "User Trait"

type UserTraitRepository struct {
	client *dbEnt.EntClient
}

func NewUserTraitRepository(client *dbEnt.EntClient) ports.UserTraitRepository {
	return &UserTraitRepository{client: client}
}

func (r *UserTraitRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserTrait], error) {
	query := r.client.DB(ctx).UserTrait.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, userTraitQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTraitRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, userTraitQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTraitRepoName)
	}

	entities := make([]*entity.UserTrait, len(records))
	for i, rec := range records {
		entities[i] = ToUserTraitEntity(rec)
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

func (r *UserTraitRepository) Get(ctx context.Context, id uuid.UUID) (*entity.UserTrait, error) {
	rec, err := r.client.DB(ctx).UserTrait.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userTraitRepoName)
	}
	return ToUserTraitEntity(rec), nil
}
