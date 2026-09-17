package db

import (
	"context"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/material"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const materialRepoName = "Material"

// MaterialRepository implements ports.MaterialRepository.
type MaterialRepository struct {
	client *dbEnt.EntClient
}

// NewMaterialRepository constructs a MaterialRepository.
func NewMaterialRepository(client *dbEnt.EntClient) ports.MaterialRepository {
	return &MaterialRepository{client: client}
}

func (r *MaterialRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error) {
	return r.find(ctx, opts)
}

func (r *MaterialRepository) FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error) {
	return r.find(
		ctx,
		opts,
		material.StatusEQ(material.StatusPublished),
		material.VisibilityEQ(material.VisibilityPublic),
	)
}

func (r *MaterialRepository) find(ctx context.Context, opts *d.QueryOptions, accessPredicates ...predicate.Material) (*d.Paginated[*entity.Material], error) {
	client := r.client.DB(ctx)

	query := client.Material.Query().
		Where(material.DeletedAtIsNil()).
		WithCategory().
		WithDifficulty().
		WithTags(func(tq *generate.TagQuery) {
			tq.WithElements()
		})
	query.Where(accessPredicates...)
	query.Where(materialSearchPredicates(opts)...)
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, materialQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialRepoName)
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, materialQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialRepoName)
	}

	entities := make([]*entity.Material, len(records))
	for i, record := range records {
		entities[i] = ToMaterialEntity(record)
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

	return &d.Paginated[*entity.Material]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *MaterialRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Material, error) {
	return r.get(ctx, id)
}

func (r *MaterialRepository) GetPublic(ctx context.Context, id uuid.UUID) (*entity.Material, error) {
	return r.get(
		ctx,
		id,
		material.StatusEQ(material.StatusPublished),
		material.VisibilityEQ(material.VisibilityPublic),
	)
}

func (r *MaterialRepository) get(ctx context.Context, id uuid.UUID, accessPredicates ...predicate.Material) (*entity.Material, error) {
	predicates := append([]predicate.Material{
		material.IDEQ(id),
		material.DeletedAtIsNil(),
	}, accessPredicates...)
	record, err := r.client.DB(ctx).Material.Query().
		Where(predicates...).
		WithCategory().
		WithDifficulty().
		WithTags(func(tq *generate.TagQuery) {
			tq.WithElements()
		}).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialRepoName)
	}
	return ToMaterialEntity(record), nil
}

func (r *MaterialRepository) Create(ctx context.Context, e *entity.Material) error {
	create := BuildCreateMaterial(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	if created := ToMaterialEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *MaterialRepository) Update(ctx context.Context, e *entity.Material) error {
	update := BuildUpdateMaterial(r.client.DB(ctx), e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	if updated := ToMaterialEntity(record); updated != nil {
		e.UpdatedAt = updated.UpdatedAt
	}
	return nil
}

func (r *MaterialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.client.DB(ctx).Material.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}

func (r *MaterialRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Material.Query().
		Where(material.IDEQ(id), material.DeletedAtIsNil()).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, materialRepoName)
	}
	return exists, nil
}

func (r *MaterialRepository) IncrementViewCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.client.DB(ctx).Material.UpdateOneID(id).
		AddViewCount(1).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}

func (r *MaterialRepository) AddTags(ctx context.Context, materialID uuid.UUID, tagIDs []uuid.UUID) error {
	_, err := r.client.DB(ctx).Material.UpdateOneID(materialID).
		AddTagIDs(tagIDs...).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}

func (r *MaterialRepository) RemoveAllTags(ctx context.Context, materialID uuid.UUID) error {
	_, err := r.client.DB(ctx).Material.UpdateOneID(materialID).
		ClearTags().
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}
