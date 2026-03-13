package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/permission"
	"github.com/huynhanx03/judgify/internal/identity/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const permissionRepoName = "PermissionRepository"

type PermissionRepository struct {
	client *dbEnt.EntClient
}

func NewPermissionRepository(client *dbEnt.EntClient) ports.PermissionRepository {
	return &PermissionRepository{client: client}
}

func (r *PermissionRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Permission], error) {
	client := r.client.DB(ctx)

	query := client.Permission.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, permissionRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, permissionRepoName)
	}

	entities := make([]*entity.Permission, len(records))
	for i, record := range records {
		entities[i] = mapper.ToPermissionEntity(record)
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

	return &d.Paginated[*entity.Permission]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *PermissionRepository) Get(ctx context.Context, id int) (*entity.Permission, error) {
	record, err := r.client.DB(ctx).Permission.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, permissionRepoName)
	}
	return mapper.ToPermissionEntity(record), nil
}

func (r *PermissionRepository) Create(ctx context.Context, e *entity.Permission) error {
	create := builder.BuildCreatePermission(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, permissionRepoName)
	}

	if created := mapper.ToPermissionEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *PermissionRepository) Update(ctx context.Context, e *entity.Permission) error {
	update := builder.BuildUpdatePermission(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, permissionRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *PermissionRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Permission.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, permissionRepoName)
	}
	return nil
}

func (r *PermissionRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Permission.Query().Where(permission.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, permissionRepoName)
}

func (r *PermissionRepository) FindByRoleIDs(ctx context.Context, roleIDs []int) ([]*entity.Permission, error) {
	records, err := r.client.DB(ctx).Permission.Query().
		Where(permission.RoleIDIn(roleIDs...)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, permissionRepoName)
	}

	entities := make([]*entity.Permission, len(records))
	for i, m := range records {
		entities[i] = mapper.ToPermissionEntity(m)
	}
	return entities, nil
}
