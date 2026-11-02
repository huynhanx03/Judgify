package db

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/materialcategory"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const materialCategoryRepoName = "MaterialCategory"

// MaterialCategoryRepository implements ports.MaterialCategoryRepository.
type MaterialCategoryRepository struct {
	client *dbEnt.EntClient
}

// NewMaterialCategoryRepository constructs a MaterialCategoryRepository.
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
	counts, err := r.articleCounts(ctx, true)
	if err != nil {
		return nil, err
	}

	entities := make([]*entity.MaterialCategory, len(records))
	for i, record := range records {
		e := ToMaterialCategoryEntity(record)
		e.ArticleCount = counts[e.ID]
		entities[i] = e
	}

	return entities, nil
}

func (r *MaterialCategoryRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.MaterialCategory], error) {
	client := r.client.DB(ctx)

	query := client.MaterialCategory.Query().
		Where(materialcategory.DeletedAtIsNil())

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, materialCategoryQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	counts, err := r.articleCounts(ctx, false)
	if err != nil {
		return nil, err
	}

	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, materialCategoryQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}

	entities := make([]*entity.MaterialCategory, len(records))
	for i, record := range records {
		e := ToMaterialCategoryEntity(record)
		e.ArticleCount = counts[e.ID]
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

// articleCounts projects category membership from the immutable revision
// selected by each material identity. The anonymous path intentionally uses
// only the published/public pointer so draft and authenticated content cannot
// leak through aggregate counts.
func (r *MaterialCategoryRepository) articleCounts(
	ctx context.Context,
	publicOnly bool,
) (map[uuid.UUID]int, error) {
	if r == nil || r.client == nil || ctx == nil {
		return nil, fmt.Errorf("%s: invalid category count query", materialCategoryRepoName)
	}
	statement := `
		SELECT revision.category_id, COUNT(*)
		FROM materials AS material
		JOIN material_revisions AS revision
		  ON revision.id = COALESCE(material.current_draft_revision_id, material.published_revision_id)
		 AND revision.material_id = material.id
		WHERE material.deleted_at IS NULL
		GROUP BY revision.category_id`
	if publicOnly {
		statement = `
			SELECT revision.category_id, COUNT(*)
			FROM materials AS material
			JOIN material_revisions AS revision
			  ON revision.id = material.published_revision_id
			 AND revision.material_id = material.id
			WHERE material.deleted_at IS NULL
			  AND material.status = 'published'
			  AND material.visibility = 'public'
			GROUP BY revision.category_id`
	}
	rows, err := r.client.DB(ctx).MaterialCategory.QueryContext(ctx, statement)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	defer func() { _ = rows.Close() }()
	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var categoryID uuid.UUID
		var count int64
		if err := rows.Scan(&categoryID, &count); err != nil {
			return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
		}
		if categoryID == uuid.Nil || count < 0 || uint64(count) > uint64(^uint(0)>>1) {
			return nil, fmt.Errorf("%s: invalid category count projection", materialCategoryRepoName)
		}
		counts[categoryID] = int(count)
	}
	if err := rows.Err(); err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return counts, nil
}

func (r *MaterialCategoryRepository) Get(ctx context.Context, id uuid.UUID) (*entity.MaterialCategory, error) {
	record, err := r.client.DB(ctx).MaterialCategory.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return ToMaterialCategoryEntity(record), nil
}

func (r *MaterialCategoryRepository) Create(ctx context.Context, e *entity.MaterialCategory) error {
	create := BuildCreateMaterialCategory(r.client.DB(ctx), e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	if created := ToMaterialCategoryEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *MaterialCategoryRepository) Update(ctx context.Context, e *entity.MaterialCategory) error {
	update := BuildUpdateMaterialCategory(r.client.DB(ctx), e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	if updated := ToMaterialCategoryEntity(record); updated != nil {
		e.UpdatedAt = updated.UpdatedAt
	}
	return nil
}

func (r *MaterialCategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.client.DB(ctx).MaterialCategory.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return nil
}

func (r *MaterialCategoryRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).MaterialCategory.Query().
		Where(materialcategory.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, materialCategoryRepoName)
	}
	return exists, nil
}
