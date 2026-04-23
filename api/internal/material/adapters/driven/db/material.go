package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/material"
	"github.com/huynhanx03/judgify/internal/material/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/material/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

const materialRepoName = "Material"

// MaterialRepository implements ports.MaterialRepository.
type MaterialRepository struct {
	client *dbEnt.EntClient
}

// NewMaterialRepository creates a new MaterialRepository.
func NewMaterialRepository(client *dbEnt.EntClient) ports.MaterialRepository {
	return &MaterialRepository{client: client}
}

func (r *MaterialRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error) {
	client := r.client.DB(ctx)

	query := client.Material.Query().
		Where(material.DeletedAtIsNil()).
		WithCategory().
		WithDifficulty().
		WithTags(func(tq *generate.TagQuery) {
			tq.WithElements()
		})

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialRepoName)
	}

	entities := make([]*entity.Material, len(records))
	for i, record := range records {
		entities[i] = mapper.ToMaterialEntity(record)
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

func (r *MaterialRepository) Get(ctx context.Context, id int) (*entity.Material, error) {
	record, err := r.client.DB(ctx).Material.Query().
		Where(material.IDEQ(id), material.DeletedAtIsNil()).
		WithCategory().
		WithDifficulty().
		WithTags(func(tq *generate.TagQuery) {
			tq.WithElements()
		}).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialRepoName)
	}
	return mapper.ToMaterialEntity(record), nil
}

func (r *MaterialRepository) Create(ctx context.Context, e *entity.Material) error {
	create := builder.BuildCreateMaterial(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	if created := mapper.ToMaterialEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *MaterialRepository) Update(ctx context.Context, e *entity.Material) error {
	update := builder.BuildUpdateMaterial(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	if updated := mapper.ToMaterialEntity(record); updated != nil {
		e.UpdatedAt = updated.UpdatedAt
	}
	return nil
}

func (r *MaterialRepository) Delete(ctx context.Context, id int) error {
	err := r.client.DB(ctx).Material.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}

func (r *MaterialRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Material.Query().
		Where(material.IDEQ(id), material.DeletedAtIsNil()).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, materialRepoName)
	}
	return exists, nil
}

func (r *MaterialRepository) IncrementViewCount(ctx context.Context, id int) error {
	_, err := r.client.DB(ctx).Material.UpdateOneID(id).
		AddViewCount(1).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}

func (r *MaterialRepository) AddTags(ctx context.Context, materialID int, tagIDs []int) error {
	_, err := r.client.DB(ctx).Material.UpdateOneID(materialID).
		AddTagIDs(tagIDs...).
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}

func (r *MaterialRepository) RemoveAllTags(ctx context.Context, materialID int) error {
	_, err := r.client.DB(ctx).Material.UpdateOneID(materialID).
		ClearTags().
		Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialRepoName)
	}
	return nil
}
