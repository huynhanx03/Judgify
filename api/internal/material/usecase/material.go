package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type materialService struct {
	repo    ports.MaterialRepository
	journal platformmutation.Executor
}

// NewMaterialService creates a new MaterialService.
func NewMaterialService(repo ports.MaterialRepository, journals ...platformmutation.Executor) ports.MaterialService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &materialService{repo: repo, journal: journal}
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

func mapMaterialPage(paginated *d.Paginated[*entity.Material]) *d.Paginated[*contract.MaterialResponse] {
	records := []*entity.Material{}
	if paginated.Records != nil {
		records = *paginated.Records
	}
	responses := make([]*contract.MaterialResponse, len(records))
	for i, record := range records {
		responses[i] = ToMaterialResponse(record)
	}
	return &d.Paginated[*contract.MaterialResponse]{
		Records:    &responses,
		Pagination: paginated.Pagination,
	}
}

func (s *materialService) FindPublic(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.MaterialResponse], error) {
	paginated, err := s.repo.FindPublic(ctx, opts)
	if err != nil {
		return nil, err
	}
	return mapMaterialPage(paginated), nil
}

func (s *materialService) FindAdmin(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.MaterialResponse], error) {
	paginated, err := s.repo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}
	return mapMaterialPage(paginated), nil
}

func (s *materialService) GetPublic(ctx context.Context, id uuid.UUID) (*contract.MaterialResponse, error) {
	e, err := s.repo.GetPublic(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := ToMaterialResponse(e)
	if incrementErr := s.repo.IncrementViewCount(ctx, id); incrementErr != nil {
		logger.FromContext(ctx).Warn(
			"material view count increment failed",
			zap.String("material_id", id.String()),
			zap.Error(incrementErr),
		)
	} else if resp != nil {
		resp.ViewCount = e.ViewCount + 1
	}
	return resp, nil
}

func (s *materialService) GetAdmin(ctx context.Context, id uuid.UUID) (*contract.MaterialResponse, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToMaterialResponse(e), nil
}

func (s *materialService) Create(ctx context.Context, authorID uuid.UUID, req *contract.CreateMaterialCommand) (*contract.MaterialResponse, error) {
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

	var created *entity.Material
	err := executeMaterialMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.repo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		if len(req.TagIDs) > 0 {
			if err := s.repo.AddTags(txCtx, e.ID, req.TagIDs); err != nil {
				return platformmutation.Command{}, err
			}
		}
		var err error
		created, err = s.repo.Get(txCtx, e.ID)
		if err != nil {
			return platformmutation.Command{}, err
		}
		after := materialDocument(created)
		return mutationCommand("material.created.v1", materialAggregate, created.ID,
			"material created", map[string]any{}, after), nil
	})
	if err != nil {
		return nil, err
	}

	return ToMaterialResponse(created), nil
}

func (s *materialService) Update(ctx context.Context, id uuid.UUID, req *contract.UpdateMaterialCommand) (*contract.MaterialResponse, error) {
	var updated *entity.Material
	err := executeMaterialMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		e, err := s.repo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		before := materialDocument(e)
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
		if err := s.repo.Update(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		if req.TagIDs != nil {
			if err := s.repo.RemoveAllTags(txCtx, id); err != nil {
				return platformmutation.Command{}, err
			}
			if len(*req.TagIDs) > 0 {
				if err := s.repo.AddTags(txCtx, id, *req.TagIDs); err != nil {
					return platformmutation.Command{}, err
				}
			}
		}
		updated, err = s.repo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		after := materialDocument(updated)
		return mutationCommand("material.updated.v1", materialAggregate, id,
			"material updated", before, after), nil
	})
	if err != nil {
		return nil, err
	}
	return ToMaterialResponse(updated), nil
}

func (s *materialService) Delete(ctx context.Context, id uuid.UUID) error {
	return executeMaterialMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		before := map[string]any{}
		if s.journal != nil {
			stored, err := s.repo.Get(txCtx, id)
			if err != nil {
				return platformmutation.Command{}, err
			}
			before = materialDocument(stored)
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return platformmutation.Command{}, err
		}
		return mutationCommand("material.deleted.v1", materialAggregate, id,
			"material deleted", before, map[string]any{}), nil
	})
}
