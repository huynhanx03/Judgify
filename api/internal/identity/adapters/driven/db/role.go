package db

import (
	"context"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"
	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/role"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const roleRepoName = "Role"

type RoleRepository struct {
	client *dbEnt.EntClient
}

func NewRoleRepository(client *dbEnt.EntClient) ports.RoleRepository {
	return &RoleRepository{client: client}
}

func (r *RoleRepository) FindAll(ctx context.Context) ([]*entity.Role, error) {
	records, err := r.client.DB(ctx).Role.Query().
		Where(
			role.StatusEQ(role.StatusActive),
			role.DeletedAtIsNil(),
		).
		Order(role.ByKey()).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	entities := make([]*entity.Role, len(records))
	for i, m := range records {
		entities[i] = ToRoleEntity(m)
	}
	return entities, nil
}

func (r *RoleRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Role], error) {
	client := r.client.DB(ctx)

	query := client.Role.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, roleQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, roleQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}

	entities := make([]*entity.Role, len(records))
	for i, record := range records {
		entities[i] = ToRoleEntity(record)
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

func (r *RoleRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Role, error) {
	record, err := r.client.DB(ctx).Role.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}
	return ToRoleEntity(record), nil
}

func (r *RoleRepository) GetByKey(ctx context.Context, key string) (*entity.Role, error) {
	record, err := r.client.DB(ctx).Role.Query().
		Where(
			role.Key(key),
			role.StatusEQ(role.StatusActive),
			role.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, roleRepoName)
	}
	return ToRoleEntity(record), nil
}

func (r *RoleRepository) Create(ctx context.Context, e *entity.Role) error {
	create := BuildCreateRole(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, roleRepoName)
	}

	if created := ToRoleEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *RoleRepository) UpdateCAS(ctx context.Context, e *entity.Role, expectedVersion int64) error {
	if e == nil || e.ID == uuid.Nil || expectedVersion < 1 || e.Version != expectedVersion+1 {
		return ports.ErrRoleVersionConflict
	}
	update := r.client.DB(ctx).Role.Update().
		Where(
			role.IDEQ(e.ID),
			role.KeyEQ(e.Key),
			role.IsSystemEQ(false),
			role.StatusEQ(role.StatusActive),
			role.VersionEQ(expectedVersion),
			role.DeletedAtIsNil(),
		).
		SetName(e.Name).
		SetIsSystem(e.IsSystem).
		SetVersion(e.Version)
	if e.Status.Valid() {
		update.SetStatus(role.Status(e.Status))
	}
	if e.Description == "" {
		update.ClearDescription()
	} else {
		update.SetDescription(e.Description)
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, roleRepoName)
	}
	if affected != 1 {
		return ports.ErrRoleVersionConflict
	}
	return nil
}

func (r *RoleRepository) LoadCommandReceipt(ctx context.Context, commandID uuid.UUID) (ports.RoleCommandReceipt, bool, error) {
	if r == nil || r.client == nil || ctx == nil || commandID == uuid.Nil {
		return ports.RoleCommandReceipt{}, false, ports.ErrInvalidRoleCommand
	}
	record, err := r.client.DB(ctx).OutboxEvent.Get(ctx, commandID)
	if generate.IsNotFound(err) {
		return ports.RoleCommandReceipt{}, false, nil
	}
	if err != nil {
		return ports.RoleCommandReceipt{}, false, commonEnt.MapEntError(err, roleRepoName)
	}
	if record == nil {
		return ports.RoleCommandReceipt{}, false, ports.ErrInvalidRoleCommand
	}
	return ports.RoleCommandReceipt{
		EventID:          record.ID,
		EventType:        record.EventType,
		AggregateType:    record.AggregateType,
		AggregateID:      record.AggregateID,
		AggregateVersion: record.AggregateVersion,
		Payload:          append([]byte(nil), record.Payload...),
		OccurredAt:       record.OccurredAt.UTC(),
	}, true, nil
}

func (r *RoleRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Role.Query().Where(role.ID(id)).Exist(ctx)

	if err != nil {
		return false, commonEnt.MapEntError(err, roleRepoName)
	}

	return exists, nil
}
