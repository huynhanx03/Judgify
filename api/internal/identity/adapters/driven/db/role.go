package db

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"go.uber.org/zap"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/role"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const roleRepoName = "Role"

type RoleRepository struct {
	client *dbEnt.EntClient
}

func NewRoleRepository(client *dbEnt.EntClient) ports.RoleRepository {
	return &RoleRepository{client: client}
}

func (r *RoleRepository) FindAll(ctx context.Context) ([]*entity.Role, error) {
	records, err := r.client.DB(ctx).Role.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	entities := make([]*entity.Role, len(records))
	for i, m := range records {
		entities[i] = mapper.ToRoleEntity(m)
	}
	return entities, nil
}

func (r *RoleRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Role], error) {
	client := r.client.DB(ctx)

	query := client.Role.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	entities := make([]*entity.Role, len(records))
	for i, record := range records {
		entities[i] = mapper.ToRoleEntity(record)
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

	return &d.Paginated[*entity.Role]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *RoleRepository) Get(ctx context.Context, id int) (*entity.Role, error) {
	record, err := r.client.DB(ctx).Role.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}
	return mapper.ToRoleEntity(record), nil
}

func (r *RoleRepository) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	record, err := r.client.DB(ctx).Role.Query().
		Where(role.Name(name)).
		Only(ctx)
	if err != nil {
		logger.FromContext(ctx).Error("RoleRepository.GetByName", zap.String("name", name), zap.Error(err))
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}
	return mapper.ToRoleEntity(record), nil
}

func (r *RoleRepository) Create(ctx context.Context, e *entity.Role) error {
	create := builder.BuildCreateRole(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, roleRepoName)
	}

	if created := mapper.ToRoleEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *RoleRepository) Update(ctx context.Context, e *entity.Role) error {
	update := builder.BuildUpdateRole(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, roleRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *RoleRepository) UpdateBulk(ctx context.Context, entities []*entity.Role) error {
	builders := make([]*generate.RoleCreate, len(entities))
	for i, e := range entities {
		builders[i] = builder.BuildCreateRole(ctx, e)
	}

	err := r.client.DB(ctx).Role.CreateBulk(builders...).
		OnConflict().
		UpdateNewValues().
		Exec(ctx)
	return commonEnt.MapEntError(err, roleRepoName)
}

func (r *RoleRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Role.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, roleRepoName)
	}
	return nil
}

func (r *RoleRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Role.Query().Where(role.ID(id)).Exist(ctx)

	if err != nil {
		return false, commonEnt.MapEntError(err, roleRepoName)
	}

	return exists, nil
}

func (r *RoleRepository) FindDescendants(ctx context.Context, lft, rgt int) ([]*entity.Role, error) {
	records, err := r.client.DB(ctx).Role.Query().
		Where(role.LftGTE(lft), role.RgtLTE(rgt)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	entities := make([]*entity.Role, len(records))
	for i, m := range records {
		entities[i] = mapper.ToRoleEntity(m)
	}
	return entities, nil
}
