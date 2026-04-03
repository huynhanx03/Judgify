package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/problem/core/mapper"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)


type problemService struct {
	problemRepo    ports.ProblemRepository
	tagRepo        ports.TagRepository
	difficultyRepo ports.DifficultyRepository
}

// NewProblemService creates a new ProblemService instance.
func NewProblemService(problemRepo ports.ProblemRepository, tagRepo ports.TagRepository, difficultyRepo ports.DifficultyRepository) ports.ProblemService {
	return &problemService{problemRepo: problemRepo, tagRepo: tagRepo, difficultyRepo: difficultyRepo}
}

// enrichSolvedStatus sets IsSolved on entities if user is authenticated.
func (s *problemService) enrichSolvedStatus(ctx context.Context, entities []*entity.Problem) {
	userID, ok := ctx.Value(constraints.ContextKeyUserID).(int)
	if !ok || userID == 0 {
		return
	}
	ids := make([]int, len(entities))
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
func (s *problemService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ProblemResponse], error) {
	problems, err := s.problemRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if problems.Records == nil {
		return &d.Paginated[*dto.ProblemResponse]{
			Records:    &[]*dto.ProblemResponse{},
			Pagination: problems.Pagination,
		}, nil
	}

	entities := *problems.Records
	s.enrichSolvedStatus(ctx, entities)

	responses := make([]*dto.ProblemResponse, len(entities))
	for i, p := range entities {
		responses[i] = mapper.ToProblemResponse(p)
	}

	return &d.Paginated[*dto.ProblemResponse]{
		Records:    &responses,
		Pagination: problems.Pagination,
	}, nil
}

// Get retrieves a problem by ID.
func (s *problemService) Get(ctx context.Context, id int) (*dto.ProblemResponse, error) {
	problem, err := s.problemRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.enrichSolvedStatus(ctx, []*entity.Problem{problem})
	return mapper.ToProblemResponse(problem), nil
}

// Create creates a new problem with optional tags.
func (s *problemService) Create(ctx context.Context, authorID int, req *dto.CreateProblemRequest) (*dto.ProblemResponse, error) {
	problem := mapper.ToProblemEntityFromCreate(authorID, req)

	if err := s.problemRepo.Create(ctx, problem); err != nil {
		return nil, err
	}

	// Attach tags if provided
	if len(req.TagIDs) > 0 {
		if err := s.problemRepo.AddTags(ctx, problem.ID, req.TagIDs); err != nil {
			return nil, err
		}
	}

	// Re-fetch to get full eager-loaded data (tags with elements, difficulty)
	created, err := s.problemRepo.Get(ctx, problem.ID)
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("problem created successfully", zap.Int("problem_id", problem.ID), zap.Int("author_id", authorID))
	return mapper.ToProblemResponse(created), nil
}

// Update updates an existing problem.
func (s *problemService) Update(ctx context.Context, id int, req *dto.UpdateProblemRequest) (*dto.ProblemResponse, error) {
	problem, err := s.problemRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		problem.Title = *req.Title
	}
	if req.Description != nil {
		problem.Description = *req.Description
	}
	if req.DifficultyID != nil {
		problem.DifficultyID = *req.DifficultyID
	}
	if req.TimeLimitMs != nil {
		problem.TimeLimitMs = *req.TimeLimitMs
	}
	if req.MemoryLimitKb != nil {
		problem.MemoryLimitKb = *req.MemoryLimitKb
	}
	if req.IsPublished != nil {
		problem.IsPublished = *req.IsPublished
	}

	problem.ID = id
	if err := s.problemRepo.Update(ctx, problem); err != nil {
		return nil, err
	}

	// Replace tags if provided
	if req.TagIDs != nil {
		if err := s.problemRepo.ReplaceTags(ctx, id, *req.TagIDs); err != nil {
			return nil, err
		}
	}

	// Re-fetch to get full eager-loaded data
	updated, err := s.problemRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("problem updated successfully", zap.Int("problem_id", id))
	return mapper.ToProblemResponse(updated), nil
}

// Delete removes a problem by ID.
func (s *problemService) Delete(ctx context.Context, id int) error {
	exists, err := s.problemRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjProblem), nil)
	}

	if err := s.problemRepo.Delete(ctx, id); err != nil {
		return err
	}

	logger.FromContext(ctx).Info("problem deleted successfully", zap.Int("problem_id", id))
	return nil
}
