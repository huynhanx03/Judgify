package usecase

import (
	"context"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type materialCategoryService struct {
	repo    ports.MaterialCategoryRepository
	cache   ports.MaterialCategoryCache
	journal platformmutation.Executor
}

// NewMaterialCategoryService creates a new MaterialCategoryService.
func NewMaterialCategoryService(
	repo ports.MaterialCategoryRepository,
	cache ports.MaterialCategoryCache,
	journals ...platformmutation.Executor,
) ports.MaterialCategoryService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &materialCategoryService{repo: repo, cache: cache, journal: journal}
}

func (s *materialCategoryService) FindAll(ctx context.Context) ([]*contract.MaterialCategoryResult, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.MaterialCategoryResult, error) {
		logger.FromContext(ctx).Debug("cache miss, loading material categories from db")

		categories, err := s.repo.FindAll(ctx)
		if err != nil {
			return nil, err
		}

		result := make([]*contract.MaterialCategoryResult, len(categories))
		for i, c := range categories {
			result[i] = ToMaterialCategoryResult(c)
		}
		return result, nil
	})
}

func (s *materialCategoryService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.MaterialCategoryResult], error) {
	paginated, err := s.repo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	records := *paginated.Records
	dtos := make([]*contract.MaterialCategoryResult, len(records))
	for i, e := range records {
		dtos[i] = ToMaterialCategoryResult(e)
	}

	return &d.Paginated[*contract.MaterialCategoryResult]{
		Records:    &dtos,
		Pagination: paginated.Pagination,
	}, nil
}

func (s *materialCategoryService) Get(ctx context.Context, id uuid.UUID) (*contract.MaterialCategoryResult, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.MaterialCategoryResult, error) {
		logger.FromContext(ctx).Debug("cache miss, loading material category from db", zap.String("category_id", id.String()))

		e, err := s.repo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToMaterialCategoryResult(e), nil
	})
}

func (s *materialCategoryService) Create(ctx context.Context, req *contract.CreateMaterialCategoryCommand) (*contract.MaterialCategoryResult, error) {
	e := &entity.MaterialCategory{
		Name:        req.Name,
		Description: req.Description,
	}

	err := executeMaterialMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.repo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		after := materialCategoryDocument(e)
		return mutationCommand("material_category.created.v1", materialCategoryAggregate, e.ID,
			"material category created", map[string]any{}, after), nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.cache.Invalidate(ctx, e.ID); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("material category created successfully", zap.String("category_id", e.ID.String()))

	return ToMaterialCategoryResult(e), nil
}

func (s *materialCategoryService) Update(ctx context.Context, id uuid.UUID, req *contract.UpdateMaterialCategoryCommand) (*contract.MaterialCategoryResult, error) {
	var e *entity.MaterialCategory
	err := executeMaterialMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var err error
		e, err = s.repo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		before := materialCategoryDocument(e)
		if req.Name != nil {
			e.Name = *req.Name
		}
		if req.Description != nil {
			e.Description = *req.Description
		}
		if err := s.repo.Update(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		after := materialCategoryDocument(e)
		return mutationCommand("material_category.updated.v1", materialCategoryAggregate, id,
			"material category updated", before, after), nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("material category updated successfully", zap.String("category_id", id.String()))

	return ToMaterialCategoryResult(e), nil
}

func (s *materialCategoryService) Delete(ctx context.Context, id uuid.UUID) error {
	err := executeMaterialMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		before := map[string]any{}
		if s.journal != nil {
			stored, err := s.repo.Get(txCtx, id)
			if err != nil {
				return platformmutation.Command{}, err
			}
			before = materialCategoryDocument(stored)
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return platformmutation.Command{}, err
		}
		return mutationCommand("material_category.deleted.v1", materialCategoryAggregate, id,
			"material category deleted", before, map[string]any{}), nil
	})
	if err != nil {
		return err
	}

	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("material category deleted successfully", zap.String("category_id", id.String()))

	return nil
}
