package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/mapper"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)


type difficultyService struct {
	difficultyRepo ports.DifficultyRepository
}

// NewDifficultyService creates a new DifficultyService instance.
func NewDifficultyService(difficultyRepo ports.DifficultyRepository) ports.DifficultyService {
	return &difficultyService{difficultyRepo: difficultyRepo}
}

// Find retrieves difficulties with pagination.
func (s *difficultyService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.DifficultyResponse], error) {
	difficulties, err := s.difficultyRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if difficulties.Records == nil {
		return &d.Paginated[*dto.DifficultyResponse]{
			Records:    &[]*dto.DifficultyResponse{},
			Pagination: difficulties.Pagination,
		}, nil
	}

	entities := *difficulties.Records
	responses := make([]*dto.DifficultyResponse, len(entities))
	for i, d := range entities {
		responses[i] = mapper.ToDifficultyResponse(d)
	}

	return &d.Paginated[*dto.DifficultyResponse]{
		Records:    &responses,
		Pagination: difficulties.Pagination,
	}, nil
}

// Get retrieves a difficulty by ID.
func (s *difficultyService) Get(ctx context.Context, id int) (*dto.DifficultyResponse, error) {
	difficulty, err := s.difficultyRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToDifficultyResponse(difficulty), nil
}

// Create creates a new difficulty.
func (s *difficultyService) Create(ctx context.Context, req *dto.CreateDifficultyRequest) (*dto.DifficultyResponse, error) {
	difficulty := mapper.ToDifficultyEntityFromCreate(req)
	if err := s.difficultyRepo.Create(ctx, difficulty); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("difficulty created", zap.Int("difficulty_id", difficulty.ID))
	return mapper.ToDifficultyResponse(difficulty), nil
}

// Update updates an existing difficulty.
func (s *difficultyService) Update(ctx context.Context, id int, req *dto.UpdateDifficultyRequest) (*dto.DifficultyResponse, error) {
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
		return nil, err
	}

	logger.FromContext(ctx).Info("difficulty updated", zap.Int("difficulty_id", difficulty.ID))
	return mapper.ToDifficultyResponse(difficulty), nil
}

// Delete removes a difficulty by ID.
func (s *difficultyService) Delete(ctx context.Context, id int) error {
	exists, err := s.difficultyRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjDifficulty), nil)
	}

	if err := s.difficultyRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("difficulty deleted", zap.Int("difficulty_id", id))
	return nil
}

// FindAll retrieves all difficulties without pagination.
func (s *difficultyService) FindAll(ctx context.Context) ([]*dto.DifficultyResponse, error) {
	difficulties, err := s.difficultyRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.DifficultyResponse, len(difficulties))
	for i, d := range difficulties {
		responses[i] = mapper.ToDifficultyResponse(d)
	}
	return responses, nil
}
