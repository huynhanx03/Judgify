package service

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/cultivation/constant"
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)


type levelService struct {
	levelRepo ports.LevelRepository
}

// NewLevelService creates a new LevelService instance.
func NewLevelService(levelRepo ports.LevelRepository) ports.LevelService {
	return &levelService{levelRepo: levelRepo}
}

func (s *levelService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.LevelResponse], error) {
	result, err := s.levelRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.LevelResponse]{Records: &[]*dto.LevelResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.LevelResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToLevelResponse(e)
	}

	return &d.Paginated[*dto.LevelResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *levelService) Get(ctx context.Context, id int) (*dto.LevelResponse, error) {
	e, err := s.levelRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToLevelResponse(e), nil
}

func (s *levelService) Create(ctx context.Context, req *dto.CreateLevelRequest) (*dto.LevelResponse, error) {
	e := mapper.ToLevelEntityFromCreate(req)
	if err := s.levelRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("level created", zap.Int("level_id", e.ID))
	return mapper.ToLevelResponse(e), nil
}

func (s *levelService) Update(ctx context.Context, id int, req *dto.UpdateLevelRequest) (*dto.LevelResponse, error) {
	e, err := s.levelRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.MinExp != nil {
		e.MinExp = *req.MinExp
	}
	if req.Description != nil {
		e.Description = *req.Description
	}

	if err := s.levelRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("level updated", zap.Int("level_id", e.ID))
	return mapper.ToLevelResponse(e), nil
}

func (s *levelService) Delete(ctx context.Context, id int) error {
	exists, err := s.levelRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, constant.MsgLevelNotFound, nil)
	}
	if err := s.levelRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("level deleted", zap.Int("level_id", id))
	return nil
}
