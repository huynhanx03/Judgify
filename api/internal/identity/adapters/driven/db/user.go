package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const userRepoName = "UserRepository"

type UserRepository struct {
	client *dbEnt.EntClient
}

func NewUserRepository(client *dbEnt.EntClient) ports.UserRepository {
	return &UserRepository{client: client}
}

func (r *UserRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.User], error) {
	client := r.client.DB(ctx)

	query := client.User.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}

	entities := make([]*entity.User, len(records))
	for i, record := range records {
		entities[i] = mapper.ToUserEntity(record)
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

	return &d.Paginated[*entity.User]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *UserRepository) Get(ctx context.Context, id int) (*entity.User, error) {
	record, err := r.client.DB(ctx).User.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}
	return mapper.ToUserEntity(record), nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	record, err := r.client.DB(ctx).User.Query().
		Where(user.Username(username)).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}
	return mapper.ToUserEntity(record), nil
}

func (r *UserRepository) Create(ctx context.Context, e *entity.User) error {
	create := builder.BuildCreateUser(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userRepoName)
	}

	if created := mapper.ToUserEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, e *entity.User) error {
	update := builder.BuildUpdateUser(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id int) error {
	return commonEnt.MapEntError(r.client.DB(ctx).User.DeleteOneID(id).Exec(ctx), userRepoName)
}

func (r *UserRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).User.Query().Where(user.ID(id)).Exist(ctx)
	if err != nil {
		return exists, commonEnt.MapEntError(err, userRepoName)
	}
	return exists, nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	exists, err := r.client.DB(ctx).User.Query().Where(user.Username(username)).Exist(ctx)
	if err != nil {
		return exists, commonEnt.MapEntError(err, userRepoName)
	}
	return exists, nil
}
