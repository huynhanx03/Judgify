package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type problemService struct {
	problemRepo    ports.ProblemRepository
	tagRepo        ports.TagRepository
	difficultyRepo ports.DifficultyRepository
	txMgr          tx.Manager
}

// NewProblemService creates a new ProblemService instance.
func NewProblemService(
	problemRepo ports.ProblemRepository,
	tagRepo ports.TagRepository,
	difficultyRepo ports.DifficultyRepository,
	txMgr tx.Manager,
) ports.ProblemService {
	return &problemService{
		problemRepo:    problemRepo,
		tagRepo:        tagRepo,
		difficultyRepo: difficultyRepo,
		txMgr:          txMgr,
	}
}

// enrichSolvedStatus sets IsSolved on entities if user is authenticated.
func (s *problemService) enrichSolvedStatus(ctx context.Context, entities []*entity.Problem) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return
	}
	ids := make([]uuid.UUID, len(entities))
	for i, p := range entities {
		ids[i] = p.ID
	}
	solvedMap, err := s.problemRepo.GetSolvedProblemIDs(ctx, userID, ids)
	if err != nil {
		return
	}
	for _, p := range entities {
		solved := solvedMap[p.ID]
		p.IsSolved = &solved
	}
}

// Find retrieves problems with pagination.
// Tags (with elements) and difficulty are eager-loaded by the repository — no N+1.
func (s *problemService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ProblemResult], error) {
	problems, err := s.problemRepo.FindPublished(ctx, opts)
	if err != nil {
		return nil, err
	}

	if problems.Records == nil {
		return &d.Paginated[*contract.ProblemResult]{
			Records:    &[]*contract.ProblemResult{},
			Pagination: problems.Pagination,
		}, nil
	}

	entities := *problems.Records
	s.enrichSolvedStatus(ctx, entities)

	responses := make([]*contract.ProblemResult, len(entities))
	for i, p := range entities {
		responses[i] = ToProblemResult(p)
	}

	return &d.Paginated[*contract.ProblemResult]{
		Records:    &responses,
		Pagination: problems.Pagination,
	}, nil
}

// Get retrieves a problem by ID.
func (s *problemService) Get(ctx context.Context, id uuid.UUID) (*contract.ProblemResult, error) {
	problem, err := s.problemRepo.GetPublished(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichSolvedStatus(ctx, []*entity.Problem{problem})
	return ToProblemResult(problem), nil
}

// FindAdmin retrieves every problem status for authorized administration.
func (s *problemService) FindAdmin(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ProblemResult], error) {
	problems, err := s.problemRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}
	if problems.Records == nil {
		responses := []*contract.ProblemResult{}
		return &d.Paginated[*contract.ProblemResult]{Records: &responses, Pagination: problems.Pagination}, nil
	}

	entities := *problems.Records
	responses := make([]*contract.ProblemResult, len(entities))
	for i, problem := range entities {
		responses[i] = ToProblemResult(problem)
	}
	return &d.Paginated[*contract.ProblemResult]{Records: &responses, Pagination: problems.Pagination}, nil
}

// GetAdmin retrieves a problem regardless of publication status.
func (s *problemService) GetAdmin(ctx context.Context, id uuid.UUID) (*contract.ProblemResult, error) {
	problem, err := s.problemRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToProblemResult(problem), nil
}

// Create creates a new problem with optional tags.
func (s *problemService) Create(ctx context.Context, authorID uuid.UUID, req *contract.CreateProblemCommand) (*contract.ProblemResult, error) {
	if req == nil || authorID == uuid.Nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"problem create request and author are required",
			nil,
		)
	}
	problem, err := ToProblemEntityFromCreate(authorID, req)
	if err != nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"problem stable identity is invalid",
			err,
		)
	}

	var created *entity.Problem
	if err := s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		if err := s.problemRepo.Create(txCtx, problem); err != nil {
			return err
		}
		if len(req.TagIDs) > 0 {
			if err := s.problemRepo.AddTags(txCtx, problem.ID, req.TagIDs); err != nil {
				return err
			}
		}
		var err error
		created, err = s.problemRepo.Get(txCtx, problem.ID)
		return err
	}); err != nil {
		if errors.Is(err, ports.ErrProblemSlugConflict) {
			return nil, apperr.New(
				apperr.CodeConflict,
				"problem slug is already in use",
				err,
			).WithDetails(map[string]any{
				"field":  "slug",
				"reason": "already_exists",
			})
		}
		return nil, err
	}

	logger.FromContext(ctx).Info("problem created successfully", zap.String("problem_id", problem.ID.String()), zap.String("author_id", authorID.String()))
	return ToProblemResult(created), nil
}
