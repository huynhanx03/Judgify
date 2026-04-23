package service

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/material/constant"
	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
	"github.com/huynhanx03/judgify/internal/material/core/mapper"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

type materialService struct {
	repo ports.MaterialRepository
}

// NewMaterialService creates a new MaterialService.
func NewMaterialService(repo ports.MaterialRepository) ports.MaterialService {
	return &materialService{repo: repo}
}

// calculateEstimatedReadTime estimates reading time based on content length.
// Formula: len(content) / 1000 (chars per minute), minimum 1 minute.
func calculateEstimatedReadTime(content string) int {
	if content == "" {
		return 0
	}
	minutes := len(content) / 1000
	if minutes < 1 {
		return 1
	}
	return minutes
}

func (s *materialService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.MaterialResponse], error) {
	paginated, err := s.repo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	records := *paginated.Records
	dtos := make([]*dto.MaterialResponse, len(records))
	for i, e := range records {
		dtos[i] = mapper.ToMaterialResponse(e)
	}

	return &d.Paginated[*dto.MaterialResponse]{
		Records:    &dtos,
		Pagination: paginated.Pagination,
	}, nil
}

func (s *materialService) Get(ctx context.Context, id int) (*dto.MaterialResponse, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Increment view count asynchronously
	go func() {
		_ = s.repo.IncrementViewCount(context.Background(), id)
	}()

	resp := mapper.ToMaterialResponse(e)
	if resp != nil {
		resp.ViewCount = e.ViewCount + 1
	}
	return resp, nil
}

func (s *materialService) Create(ctx context.Context, authorID int, req *dto.CreateMaterialRequest) (*dto.MaterialResponse, error) {
	status := constant.MaterialStatusDraft
	if req.Status != "" {
		status = req.Status
	}
	visibility := constant.MaterialVisibilityPublic
	if req.Visibility != "" {
		visibility = req.Visibility
	}

	e := &entity.Material{
		Title:             req.Title,
		Description:       req.Description,
		Content:           req.Content,
		DifficultyID:      req.DifficultyID,
		AuthorID:          authorID,
		CategoryID:        req.CategoryID,
		Status:            status,
		Visibility:        visibility,
		ViewCount:         0,
		EstimatedReadTime: calculateEstimatedReadTime(req.Content),
	}

	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}

	// Add tags if provided
	if len(req.TagIDs) > 0 {
		if err := s.repo.AddTags(ctx, e.ID, req.TagIDs); err != nil {
			return nil, err
		}
	}

	// Reload to get full entity with edges
	e, err := s.repo.Get(ctx, e.ID)
	if err != nil {
		return nil, err
	}

	return mapper.ToMaterialResponse(e), nil
}

func (s *materialService) Update(ctx context.Context, id int, req *dto.UpdateMaterialRequest) (*dto.MaterialResponse, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	contentChanged := false

	if req.Title != nil {
		e.Title = *req.Title
	}
	if req.Description != nil {
		e.Description = *req.Description
	}
	if req.Content != nil {
		e.Content = *req.Content
		contentChanged = true
	}
	if req.DifficultyID != nil {
		e.DifficultyID = *req.DifficultyID
	}
	if req.CategoryID != nil {
		e.CategoryID = *req.CategoryID
	}
	if req.Status != nil {
		e.Status = *req.Status
	}
	if req.Visibility != nil {
		e.Visibility = *req.Visibility
	}

	if contentChanged {
		e.EstimatedReadTime = calculateEstimatedReadTime(e.Content)
	}

	if err := s.repo.Update(ctx, e); err != nil {
		return nil, err
	}

	// Replace tags if provided
	if req.TagIDs != nil {
		if err := s.repo.RemoveAllTags(ctx, id); err != nil {
			return nil, err
		}
		if len(*req.TagIDs) > 0 {
			if err := s.repo.AddTags(ctx, id, *req.TagIDs); err != nil {
				return nil, err
			}
		}
	}

	// Reload to get full entity with edges
	e, err = s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return mapper.ToMaterialResponse(e), nil
}

func (s *materialService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
