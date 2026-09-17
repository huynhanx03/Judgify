package db

import (
	"context"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/trait"
	"github.com/huynhanx03/judgify/internal/ent/generate/traiteffectrevision"
)

const traitRepoName = "Trait"

type TraitRepository struct {
	client *dbEnt.EntClient
}

func NewTraitRepository(client *dbEnt.EntClient) *TraitRepository {
	return &TraitRepository{client: client}
}

func (r *TraitRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Trait], error) {
	query := r.client.DB(ctx).Trait.Query()
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, traitQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, traitQueryColumns...)
	})

	records, err := query.WithRarity().WithEffectRevisions(currentTraitEffectQuery).All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}

	entities := make([]*entity.Trait, len(records))
	for i, rec := range records {
		entities[i] = ToTraitEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.Trait]{Records: &entities, Pagination: meta}, nil
}

func (r *TraitRepository) FindAll(ctx context.Context) ([]*entity.Trait, error) {
	records, err := r.client.DB(ctx).Trait.Query().WithRarity().
		WithEffectRevisions(currentTraitEffectQuery).All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}

	entities := make([]*entity.Trait, len(records))
	for i, rec := range records {
		entities[i] = ToTraitEntity(rec)
	}
	return entities, nil
}

func (r *TraitRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Trait, error) {
	rec, err := r.client.DB(ctx).Trait.Query().Where(trait.ID(id)).WithRarity().
		WithEffectRevisions(currentTraitEffectQuery).Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, traitRepoName)
	}
	return ToTraitEntity(rec), nil
}

func (r *TraitRepository) Create(ctx context.Context, e *entity.Trait) error {
	rec, err := BuildCreateTrait(r.client.DB(ctx), e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, traitRepoName)
	}
	if created := ToTraitEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *TraitRepository) Update(ctx context.Context, e *entity.Trait, expectedVersion int64) error {
	if e == nil || expectedVersion < 1 || e.Version != expectedVersion {
		return entity.ErrTraitConflict
	}
	rec, err := BuildUpdateTrait(r.client.DB(ctx), e, expectedVersion).Save(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return entity.ErrTraitConflict
		}
		return commonEnt.MapEntError(err, traitRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	e.Version = rec.Version
	return nil
}

func (r *TraitRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.client.DB(ctx).Trait.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, traitRepoName)
	}
	return nil
}

func (r *TraitRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Trait.Query().Where(trait.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, traitRepoName)
	}
	return exists, nil
}

func currentTraitEffectQuery(query *generate.TraitEffectRevisionQuery) {
	query.Order(
		traiteffectrevision.ByEffectiveAt(sql.OrderDesc()),
		traiteffectrevision.ByRevision(sql.OrderDesc()),
		traiteffectrevision.ByID(sql.OrderDesc()),
	).Limit(1)
}
