package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/difficulty"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

const difficultyRepoName = "Difficulty"

type DifficultyRepository struct {
	client *dbEnt.EntClient
}

func NewDifficultyRepository(client *dbEnt.EntClient) ports.DifficultyRepository {
	return &DifficultyRepository{client: client}
}

func (r *DifficultyRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Difficulty], error) {
	client := r.client.DB(ctx)

	query := client.Difficulty.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}

	entities := make([]*entity.Difficulty, len(records))
	for i, record := range records {
		entities[i] = mapper.ToDifficultyEntity(record)
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

	return &d.Paginated[*entity.Difficulty]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *DifficultyRepository) Get(ctx context.Context, id int) (*entity.Difficulty, error) {
	record, err := r.client.DB(ctx).Difficulty.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}
	return mapper.ToDifficultyEntity(record), nil
}

func (r *DifficultyRepository) Create(ctx context.Context, e *entity.Difficulty) error {
	create := builder.BuildCreateDifficulty(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, difficultyRepoName)
	}

	if created := mapper.ToDifficultyEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *DifficultyRepository) Update(ctx context.Context, e *entity.Difficulty) error {
	update := builder.BuildUpdateDifficulty(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, difficultyRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *DifficultyRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Difficulty.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, difficultyRepoName)
	}
	return nil
}

func (r *DifficultyRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Difficulty.Query().Where(difficulty.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, difficultyRepoName)
	}
	return exists, nil
}

func (r *DifficultyRepository) FindAll(ctx context.Context) ([]*entity.Difficulty, error) {
	records, err := r.client.DB(ctx).Difficulty.Query().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, difficultyRepoName)
	}
	entities := make([]*entity.Difficulty, len(records))
	for i, rec := range records {
		entities[i] = mapper.ToDifficultyEntity(rec)
	}
	return entities, nil
}
