package service

import (
	"context"
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/mapper"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

const problemServiceName = "ProblemService"

type problemService struct {
	problemRepo    ports.ProblemRepository
	tagRepo        ports.TagRepository
	difficultyRepo ports.DifficultyRepository
}

// NewProblemService creates a new ProblemService instance.
func NewProblemService(problemRepo ports.ProblemRepository, tagRepo ports.TagRepository, difficultyRepo ports.DifficultyRepository) ports.ProblemService {
	return &problemService{problemRepo: problemRepo, tagRepo: tagRepo, difficultyRepo: difficultyRepo}
}

// Find retrieves problems with pagination.
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
	responses := make([]*dto.ProblemResponse, len(entities))
	for i, p := range entities {
		resp := mapper.ToProblemResponse(p)
		s.attachDifficulty(ctx, resp, p.DifficultyID)
		tags, err := s.getTagResponses(ctx, p.ID)
		if err == nil {
			resp.Tags = tags
		}
		responses[i] = resp
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

	resp := mapper.ToProblemResponse(problem)
	s.attachDifficulty(ctx, resp, problem.DifficultyID)
	tags, err := s.getTagResponses(ctx, id)
	if err == nil {
		resp.Tags = tags
	}
	return resp, nil
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

	resp := mapper.ToProblemResponse(problem)
	s.attachDifficulty(ctx, resp, problem.DifficultyID)
	tags, err := s.getTagResponses(ctx, problem.ID)
	if err == nil {
		resp.Tags = tags
	}
	return resp, nil
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

	resp := mapper.ToProblemResponse(problem)
	s.attachDifficulty(ctx, resp, problem.DifficultyID)
	tags, tagErr := s.getTagResponses(ctx, id)
	if tagErr == nil {
		resp.Tags = tags
	}
	return resp, nil
}

// Delete removes a problem by ID.
func (s *problemService) Delete(ctx context.Context, id int) error {
	exists, err := s.problemRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.NewError(problemServiceName, response.CodeNotFound, apperr.MsgNotFound, http.StatusNotFound, nil)
	}

	return s.problemRepo.Delete(ctx, id)
}

// attachDifficulty fetches and attaches difficulty to a problem response.
func (s *problemService) attachDifficulty(ctx context.Context, resp *dto.ProblemResponse, difficultyID int) {
	difficulty, err := s.difficultyRepo.Get(ctx, difficultyID)
	if err == nil {
		resp.Difficulty = mapper.ToDifficultyResponse(difficulty)
	}
}

// getTagResponses fetches tag responses for a problem.
func (s *problemService) getTagResponses(ctx context.Context, problemID int) ([]*dto.TagResponse, error) {
	tagIDs, err := s.problemRepo.GetTagIDs(ctx, problemID)
	if err != nil {
		return nil, err
	}
	if len(tagIDs) == 0 {
		return []*dto.TagResponse{}, nil
	}

	tags, err := s.tagRepo.FindByIDs(ctx, tagIDs)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.TagResponse, len(tags))
	for i, t := range tags {
		responses[i] = mapper.ToTagResponse(t)
	}
	return responses, nil
}
