package db

import (
	"context"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

const problemRepoName = "ProblemRepository"

type ProblemRepository struct {
	client *dbEnt.EntClient
}

func NewProblemRepository(client *dbEnt.EntClient) ports.ProblemRepository {
	return &ProblemRepository{client: client}
}

func (r *ProblemRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Problem], error) {
	client := r.client.DB(ctx)

	query := client.Problem.Query()
	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplyFilters(opts.Filters, s)
		})
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}

	if opts != nil {
		query.Where(func(s *sql.Selector) {
			commonEnt.ApplySort(opts.Sort, s)
			commonEnt.ApplyPagination(opts.Pagination, s)
		})
	}

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}

	entities := make([]*entity.Problem, len(records))
	for i, record := range records {
		entities[i] = mapper.ToProblemEntity(record)
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

	return &d.Paginated[*entity.Problem]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *ProblemRepository) Get(ctx context.Context, id int) (*entity.Problem, error) {
	record, err := r.client.DB(ctx).Problem.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}
	return mapper.ToProblemEntity(record), nil
}

func (r *ProblemRepository) Create(ctx context.Context, e *entity.Problem) error {
	create := builder.BuildCreateProblem(ctx, e)
	record, err := create.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, problemRepoName)
	}

	if created := mapper.ToProblemEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func (r *ProblemRepository) Update(ctx context.Context, e *entity.Problem) error {
	update := builder.BuildUpdateProblem(ctx, e)
	record, err := update.Save(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, problemRepoName)
	}
	e.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *ProblemRepository) Delete(ctx context.Context, id int) error {
	if err := r.client.DB(ctx).Problem.DeleteOneID(id).Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, problemRepoName)
	}
	return nil
}

func (r *ProblemRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.client.DB(ctx).Problem.Query().Where(problem.ID(id)).Exist(ctx)
	return exists, commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) AddTags(ctx context.Context, problemID int, tagIDs []int) error {
	err := r.client.DB(ctx).Problem.UpdateOneID(problemID).AddTagIDs(tagIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) RemoveTags(ctx context.Context, problemID int, tagIDs []int) error {
	err := r.client.DB(ctx).Problem.UpdateOneID(problemID).RemoveTagIDs(tagIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) ReplaceTags(ctx context.Context, problemID int, tagIDs []int) error {
	err := r.client.DB(ctx).Problem.UpdateOneID(problemID).ClearTags().AddTagIDs(tagIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) GetTagIDs(ctx context.Context, problemID int) ([]int, error) {
	ids, err := r.client.DB(ctx).Problem.Query().Where(problem.ID(problemID)).QueryTags().IDs(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}
	return ids, nil
}
