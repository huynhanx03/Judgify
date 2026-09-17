package db

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const userRepoName = "User"

type UserRepository struct {
	client *dbEnt.EntClient
}

func NewUserRepository(client *dbEnt.EntClient) ports.UserRepository {
	return &UserRepository{client: client}
}

func (r *UserRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.User], error) {
	client := r.client.DB(ctx)

	query := client.User.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, userQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, userQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}

	entities := make([]*entity.User, len(records))
	for i, record := range records {
		entities[i] = ToUserEntity(record)
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

func (r *UserRepository) Get(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	record, err := r.client.DB(ctx).User.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}
	return ToUserEntity(record), nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	record, err := r.client.DB(ctx).User.Query().
		Where(user.Username(username)).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}
	return ToUserEntity(record), nil
}

func (r *UserRepository) Create(ctx context.Context, e *entity.User) error {
	create := BuildCreateUser(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, userRepoName)
	}

	if created := ToUserEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *UserRepository) TransitionLifecycleCAS(
	ctx context.Context,
	id uuid.UUID,
	expectedVersion int64,
	from entity.UserSecurityStatus,
	to entity.UserSecurityStatus,
	reason string,
	at time.Time,
	actorID uuid.UUID,
) (*entity.User, error) {
	if r == nil || r.client == nil || ctx == nil || id == uuid.Nil ||
		expectedVersion < 1 || expectedVersion == math.MaxInt64 ||
		actorID == uuid.Nil || at.IsZero() ||
		strings.TrimSpace(reason) != reason || reason == "" ||
		!validUserLifecycleTransition(from, to) {
		return nil, ports.ErrInvalidUserLifecycle
	}
	client := r.client.DB(ctx)
	record, err := client.User.Query().Where(
		user.IDEQ(id),
		user.StatusEQ(user.Status(from)),
		user.SecurityVersionEQ(expectedVersion),
		user.DeletedAtIsNil(),
	).Only(ctx)
	if generate.IsNotFound(err) {
		return nil, ports.ErrUserLifecycleConflict
	}
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}

	update := client.User.Update().Where(
		user.IDEQ(id),
		user.StatusEQ(user.Status(from)),
		user.SecurityVersionEQ(expectedVersion),
		user.DeletedAtIsNil(),
	).SetStatus(user.Status(to)).SetSecurityVersion(expectedVersion + 1)
	switch to {
	case entity.UserSecurityStatusSuspended:
		update.SetSuspendedAt(at.UTC()).SetSuspensionReason(reason)
	case entity.UserSecurityStatusActive:
		update.ClearSuspendedAt().ClearSuspensionReason()
	case entity.UserSecurityStatusDeleted:
		update.ClearSuspendedAt().ClearSuspensionReason().
			SetDeletedAt(at.UTC()).SetDeletedBy(actorID.String())
	default:
		return nil, ports.ErrInvalidUserLifecycle
	}
	affected, err := update.Save(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, userRepoName)
	}
	if affected != 1 {
		return nil, ports.ErrUserLifecycleConflict
	}

	result := ToUserEntity(record)
	if result == nil {
		return nil, ports.ErrInvalidUserLifecycle
	}
	result.SecurityStatus = to
	result.SecurityVersion = uint64(expectedVersion + 1)
	result.UpdatedAt = at.UTC()
	if to == entity.UserSecurityStatusSuspended {
		suspendedAt := at.UTC()
		result.SuspendedAt = &suspendedAt
		result.SuspensionReason = reason
	} else {
		result.SuspendedAt = nil
		result.SuspensionReason = ""
	}
	return result, nil
}

func (r *UserRepository) LoadCommandReceipt(
	ctx context.Context,
	commandID uuid.UUID,
) (ports.UserCommandReceipt, bool, error) {
	if r == nil || r.client == nil || ctx == nil || commandID == uuid.Nil {
		return ports.UserCommandReceipt{}, false, ports.ErrInvalidUserCommand
	}
	record, err := r.client.DB(ctx).OutboxEvent.Get(ctx, commandID)
	if generate.IsNotFound(err) {
		return ports.UserCommandReceipt{}, false, nil
	}
	if err != nil {
		return ports.UserCommandReceipt{}, false, commonEnt.MapEntError(err, userRepoName+" Command")
	}
	if record == nil {
		return ports.UserCommandReceipt{}, false, ports.ErrInvalidUserCommand
	}
	return ports.UserCommandReceipt{
		EventID:          record.ID,
		EventType:        record.EventType,
		AggregateType:    record.AggregateType,
		AggregateID:      record.AggregateID,
		AggregateVersion: record.AggregateVersion,
		Payload:          append([]byte(nil), record.Payload...),
		OccurredAt:       record.OccurredAt.UTC(),
	}, true, nil
}

func validUserLifecycleTransition(
	from entity.UserSecurityStatus,
	to entity.UserSecurityStatus,
) bool {
	switch to {
	case entity.UserSecurityStatusSuspended:
		return from == entity.UserSecurityStatusActive
	case entity.UserSecurityStatusActive:
		return from == entity.UserSecurityStatusSuspended
	case entity.UserSecurityStatusDeleted:
		return from == entity.UserSecurityStatusInvited ||
			from == entity.UserSecurityStatusActive ||
			from == entity.UserSecurityStatusSuspended
	default:
		return false
	}
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	exists, err := r.client.DB(ctx).User.Query().Where(user.Username(username)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, userRepoName)
	}
	return exists, nil
}
