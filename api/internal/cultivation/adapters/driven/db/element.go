package db

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
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
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, elementQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, elementQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}

	entities := make([]*entity.Element, len(records))
	for i, rec := range records {
		entities[i] = ToElementEntity(rec)
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

func (r *ElementRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Element, error) {
	rec, err := r.client.DB(ctx).Element.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}
	return ToElementEntity(rec), nil
}

func (r *ElementRepository) Create(ctx context.Context, e *entity.Element) error {
	if err := validateElementWrite(r, ctx, e, false); err != nil {
		return err
	}
	var created *entity.Element
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		record, err := BuildCreateElement(client, e).Save(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, elementRepoName)
		}
		checksum := elementRevisionChecksum(record.ID, 1, record.Name, record.Code, record.Description)
		if err := client.ElementRevision.Create().
			SetElementID(record.ID).
			SetRevision(1).
			SetName(record.Name).
			SetCode(record.Code).
			SetDescription(record.Description).
			SetChecksum(checksum[:]).
			Exec(txCtx); err != nil {
			return commonEnt.MapEntError(err, elementRepoName)
		}
		created = ToElementEntity(record)
		return nil
	})
	if err != nil {
		return err
	}
	*e = *created
	return nil
}

func (r *ElementRepository) Update(ctx context.Context, e *entity.Element) error {
	if err := validateElementWrite(r, ctx, e, true); err != nil {
		return err
	}
	var updated *entity.Element
	err := r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		stored, err := client.Element.Query().
			Where(element.ID(e.ID), element.DeletedAtIsNil()).
			Modify(func(selector *sql.Selector) { selector.ForUpdate() }).
			Only(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, elementRepoName)
		}
		if stored.Version != e.Version {
			return ports.ErrElementVersionConflict
		}
		if stored.Version >= problemcontract.MaximumRewardMappingRevision {
			return fmt.Errorf("%s: maximum revision reached", elementRepoName)
		}
		nextVersion := stored.Version + 1
		checksum := elementRevisionChecksum(stored.ID, nextVersion, e.Name, e.Code, e.Description)
		if err := client.ElementRevision.Create().
			SetElementID(stored.ID).
			SetRevision(nextVersion).
			SetName(e.Name).
			SetCode(e.Code).
			SetDescription(e.Description).
			SetChecksum(checksum[:]).
			Exec(txCtx); err != nil {
			return commonEnt.MapEntError(err, elementRepoName)
		}
		record, err := BuildUpdateElement(client, e, nextVersion).
			Where(element.Version(e.Version)).
			Save(txCtx)
		if err != nil {
			if generate.IsNotFound(err) {
				return ports.ErrElementVersionConflict
			}
			return commonEnt.MapEntError(err, elementRepoName)
		}
		updated = ToElementEntity(record)
		return nil
	})
	if err != nil {
		return err
	}
	*e = *updated
	return nil
}

func validateElementWrite(r *ElementRepository, ctx context.Context, value *entity.Element, update bool) error {
	if r == nil || r.client == nil || ctx == nil || value == nil || ctx.Err() != nil {
		return errors.New("element: invalid write")
	}
	if !utf8.ValidString(value.Name) || !utf8.ValidString(value.Code) || !utf8.ValidString(value.Description) ||
		value.Name == "" || utf8.RuneCountInString(value.Name) > 50 || value.Code == "" ||
		utf8.RuneCountInString(value.Code) > 20 || utf8.RuneCountInString(value.Description) > 255 ||
		(update && (value.ID == uuid.Nil || value.Version < 1)) {
		return errors.New("element: invalid authored facts")
	}
	return nil
}

func (r *ElementRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Element.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, elementRepoName)
	}
	return nil
}

func (r *ElementRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Element.Query().Where(element.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, elementRepoName)
	}
	return exists, nil
}

func (r *ElementRepository) FindAll(ctx context.Context) ([]*entity.Element, error) {
	return r.findAll(ctx, 0)
}

func (r *ElementRepository) FindAllBounded(
	ctx context.Context,
	limit int,
) ([]*entity.Element, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%s: limit must be positive", elementRepoName)
	}
	return r.findAll(ctx, limit)
}

func (r *ElementRepository) findAll(ctx context.Context, limit int) ([]*entity.Element, error) {
	query := r.client.DB(ctx).Element.Query().Order(element.ByID())
	if limit > 0 {
		query.Limit(limit)
	}
	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, elementRepoName)
	}
	entities := make([]*entity.Element, len(records))
	for i, rec := range records {
		entities[i] = ToElementEntity(rec)
	}
	return entities, nil
}
