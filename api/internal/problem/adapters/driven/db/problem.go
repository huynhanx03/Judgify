package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"

	"entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	entpredicate "github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/userproblemsolve"
	"github.com/huynhanx03/judgify/internal/ent/generate/usersolvedproblem"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const problemRepoName = "Problem"

type ProblemRepository struct {
	client *dbEnt.EntClient
}

func NewProblemRepository(client *dbEnt.EntClient) ports.ProblemRepository {
	return &ProblemRepository{client: client}
}

func (r *ProblemRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Problem], error) {
	return r.find(ctx, opts)
}

func (r *ProblemRepository) FindPublished(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Problem], error) {
	return r.find(ctx, opts, publishedProblemPredicates()...)
}

func (r *ProblemRepository) find(ctx context.Context, opts *d.QueryOptions, predicates ...entpredicate.Problem) (*d.Paginated[*entity.Problem], error) {
	client := r.client.DB(ctx)

	predicates = append(predicates, problemRelationPredicates(opts)...)
	query := client.Problem.Query().Where(predicates...)
	query.Where(func(s *sql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, problemQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}

	query.Where(func(s *sql.Selector) {
		applyProblemQueryWindow(opts, s)
	})

	records, err := query.WithTags(func(tq *generate.TagQuery) { tq.WithElements() }).WithDifficulty().All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}

	entities := make([]*entity.Problem, len(records))
	for i, record := range records {
		entities[i] = ToProblemEntity(record)
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

func (r *ProblemRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Problem, error) {
	return r.get(ctx, problem.ID(id))
}

func (r *ProblemRepository) GetPublished(ctx context.Context, id uuid.UUID) (*entity.Problem, error) {
	predicates := append(
		[]entpredicate.Problem{problem.ID(id)},
		publishedProblemPredicates()...,
	)
	return r.get(ctx, predicates...)
}

func publishedProblemPredicates() []entpredicate.Problem {
	return []entpredicate.Problem{
		problem.LifecycleEQ(problem.LifecyclePublished),
		problem.IsPublished(true),
		problem.PublishedRevisionIDNotNil(),
		problem.PublishedRenderSourceIDNotNil(),
		problem.PublishedRenderSourceKindNotNil(),
		problem.PublishedRenderSourceChecksumNotNil(),
		problem.PublishedRenderArtifactIDNotNil(),
	}
}

func (r *ProblemRepository) get(ctx context.Context, predicates ...entpredicate.Problem) (*entity.Problem, error) {
	record, err := r.client.DB(ctx).Problem.Query().
		Where(predicates...).
		WithTags(func(tq *generate.TagQuery) { tq.WithElements() }).
		WithDifficulty().
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}
	return ToProblemEntity(record), nil
}

func (r *ProblemRepository) Create(ctx context.Context, e *entity.Problem) error {
	client := r.client.DB(ctx)
	create := BuildCreateProblem(client, e)
	record, err := create.Save(ctx)
	if err != nil {
		return mapProblemCreatePersistenceError(err)
	}

	if created := ToProblemEntity(record); created != nil {
		*e = *created
	}
	return nil
}

func mapProblemCreatePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	mapped := commonEnt.MapEntError(err, problemRepoName)
	var postgresError *pq.Error
	if errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.Constraint == problemcontract.ProblemSlugUniqueConstraintName {
		return errors.Join(ports.ErrProblemSlugConflict, mapped)
	}
	return mapped
}

func (r *ProblemRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.client.DB(ctx).Problem.Query().Where(problem.ID(id)).Exist(ctx)
	if err != nil {
		return false, commonEnt.MapEntError(err, problemRepoName)
	}
	return exists, nil
}

func (r *ProblemRepository) AddTags(ctx context.Context, problemID uuid.UUID, tagIDs []uuid.UUID) error {
	err := r.client.DB(ctx).Problem.UpdateOneID(problemID).AddTagIDs(tagIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) RemoveTags(ctx context.Context, problemID uuid.UUID, tagIDs []uuid.UUID) error {
	err := r.client.DB(ctx).Problem.UpdateOneID(problemID).RemoveTagIDs(tagIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) ReplaceTags(ctx context.Context, problemID uuid.UUID, tagIDs []uuid.UUID) error {
	err := r.client.DB(ctx).Problem.UpdateOneID(problemID).ClearTags().AddTagIDs(tagIDs...).Exec(ctx)
	return commonEnt.MapEntError(err, problemRepoName)
}

func (r *ProblemRepository) GetTagIDs(ctx context.Context, problemID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := r.client.DB(ctx).Problem.Query().Where(problem.ID(problemID)).QueryTags().IDs(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}
	return ids, nil
}

func (r *ProblemRepository) InsertUserSolved(ctx context.Context, userID, problemID uuid.UUID) error {
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

func (r *ProblemRepository) IsFirstSolve(ctx context.Context, userID, problemID uuid.UUID) (bool, error) {
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

func (r *ProblemRepository) GetSolvedProblemIDs(ctx context.Context, userID uuid.UUID, problemIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(problemIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	rows, err := r.client.DB(ctx).UserProblemSolve.Query().
		Where(
			userproblemsolve.UserID(userID),
			userproblemsolve.ProblemIDIn(problemIDs...),
			userproblemsolve.StatusEQ(userproblemsolve.StatusSolved),
		).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, problemRepoName)
	}
	result := make(map[uuid.UUID]bool, len(rows))
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
		total, accepted, err := r.problemSubmissionCounts(ctx, pid)
		if err != nil {
			return err
		}
		if err := r.client.DB(ctx).Problem.UpdateOneID(pid).
			SetSubmissionCount(total).
			SetAcceptedCount(accepted).
			Exec(ctx); err != nil {
			return fmt.Errorf(
				"%s: persist recalculated counters for %s: %w",
				problemRepoName,
				pid,
				commonEnt.MapEntError(err, problemRepoName),
			)
		}
	}
	return nil
}

func (r *ProblemRepository) problemSubmissionCounts(
	ctx context.Context,
	problemID uuid.UUID,
) (int, int, error) {
	rows, err := r.client.DB(ctx).Problem.QueryContext(
		ctx,
		"SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'accepted') FROM submissions WHERE problem_id = $1 AND deleted_at IS NULL",
		problemID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"%s: query submission counters for %s: %w",
			problemRepoName,
			problemID,
			err,
		)
	}
	var total, accepted int64
	if !rows.Next() {
		iterationErr := rows.Err()
		closeErr := rows.Close()
		if iterationErr != nil || closeErr != nil {
			return 0, 0, fmt.Errorf(
				"%s: read submission counters for %s: %w",
				problemRepoName,
				problemID,
				errors.Join(iterationErr, closeErr),
			)
		}
		return 0, 0, fmt.Errorf(
			"%s: submission counter query returned no row for %s",
			problemRepoName,
			problemID,
		)
	}
	scanErr := rows.Scan(&total, &accepted)
	hasUnexpectedRow := rows.Next()
	iterationErr := rows.Err()
	closeErr := rows.Close()
	if scanErr != nil || iterationErr != nil || closeErr != nil {
		return 0, 0, fmt.Errorf(
			"%s: decode submission counters for %s: %w",
			problemRepoName,
			problemID,
			errors.Join(scanErr, iterationErr, closeErr),
		)
	}
	if hasUnexpectedRow || total < 0 || accepted < 0 || accepted > total ||
		total > int64(math.MaxInt) || accepted > int64(math.MaxInt) {
		return 0, 0, fmt.Errorf(
			"%s: invalid submission counters for %s",
			problemRepoName,
			problemID,
		)
	}
	return int(total), int(accepted), nil
}
