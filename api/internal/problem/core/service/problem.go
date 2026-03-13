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
	problemRepo ports.ProblemRepository
	tagRepo     ports.TagRepository
}

// NewProblemService creates a new ProblemService instance.
func NewProblemService(problemRepo ports.ProblemRepository, tagRepo ports.TagRepository) ports.ProblemService {
	return &problemService{problemRepo: problemRepo, tagRepo: tagRepo}
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
		// Attach tags
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
	if req.Difficulty != nil {
		problem.Difficulty = *req.Difficulty
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
