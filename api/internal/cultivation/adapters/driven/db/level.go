package db

import (
	"context"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/level"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const levelRepoName = "Level"

type LevelRepository struct {
	client *dbEnt.EntClient
}

func NewLevelRepository(client *dbEnt.EntClient) ports.LevelRepository {
	return &LevelRepository{client: client}
}

func (r *LevelRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Level], error) {
	query := r.client.DB(ctx).Level.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, levelRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, levelRepoName)
	}

	entities := make([]*entity.Level, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToLevelEntity(rec)
	}

	paginationOpts := &d.PaginationOptions{}
	if opts != nil && opts.Pagination != nil {
		paginationOpts = opts.Pagination
	} else {
		paginationOpts.SetDefaults()
	}

	meta := d.CalculatePagination(paginationOpts.Page, paginationOpts.PageSize, int64(total))

	return &d.Paginated[*entity.Level]{Records: &entities, Pagination: meta}, nil
}

func (r *LevelRepository) Get(ctx context.Context, id int) (*entity.Level, error) {
	rec, err := r.client.DB(ctx).Level.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, levelRepoName)
	}
	return mapper.ToLevelEntity(rec), nil
}

func (r *LevelRepository) Create(ctx context.Context, e *entity.Level) error {
	rec, err := builder.BuildCreateLevel(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, levelRepoName)
	}
	if created := mapper.ToLevelEntity(rec); created != nil {
		*e = *created
	}
	return nil
}

func (r *LevelRepository) Update(ctx context.Context, e *entity.Level) error {
	rec, err := builder.BuildUpdateLevel(ctx, e).Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, levelRepoName)
	}
	e.UpdatedAt = rec.UpdatedAt
	return nil
}

func (r *LevelRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Level.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, levelRepoName)
	}
	return nil
}

func (r *LevelRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Level.Query().Where(level.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, levelRepoName)
	}
	return exists, nil
}

func (r *LevelRepository) FindAll(ctx context.Context) ([]*entity.Level, error) {
	records, err := r.client.DB(ctx).Level.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, levelRepoName)
	}
	entities := make([]*entity.Level, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToLevelEntity(rec)
	}
	return entities, nil
}
