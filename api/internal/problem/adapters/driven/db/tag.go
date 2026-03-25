package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/tag"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

const tagRepoName = "TagRepository"

type TagRepository struct {
	client *dbEnt.EntClient
}

func NewTagRepository(client *dbEnt.EntClient) ports.TagRepository {
	return &TagRepository{client: client}
}

func (r *TagRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Tag], error) {
	client := r.client.DB(ctx)

	query := client.Tag.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}

	entities := make([]*entity.Tag, len(records))
	for i, record := range records {
		entities[i] = mapper.ToTagEntity(record)
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

	return &d.Paginated[*entity.Tag]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *TagRepository) Get(ctx context.Context, id int) (*entity.Tag, error) {
	record, err := r.client.DB(ctx).Tag.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}
	return mapper.ToTagEntity(record), nil
}

func (r *TagRepository) Create(ctx context.Context, e *entity.Tag) error {
	create := builder.BuildCreateTag(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, tagRepoName)
	}

	if created := mapper.ToTagEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *TagRepository) Update(ctx context.Context, e *entity.Tag) error {
	update := builder.BuildUpdateTag(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, tagRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *TagRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Tag.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, tagRepoName)
	}
	return nil
}

func (r *TagRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Tag.Query().Where(tag.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, tagRepoName)
}

func (r *TagRepository) FindByIDs(ctx context.Context, ids []int) ([]*entity.Tag, error) {
	records, err := r.client.DB(ctx).Tag.Query().
		Where(tag.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, tagRepoName)
	}

	entities := make([]*entity.Tag, len(records))
	for i, m := range records {
		entities[i] = mapper.ToTagEntity(m)
	}
	return entities, nil
}
