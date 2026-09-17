package db

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	entsql "entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestdraftproblem"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestproblem"
	"github.com/huynhanx03/judgify/internal/ent/generate/contestregistration"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ports"
)

const contestRepoName = "Contest"

const maximumContestProblemProjections = 500

var contestProblemAliasPattern = regexp.MustCompile(
	`^[A-Z][A-Z0-9]{0,15}$`,
)

var contestRuntimeKeyPattern = regexp.MustCompile(
	`^[a-z0-9][a-z0-9._-]{0,63}$`,
)

var contestQueryColumns = []string{
	contest.FieldID,
	contest.FieldCreatedAt,
	contest.FieldUpdatedAt,
	contest.FieldTitle,
	contest.FieldStartTime,
	contest.FieldEndTime,
	contest.FieldStatus,
	contest.FieldAuthorID,
	contest.FieldMaxParticipants,
}

// ContestRepository implements ports.ContestRepository.
type ContestRepository struct {
	client *dbEnt.EntClient
}

// NewContestRepository constructs a ContestRepository.
func NewContestRepository(client *dbEnt.EntClient) ports.ContestRepository {
	return &ContestRepository{client: client}
}

func (r *ContestRepository) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error) {
	return r.find(ctx, opts)
}

// FindPublic returns only contests whose publication lifecycle has started.
// This repository-owned predicate is deliberately separate from caller-owned
// query filters so a crafted status filter can only narrow the public set.
func (r *ContestRepository) FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error) {
	return r.find(
		ctx,
		opts,
		contest.StatusNEQ(contest.StatusDraft),
		contest.PublishedAtNotNil(),
	)
}

func (r *ContestRepository) find(
	ctx context.Context,
	opts *d.QueryOptions,
	accessPredicates ...predicate.Contest,
) (*d.Paginated[*entity.Contest], error) {
	client := r.client.DB(ctx)

	query := client.Contest.Query().Where(accessPredicates...).Where(contestSearchPredicates(opts)...)
	query.Where(func(s *entsql.Selector) {
		commonEnt.ApplyQueryFilters(opts, s, contestQueryColumns...)
	})

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}

	query.Where(func(s *entsql.Selector) {
		commonEnt.ApplyQueryWindow(opts, s, contestQueryColumns...)
	})

	records, err := query.All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}

	entities := make([]*entity.Contest, len(records))
	for i, record := range records {
		entities[i] = ToContestEntity(record)
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

	return &d.Paginated[*entity.Contest]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}

func (r *ContestRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Contest, error) {
	return r.get(ctx, id)
}

// GetPublic applies the same non-draft boundary to direct-ID reads. Returning
// not found for a draft avoids leaking its existence or private configuration.
func (r *ContestRepository) GetPublic(ctx context.Context, id uuid.UUID) (*entity.Contest, error) {
	return r.get(
		ctx,
		id,
		contest.StatusNEQ(contest.StatusDraft),
		contest.PublishedAtNotNil(),
	)
}

func (r *ContestRepository) get(ctx context.Context, id uuid.UUID, accessPredicates ...predicate.Contest) (*entity.Contest, error) {
	predicates := append([]predicate.Contest{contest.IDEQ(id)}, accessPredicates...)
	record, err := r.client.DB(ctx).Contest.Query().Where(predicates...).Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	return ToContestEntity(record), nil
}

func (r *ContestRepository) CreateDraft(
	ctx context.Context,
	value *entity.Contest,
	problems []entity.ContestDraftProblem,
) error {
	if r == nil || r.client == nil || ctx == nil || value == nil {
		return entity.ErrContestDraftInvalid
	}
	return r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		record, err := BuildCreateContest(client, value).Save(txCtx)
		if err != nil {
			return commonEnt.MapEntError(err, contestRepoName)
		}
		if err := replaceContestDraftProblems(
			txCtx,
			client,
			record.ID,
			problems,
		); err != nil {
			return err
		}
		created := ToContestEntity(record)
		if created == nil {
			return entity.ErrContestDraftInvalid
		}
		*value = *created
		return nil
	})
}

func (r *ContestRepository) UpdateDraft(
	ctx context.Context,
	value *entity.Contest,
	expectedVersion int64,
	problems []entity.ContestDraftProblem,
) error {
	if r == nil || r.client == nil || ctx == nil || value == nil ||
		value.ID == uuid.Nil || expectedVersion < 1 {
		return entity.ErrContestDraftInvalid
	}
	return r.client.DoInTx(ctx, func(txCtx context.Context) error {
		client := r.client.DB(txCtx)
		current, err := client.Contest.Query().
			Where(contest.IDEQ(value.ID)).
			Select(
				contest.FieldID,
				contest.FieldStatus,
				contest.FieldVersion,
			).
			Modify(func(selector *entsql.Selector) { selector.ForUpdate() }).
			Only(txCtx)
		if generate.IsNotFound(err) {
			return entity.ErrContestNotFound
		}
		if err != nil {
			return commonEnt.MapEntError(err, contestRepoName)
		}
		if current.Status != contest.StatusDraft {
			return entity.ErrContestNotDraft
		}
		if current.Version != expectedVersion {
			return entity.ErrContestVersionConflict
		}

		value.Version = expectedVersion + 1
		record, err := BuildUpdateContest(client, value).Save(txCtx)
		if generate.IsNotFound(err) {
			return entity.ErrContestVersionConflict
		}
		if err != nil {
			return commonEnt.MapEntError(err, contestRepoName)
		}
		if err := replaceContestDraftProblems(
			txCtx,
			client,
			value.ID,
			problems,
		); err != nil {
			return err
		}
		updated := ToContestEntity(record)
		if updated == nil {
			return entity.ErrContestDraftInvalid
		}
		*value = *updated
		return nil
	})
}

func replaceContestDraftProblems(
	ctx context.Context,
	client *generate.Client,
	contestID uuid.UUID,
	problems []entity.ContestDraftProblem,
) error {
	if client == nil || contestID == uuid.Nil ||
		len(problems) > maximumContestProblemProjections {
		return entity.ErrContestDraftProblem
	}
	if _, err := client.ContestDraftProblem.Delete().
		Where(contestdraftproblem.ContestIDEQ(contestID)).
		Exec(ctx); err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	if len(problems) == 0 {
		return nil
	}

	problemIDs := make([]uuid.UUID, len(problems))
	for index, selected := range problems {
		problemIDs[index] = selected.ProblemID
	}
	publishedProblems, err := client.Problem.Query().
		Where(
			problem.IDIn(problemIDs...),
			problem.LifecycleEQ(problem.LifecyclePublished),
		).
		Select(
			problem.FieldID,
			problem.FieldSlug,
			problem.FieldTitle,
		).
		All(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	if len(publishedProblems) != len(problemIDs) {
		return entity.ErrContestDraftProblem
	}
	metadata := make(
		map[uuid.UUID]struct{ slug, title string },
		len(publishedProblems),
	)
	for _, publishedProblem := range publishedProblems {
		metadata[publishedProblem.ID] = struct{ slug, title string }{
			slug:  publishedProblem.Slug,
			title: publishedProblem.Title,
		}
	}

	creates := make(
		[]*generate.ContestDraftProblemCreate,
		len(problems),
	)
	for index, selected := range problems {
		details, exists := metadata[selected.ProblemID]
		if !exists || details.slug == "" || details.title == "" {
			return entity.ErrContestDraftProblem
		}
		problems[index].ProblemSlug = details.slug
		problems[index].ProblemTitle = details.title
		creates[index] = client.ContestDraftProblem.Create().
			SetContestID(contestID).
			SetProblemID(selected.ProblemID).
			SetDisplayOrder(selected.DisplayOrder).
			SetAlias(selected.Alias).
			SetNillablePoints(selected.Points).
			SetVisibleBeforeStart(selected.VisibleBeforeStart)
	}
	if _, err := client.ContestDraftProblem.CreateBulk(creates...).
		Save(ctx); err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	return nil
}

func (r *ContestRepository) DeleteDraft(
	ctx context.Context,
	id uuid.UUID,
) error {
	if r == nil || r.client == nil || ctx == nil || id == uuid.Nil {
		return entity.ErrContestDraftInvalid
	}
	deleted, err := r.client.DB(ctx).Contest.Delete().
		Where(
			contest.IDEQ(id),
			contest.StatusEQ(contest.StatusDraft),
		).
		Exec(ctx)
	if err != nil {
		return commonEnt.MapEntError(err, contestRepoName)
	}
	if deleted != 1 {
		return entity.ErrContestNotDraft
	}
	return nil
}

func (r *ContestRepository) GetReadModels(
	ctx context.Context,
	contestIDs []uuid.UUID,
	includeDraftProblems bool,
) (map[uuid.UUID]ports.ContestReadModel, error) {
	if r == nil || r.client == nil || ctx == nil ||
		len(contestIDs) > maximumContestProblemProjections {
		return nil, entity.ErrContestDraftInvalid
	}
	result := make(
		map[uuid.UUID]ports.ContestReadModel,
		len(contestIDs),
	)
	uniqueIDs := make([]uuid.UUID, 0, len(contestIDs))
	for _, contestID := range contestIDs {
		if contestID == uuid.Nil {
			return nil, entity.ErrContestDraftInvalid
		}
		if _, exists := result[contestID]; exists {
			continue
		}
		result[contestID] = ports.ContestReadModel{}
		uniqueIDs = append(uniqueIDs, contestID)
	}
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	var counts []struct {
		ContestID uuid.UUID `json:"contest_id"`
		Count     int       `json:"count"`
	}
	if err := r.client.DB(ctx).ContestRegistration.Query().
		Where(
			contestregistration.ContestIDIn(uniqueIDs...),
			contestregistration.StatusEQ(
				contestregistration.StatusRegistered,
			),
		).
		GroupBy(contestregistration.FieldContestID).
		Aggregate(generate.Count()).
		Scan(ctx, &counts); err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	for _, count := range counts {
		model, exists := result[count.ContestID]
		if !exists || count.Count < 0 {
			return nil, entity.ErrContestDraftInvalid
		}
		model.ParticipantCount = count.Count
		result[count.ContestID] = model
	}
	var problemCounts []struct {
		ContestID uuid.UUID `json:"contest_id"`
		Count     int       `json:"count"`
	}
	if err := r.client.DB(ctx).ContestProblem.Query().
		Where(contestproblem.ContestIDIn(uniqueIDs...)).
		GroupBy(contestproblem.FieldContestID).
		Aggregate(generate.Count()).
		Scan(ctx, &problemCounts); err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	for _, count := range problemCounts {
		model, exists := result[count.ContestID]
		if !exists || count.Count < 0 {
			return nil, entity.ErrContestDraftInvalid
		}
		model.ProblemCount = count.Count
		result[count.ContestID] = model
	}
	if !includeDraftProblems {
		return result, nil
	}

	records, err := r.client.DB(ctx).ContestDraftProblem.Query().
		Where(contestdraftproblem.ContestIDIn(uniqueIDs...)).
		WithProblem(func(query *generate.ProblemQuery) {
			query.Select(
				problem.FieldID,
				problem.FieldSlug,
				problem.FieldTitle,
			)
		}).
		Order(
			contestdraftproblem.ByContestID(),
			contestdraftproblem.ByDisplayOrder(),
		).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	for _, record := range records {
		problemRecord, err := record.Edges.ProblemOrErr()
		if err != nil || problemRecord == nil ||
			problemRecord.ID != record.ProblemID {
			return nil, entity.ErrContestDraftProblem
		}
		model, exists := result[record.ContestID]
		if !exists {
			return nil, entity.ErrContestDraftInvalid
		}
		model.DraftProblems = append(
			model.DraftProblems,
			entity.ContestDraftProblem{
				ProblemID:          record.ProblemID,
				ProblemSlug:        problemRecord.Slug,
				ProblemTitle:       problemRecord.Title,
				DisplayOrder:       record.DisplayOrder,
				Alias:              record.Alias,
				Points:             record.Points,
				VisibleBeforeStart: record.VisibleBeforeStart,
			},
		)
		result[record.ContestID] = model
	}
	return result, nil
}

func (r *ContestRepository) GetContestProblemProjections(
	ctx context.Context,
	contestID uuid.UUID,
) ([]ports.ContestProblemProjection, error) {
	if r == nil || r.client == nil || ctx == nil || contestID == uuid.Nil {
		return nil, entity.ErrContestProblemSnapshot
	}
	rows, err := r.client.DB(ctx).Contest.QueryContext(ctx, `
SELECT selected.id, selected.problem_id, owner.slug, revision.title,
       selected.display_order, selected.alias, selected.points,
       selected.visible_before_start
FROM contest_problems AS selected
JOIN problems AS owner ON owner.id = selected.problem_id
JOIN problem_revisions AS revision
  ON revision.id = selected.problem_revision_id
 AND revision.problem_id = selected.problem_id
WHERE selected.contest_id = $1
ORDER BY selected.display_order ASC, selected.id ASC
LIMIT $2`,
		contestID,
		maximumContestProblemProjections+1,
	)
	if err != nil {
		return nil, fmt.Errorf("read contest problem projection: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]ports.ContestProblemProjection, 0, 16)
	seenProblems := make(map[uuid.UUID]struct{})
	seenAliases := make(map[string]struct{})
	seenOrders := make(map[int]struct{})
	for rows.Next() {
		var value ports.ContestProblemProjection
		if err := rows.Scan(
			&value.ContestProblemID,
			&value.ProblemID,
			&value.ProblemSlug,
			&value.ProblemTitle,
			&value.DisplayOrder,
			&value.Alias,
			&value.Points,
			&value.VisibleBeforeStart,
		); err != nil {
			return nil, fmt.Errorf(
				"scan contest problem projection: %w",
				err,
			)
		}
		if value.ContestProblemID == uuid.Nil ||
			value.ProblemID == uuid.Nil ||
			value.ProblemSlug == "" ||
			value.ProblemTitle == "" ||
			value.DisplayOrder < 1 ||
			!contestProblemAliasPattern.MatchString(value.Alias) ||
			(value.Points != nil && *value.Points <= 0) {
			return nil, entity.ErrContestProblemSnapshot
		}
		if _, exists := seenProblems[value.ProblemID]; exists {
			return nil, entity.ErrContestProblemSnapshot
		}
		if _, exists := seenAliases[value.Alias]; exists {
			return nil, entity.ErrContestProblemSnapshot
		}
		if _, exists := seenOrders[value.DisplayOrder]; exists {
			return nil, entity.ErrContestProblemSnapshot
		}
		seenProblems[value.ProblemID] = struct{}{}
		seenAliases[value.Alias] = struct{}{}
		seenOrders[value.DisplayOrder] = struct{}{}
		result = append(result, value)
		if len(result) > maximumContestProblemProjections {
			return nil, entity.ErrContestProblemSnapshot
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read contest problem projection: %w", err)
	}
	return result, nil
}

func (r *ContestRepository) GetContestProblemSnapshotRef(
	ctx context.Context,
	contestID uuid.UUID,
	contestProblemID uuid.UUID,
) (ports.ContestProblemSnapshotRef, error) {
	if r == nil || r.client == nil || ctx == nil ||
		contestID == uuid.Nil || contestProblemID == uuid.Nil {
		return ports.ContestProblemSnapshotRef{},
			entity.ErrContestProblemSnapshot
	}
	rows, err := r.client.DB(ctx).Contest.QueryContext(ctx, `
SELECT contest.id, selected.id, selected.problem_id,
       selected.problem_revision_id, selected.testset_revision_id,
       owner.slug, contest.status, contest.start_time, contest.end_time,
       contest.cancelled_at, clock_timestamp(), selected.display_order,
       selected.alias, selected.points, selected.visible_before_start
FROM contest_problems AS selected
JOIN contests AS contest
  ON contest.id = selected.contest_id
 AND contest.deleted_at IS NULL
JOIN problems AS owner
  ON owner.id = selected.problem_id
JOIN problem_revisions AS revision
  ON revision.id = selected.problem_revision_id
 AND revision.problem_id = selected.problem_id
JOIN testset_revisions AS testset
  ON testset.id = selected.testset_revision_id
 AND testset.problem_id = selected.problem_id
WHERE contest.id = $1
  AND selected.id = $2
  AND contest.published_at IS NOT NULL
LIMIT 2`,
		contestID,
		contestProblemID,
	)
	if err != nil {
		return ports.ContestProblemSnapshotRef{}, fmt.Errorf(
			"read contest problem snapshot reference: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return ports.ContestProblemSnapshotRef{}, fmt.Errorf(
				"read contest problem snapshot reference: %w",
				err,
			)
		}
		return ports.ContestProblemSnapshotRef{},
			entity.ErrContestNotFound
	}

	var (
		value       ports.ContestProblemSnapshotRef
		state       string
		cancelledAt stdsql.NullTime
		points      stdsql.NullInt64
	)
	if err := rows.Scan(
		&value.ContestID,
		&value.ContestProblemID,
		&value.ProblemID,
		&value.ProblemRevisionID,
		&value.TestsetRevisionID,
		&value.ProblemSlug,
		&state,
		&value.StartTime,
		&value.EndTime,
		&cancelledAt,
		&value.DatabaseTime,
		&value.DisplayOrder,
		&value.Alias,
		&points,
		&value.VisibleBeforeStart,
	); err != nil {
		return ports.ContestProblemSnapshotRef{}, fmt.Errorf(
			"scan contest problem snapshot reference: %w",
			err,
		)
	}
	if rows.Next() {
		return ports.ContestProblemSnapshotRef{},
			entity.ErrContestProblemSnapshot
	}
	if err := rows.Err(); err != nil {
		return ports.ContestProblemSnapshotRef{}, fmt.Errorf(
			"read contest problem snapshot reference: %w",
			err,
		)
	}
	value.State = entity.ContestState(state)
	value.StartTime = value.StartTime.UTC()
	value.EndTime = value.EndTime.UTC()
	value.DatabaseTime = value.DatabaseTime.UTC()
	if cancelledAt.Valid {
		normalized := cancelledAt.Time.UTC()
		value.CancelledAt = &normalized
	}
	if points.Valid {
		if points.Int64 <= 0 {
			return ports.ContestProblemSnapshotRef{},
				entity.ErrContestProblemSnapshot
		}
		converted := int(points.Int64)
		value.Points = &converted
	}
	if value.ContestID != contestID ||
		value.ContestProblemID != contestProblemID ||
		value.ProblemID == uuid.Nil ||
		value.ProblemRevisionID == uuid.Nil ||
		value.TestsetRevisionID == uuid.Nil ||
		value.ProblemSlug == "" ||
		!value.State.Valid() ||
		value.State == entity.ContestDraft ||
		value.StartTime.IsZero() ||
		value.EndTime.IsZero() ||
		!value.StartTime.Before(value.EndTime) ||
		value.DatabaseTime.IsZero() ||
		value.DisplayOrder < 1 ||
		!contestProblemAliasPattern.MatchString(value.Alias) {
		return ports.ContestProblemSnapshotRef{},
			entity.ErrContestProblemSnapshot
	}
	return value, nil
}

func (r *ContestRepository) FindIDsByStatus(ctx context.Context, status string) ([]uuid.UUID, error) {
	ids, err := r.client.DB(ctx).Contest.Query().
		Where(contest.StatusEQ(contest.Status(status))).
		IDs(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, contestRepoName)
	}
	return ids, nil
}
