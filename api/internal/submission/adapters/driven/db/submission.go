package db

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent/dialect/sql"

	commonEnt "github.com/huynhanx03/go-common/pkg/database/ent"
	commondto "github.com/huynhanx03/go-common/pkg/dto"

	dbEnt "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/submission"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

const (
	submissionRepoName         = "Submission"
	maximumSubmissionSummaries = 100
)

type SubmissionRepository struct {
	client     *dbEnt.EntClient
	clock      func() time.Time
	judgeToken func() uuid.UUID
}

func NewSubmissionRepository(client *dbEnt.EntClient) *SubmissionRepository {
	return &SubmissionRepository{
		client:     client,
		clock:      func() time.Time { return time.Now().UTC() },
		judgeToken: uuid.New,
	}
}

func (r *SubmissionRepository) Get(ctx context.Context, id uuid.UUID) (*entity.Submission, error) {
	record, err := r.client.DB(ctx).Submission.Get(ctx, id)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}
	projected := ToSubmissionEntity(record)
	if projected == nil {
		return nil, fmt.Errorf("map submission aggregate: empty record")
	}
	// Read the operational fields from the same aggregate. This remains a raw
	// projection so the read repository can be deployed across an Ent
	// code-generation boundary without weakening its API contract.
	if err := r.hydrateOperationalAggregate(ctx, projected); err != nil {
		return nil, err
	}
	return projected, nil
}

func (r *SubmissionRepository) hydrateOperationalAggregate(
	ctx context.Context,
	submissionEntity *entity.Submission,
) error {
	if submissionEntity == nil || submissionEntity.ID == uuid.Nil {
		return fmt.Errorf("hydrate submission aggregate: invalid submission")
	}
	rows, err := r.client.DB(ctx).Submission.QueryContext(ctx, `
		SELECT version, latest_generation, latest_generation_phase,
		       latest_generation_outcome, latest_generation_queued_at,
		       latest_generation_started_at, latest_generation_completed_at
		FROM submissions
		WHERE id = $1`, submissionEntity.ID)
	if err != nil {
		return fmt.Errorf("read submission operational aggregate: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return fmt.Errorf("read submission operational aggregate rows: %w", err)
		}
		return fmt.Errorf("read submission operational aggregate: missing submission")
	}
	var phase string
	var outcome stdsql.NullString
	if err := rows.Scan(
		&submissionEntity.Version,
		&submissionEntity.LatestGeneration,
		&phase,
		&outcome,
		&submissionEntity.LatestGenerationQueuedAt,
		&submissionEntity.LatestGenerationStartedAt,
		&submissionEntity.LatestGenerationCompletedAt,
	); err != nil {
		return fmt.Errorf("scan submission operational aggregate: %w", err)
	}
	if rows.Next() {
		return fmt.Errorf("read submission operational aggregate: duplicate submission")
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read submission operational aggregate rows: %w", err)
	}
	submissionEntity.LatestGenerationPhase = entity.Phase(phase)
	if outcome.Valid {
		submissionEntity.LatestGenerationOutcome = outcome.String
	}
	return nil
}

func (r *SubmissionRepository) GetOwned(ctx context.Context, id, userID uuid.UUID) (*entity.Submission, error) {
	record, err := r.client.DB(ctx).Submission.Query().
		Where(submission.ID(id), submission.UserID(userID)).
		Only(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}
	return ToSubmissionEntity(record), nil
}

func (r *SubmissionRepository) FindByProblemID(ctx context.Context, problemID uuid.UUID) ([]*entity.Submission, error) {
	return r.findSummaries(ctx, submission.ProblemID(problemID))
}

var submissionSummaryFields = []string{
	submission.FieldID,
	submission.FieldCreatedAt,
	submission.FieldProblemID,
	submission.FieldUserID,
	submission.FieldLanguage,
	submission.FieldStatus,
	submission.FieldPassedCount,
	submission.FieldTotalCount,
	submission.FieldTimeMs,
	submission.FieldMemoryKB,
	submission.FieldContestID,
}

// FindMyByProblem uses a composite seek over the same total descending order
// exposed by the API. Reading one extra row establishes has_more without a
// count query or an unbounded OFFSET scan.
func (r *SubmissionRepository) FindMyByProblem(
	ctx context.Context,
	query ports.MySubmissionQuery,
) (ports.MySubmissionPage, error) {
	if r == nil || r.client == nil || ctx == nil || query.Validate() != nil {
		return ports.MySubmissionPage{}, ports.ErrInvalidMySubmissionPage
	}
	predicates := []predicate.Submission{
		submission.UserID(query.UserID),
		submission.ProblemID(query.ProblemID),
	}
	if query.ContestID != nil {
		predicates = append(
			predicates,
			submission.ContestID(*query.ContestID),
		)
	}
	if query.After != nil {
		afterTime := query.After.CreatedAt.UTC()
		predicates = append(predicates, submission.Or(
			submission.CreatedAtLT(afterTime),
			submission.And(
				submission.CreatedAtEQ(afterTime),
				submission.IDLT(query.After.ID),
			),
		))
	}
	records, err := r.client.DB(ctx).Submission.Query().
		Where(predicates...).
		Select(submissionSummaryFields...).
		Order(
			submission.ByCreatedAt(sql.OrderDesc()),
			submission.ByID(sql.OrderDesc()),
		).
		Limit(query.Limit + 1).
		All(ctx)
	if err != nil {
		return ports.MySubmissionPage{}, commonEnt.MapEntError(err, submissionRepoName)
	}
	page := ports.MySubmissionPage{
		Submissions: make([]*entity.Submission, 0, min(len(records), query.Limit)),
	}
	for index, record := range records {
		if index == query.Limit {
			break
		}
		projected := ToSubmissionEntity(record)
		if projected == nil || projected.ID == uuid.Nil ||
			projected.UserID != query.UserID ||
			projected.ProblemID != query.ProblemID ||
			(query.ContestID != nil &&
				(projected.ContestID == nil ||
					*projected.ContestID != *query.ContestID)) ||
			projected.CreatedAt.IsZero() {
			return ports.MySubmissionPage{}, ports.ErrInvalidMySubmissionPage
		}
		page.Submissions = append(page.Submissions, projected)
	}
	if len(records) > query.Limit {
		last := page.Submissions[len(page.Submissions)-1]
		page.Next = &ports.SubmissionCursor{
			CreatedAt: last.CreatedAt.UTC(),
			ID:        last.ID,
		}
	}
	return page, nil
}

var adminSubmissionSummaryFields = []string{
	submission.FieldID,
	submission.FieldCreatedAt,
	submission.FieldUpdatedAt,
	submission.FieldProblemID,
	submission.FieldUserID,
	submission.FieldLanguage,
	submission.FieldStatus,
	submission.FieldPassedCount,
	submission.FieldTotalCount,
	submission.FieldTimeMs,
	submission.FieldMemoryKB,
	submission.FieldContestID,
	submission.FieldLatestGenerationStartedAt,
	submission.FieldJudgedAt,
	submission.FieldVerdictVersion,
}

func (r *SubmissionRepository) findSummaries(ctx context.Context, predicates ...predicate.Submission) ([]*entity.Submission, error) {
	records, err := r.client.DB(ctx).Submission.Query().
		Where(predicates...).
		Select(submissionSummaryFields...).
		Order(
			submission.ByCreatedAt(sql.OrderDesc()),
			submission.ByID(sql.OrderDesc()),
		).
		Limit(maximumSubmissionSummaries).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}

	entities := make([]*entity.Submission, len(records))
	for i, record := range records {
		entities[i] = ToSubmissionEntity(record)
	}
	return entities, nil
}

// FindAdmin implements the typed monitoring read model. The list projection
// never selects source code, diagnostics, bundle IDs, lease data, or rewards.
func (r *SubmissionRepository) FindAdmin(
	ctx context.Context,
	query entity.AdminSubmissionQuery,
) (*commondto.Paginated[*entity.Submission], error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	predicates := make([]predicate.Submission, 0, 5)
	if query.Status != "" {
		predicates = append(predicates, submission.StatusEQ(submission.Status(query.Status)))
	}
	if query.Language != "" {
		predicates = append(predicates, submission.LanguageEQ(submission.Language(query.Language)))
	}
	if query.ProblemID != nil {
		predicates = append(predicates, submission.ProblemID(*query.ProblemID))
	}
	if query.UserID != nil {
		predicates = append(predicates, submission.UserID(*query.UserID))
	}
	if query.ContestID != nil {
		predicates = append(predicates, submission.ContestID(*query.ContestID))
	}

	base := r.client.DB(ctx).Submission.Query().Where(predicates...)
	total, err := base.Clone().Count(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}
	meta, err := commondto.CalculatePaginationChecked(query.Page, query.PageSize, int64(total))
	if err != nil {
		return nil, entity.ErrInvalidAdminSubmissionQuery
	}

	query.Page = meta.CurrentPage
	records, err := base.
		Select(adminSubmissionSummaryFields...).
		Order(
			submission.ByCreatedAt(sql.OrderDesc()),
			submission.ByID(sql.OrderDesc()),
		).
		Limit(query.PageSize).
		Offset(query.Offset()).
		All(ctx)
	if err != nil {
		return nil, commonEnt.MapEntError(err, submissionRepoName)
	}

	entities := make([]*entity.Submission, len(records))
	for index, record := range records {
		entities[index] = ToSubmissionEntity(record)
	}
	return &commondto.Paginated[*entity.Submission]{
		Records:    &entities,
		Pagination: meta,
	}, nil
}
