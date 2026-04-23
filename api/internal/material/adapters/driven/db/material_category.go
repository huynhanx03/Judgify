package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/materialcategory"
	"github.com/huynhanx03/judgify/internal/material/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/material/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

const materialCategoryRepoName = "MaterialCategory"

// MaterialCategoryRepository implements ports.MaterialCategoryRepository.
type MaterialCategoryRepository struct {
	client *dbEnt.EntClient
}

// NewMaterialCategoryRepository creates a new MaterialCategoryRepository.
func NewMaterialCategoryRepository(client *dbEnt.EntClient) ports.MaterialCategoryRepository {
	return &MaterialCategoryRepository{client: client}
}

func (r *MaterialCategoryRepository) FindAll(ctx context.Context) ([]*entity.MaterialCategory, error) {
	records, err := r.client.DB(ctx).MaterialCategory.Query().
		Where(materialcategory.DeletedAtIsNil()).
		Order(materialcategory.ByName()).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}

	entities := make([]*entity.MaterialCategory, len(records))
	for i, record := range records {
		e := mapper.ToMaterialCategoryEntity(record)
		// Count materials in category
		count, _ := record.QueryMaterials().Where(func(s *sql.Selector) {
			s.Where(sql.IsNull(materialcategory.FieldDeletedAt))
		}).Count(ctx)
		e.ArticleCount = count
		entities[i] = e
	}

	return entities, nil
}

func (r *MaterialCategoryRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.MaterialCategory], error) {
	client := r.client.DB(ctx)

	query := client.MaterialCategory.Query().
		Where(materialcategory.DeletedAtIsNil())

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}

	entities := make([]*entity.MaterialCategory, len(records))
	for i, record := range records {
		e := mapper.ToMaterialCategoryEntity(record)
		count, _ := record.QueryMaterials().Count(ctx)
		e.ArticleCount = count
		entities[i] = e
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

	return &d.Paginated[*entity.MaterialCategory]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *MaterialCategoryRepository) Get(ctx context.Context, id int) (*entity.MaterialCategory, error) {
	record, err := r.client.DB(ctx).MaterialCategory.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return mapper.ToMaterialCategoryEntity(record), nil
}

func (r *MaterialCategoryRepository) Create(ctx context.Context, e *entity.MaterialCategory) error {
	create := builder.BuildCreateMaterialCategory(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	if created := mapper.ToMaterialCategoryEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *MaterialCategoryRepository) Update(ctx context.Context, e *entity.MaterialCategory) error {
	update := builder.BuildUpdateMaterialCategory(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	if updated := mapper.ToMaterialCategoryEntity(record); updated != nil {
		e.UpdatedAt = updated.UpdatedAt
	}
	return nil
}

func (r *MaterialCategoryRepository) Delete(ctx context.Context, id int) error {
	err := r.client.DB(ctx).MaterialCategory.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return nil
}

func (r *MaterialCategoryRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).MaterialCategory.Query().
		Where(materialcategory.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return exists, nil
}
