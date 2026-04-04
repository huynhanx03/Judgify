package db

import (
	"context"
	"time"

	commonEnt "github.com/huynhanx03/judgify/pkg/database/ent"
	d "github.com/huynhanx03/judgify/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/usersolvedproblem"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/builder"
	"github.com/huynhanx03/judgify/internal/problem/adapters/driven/db/mapper"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

const problemRepoName = "Problem"

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

	records, err := query.WithTags(func(tq *generate.TagQuery) { tq.WithElements() }).WithDifficulty().All(ctx)
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
	record, err := r.client.DB(ctx).Problem.Query().
		Where(problem.ID(id)).
		WithTags(func(tq *generate.TagQuery) { tq.WithElements() }).
		WithDifficulty().
		Only(ctx)
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
	if err != nil {
		return false, commonEnt.MapEntError(err, problemRepoName)
	}
	return exists, nil
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

func (r *ProblemRepository) InsertUserSolved(ctx context.Context, userID, problemID int) error {
	// Use Ent upsert: ON CONFLICT (user_id, problem_id) DO NOTHING
	return r.client.DB(ctx).UserSolvedProblem.Create().
		SetUserID(userID).
		SetProblemID(problemID).
		SetSolvedAt(time.Now()).
		OnConflict(
			sql.ConflictColumns("user_id", "problem_id"),
		).
		DoNothing().
		Exec(ctx)
}

func (r *ProblemRepository) IsFirstSolve(ctx context.Context, userID, problemID int) (bool, error) {
	exists, err := r.client.DB(ctx).UserSolvedProblem.Query().
		Where(
			usersolvedproblem.UserID(userID),
			usersolvedproblem.ProblemID(problemID),
		).
		Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, problemRepoName)
	}
	return !exists, nil
}

func (r *ProblemRepository) GetSolvedProblemIDs(ctx context.Context, userID int, problemIDs []int) (map[int]bool, error) {
	rows, err := r.client.DB(ctx).UserSolvedProblem.Query().
		Where(
			usersolvedproblem.UserID(userID),
			usersolvedproblem.ProblemIDIn(problemIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}
	result := make(map[int]bool, len(rows))
	for _, row := range rows {
		result[row.ProblemID] = true
	}
	return result, nil
}

func (r *ProblemRepository) RecalculateStats(ctx context.Context, since *time.Time) error {
	// Get problem IDs that had submissions since last check
	query := r.client.DB(ctx).Problem.Query()
	if since != nil {
		query = query.Where(func(s *sql.Selector) {
			sub := sql.Select("DISTINCT problem_id").From(sql.Table("submissions"))
			sub.Where(sql.GT("created_at", *since))
			s.Where(sql.In(s.C("id"), sub))
		})
	}
	problems, err := query.IDs(ctx)
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		return nil
	}

	// Recalculate each affected problem from submissions table
	for _, pid := range problems {
		var total, accepted int
		rows, err := r.client.DB(ctx).Problem.QueryContext(ctx,
			"SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'accepted') FROM submissions WHERE problem_id = $1 AND deleted_at IS NULL", pid)
		if err != nil {
			return err
		}
		if rows.Next() {
			_ = rows.Scan(&total, &accepted)
		}
		rows.Close()

		_ = r.client.DB(ctx).Problem.UpdateOneID(pid).
			SetSubmissionCount(total).
			SetAcceptedCount(accepted).
			Exec(ctx)
	}
	return nil
}
