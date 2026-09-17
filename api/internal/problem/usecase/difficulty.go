package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

type difficultyService struct {
	difficultyRepo ports.DifficultyRepository
	cache          ports.DifficultyCache
}

// NewDifficultyService creates a new DifficultyService instance.
func NewDifficultyService(difficultyRepo ports.DifficultyRepository, cache ports.DifficultyCache) ports.DifficultyService {
	return &difficultyService{
		difficultyRepo: difficultyRepo,
		cache:          cache,
	}
}

// Find retrieves difficulties with pagination.
func (s *difficultyService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.DifficultyResult], error) {
	difficulties, err := s.difficultyRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if difficulties.Records == nil {
		return &d.Paginated[*contract.DifficultyResult]{
			Records:    &[]*contract.DifficultyResult{},
			Pagination: difficulties.Pagination,
		}, nil
	}

	entities := *difficulties.Records
	responses := make([]*contract.DifficultyResult, len(entities))
	for i, d := range entities {
		responses[i] = ToDifficultyResult(d)
	}

	return &d.Paginated[*contract.DifficultyResult]{
		Records:    &responses,
		Pagination: difficulties.Pagination,
	}, nil
}

// Get retrieves a difficulty by ID.
func (s *difficultyService) Get(ctx context.Context, id uuid.UUID) (*contract.DifficultyResult, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.DifficultyResult, error) {
		logger.FromContext(ctx).Debug("cache miss, loading difficulty from db", zap.String("difficulty_id", id.String()))
		difficulty, err := s.difficultyRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToDifficultyResult(difficulty), nil
	})
}

// Create creates a new difficulty.
func (s *difficultyService) Create(ctx context.Context, req *contract.CreateDifficultyCommand) (*contract.DifficultyResult, error) {
	difficulty := ToDifficultyEntityFromCreate(req)
	if err := s.difficultyRepo.Create(ctx, difficulty); err != nil {
		if errors.Is(err, ports.ErrDifficultyVersionConflict) {
			return nil, difficultyVersionConflict(err)
		}
		return nil, err
	}
	if err := s.cache.InvalidateAll(ctx); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("difficulty created", zap.String("difficulty_id", difficulty.ID.String()))
	return ToDifficultyResult(difficulty), nil
}

// Update updates an existing difficulty.
func (s *difficultyService) Update(ctx context.Context, id uuid.UUID, req *contract.UpdateDifficultyCommand) (*contract.DifficultyResult, error) {
	difficulty, err := s.difficultyRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		difficulty.Name = *req.Name
	}
	if req.Level != nil {
		difficulty.Level = *req.Level
	}
	if req.ExpReward != nil {
		difficulty.ExpReward = *req.ExpReward
	}
	if req.Description != nil {
		difficulty.Description = *req.Description
	}

	difficulty.ID = id
	if err := s.difficultyRepo.Update(ctx, difficulty); err != nil {
		if errors.Is(err, ports.ErrDifficultyVersionConflict) {
			return nil, difficultyVersionConflict(err)
		}
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("difficulty updated", zap.String("difficulty_id", difficulty.ID.String()))
	return ToDifficultyResult(difficulty), nil
}

func difficultyVersionConflict(cause error) error {
	return apperr.New(apperr.CodeConflict, "difficulty changed; reload and try again", cause).
		WithDetails(map[string]any{"reason": "version_conflict"})
}

// Delete removes a difficulty by ID.
func (s *difficultyService) Delete(ctx context.Context, id uuid.UUID) error {
	exists, err := s.difficultyRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.For(constant.ProblemObjDifficulty).NotFound(nil)
	}

	if err := s.difficultyRepo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("difficulty deleted", zap.String("difficulty_id", id.String()))
	return nil
}

// FindAll retrieves all difficulties without pagination.
func (s *difficultyService) FindAll(ctx context.Context) ([]*contract.DifficultyResult, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.DifficultyResult, error) {
		logger.FromContext(ctx).Debug("cache miss, loading difficulties from db")
		difficulties, err := s.difficultyRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*contract.DifficultyResult, len(difficulties))
		for i, d := range difficulties {
			responses[i] = ToDifficultyResult(d)
		}
		return responses, nil
	})
}
