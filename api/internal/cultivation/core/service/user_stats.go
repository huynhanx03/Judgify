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


type userStatsService struct {
	userStatsRepo ports.UserStatsRepository
}

// NewUserStatsService creates a new UserStatsService instance.
func NewUserStatsService(repo ports.UserStatsRepository) ports.UserStatsService {
	return &userStatsService{userStatsRepo: repo}
}

func (s *userStatsService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserStatsResponse], error) {
	result, err := s.userStatsRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.UserStatsResponse]{Records: &[]*dto.UserStatsResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.UserStatsResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToUserStatsResponse(e)
	}

	return &d.Paginated[*dto.UserStatsResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *userStatsService) Get(ctx context.Context, id int) (*dto.UserStatsResponse, error) {
	e, err := s.userStatsRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToUserStatsResponse(e), nil
}

func (s *userStatsService) Create(ctx context.Context, req *dto.CreateUserStatsRequest) (*dto.UserStatsResponse, error) {
	e := mapper.ToUserStatsEntityFromCreate(req)
	if err := s.userStatsRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("user stats created", zap.Int("user_stats_id", e.ID), zap.Int("user_id", e.UserID))
	return mapper.ToUserStatsResponse(e), nil
}

func (s *userStatsService) Update(ctx context.Context, id int, req *dto.UpdateUserStatsRequest) (*dto.UserStatsResponse, error) {
	e, err := s.userStatsRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.TotalExp != nil {
		e.TotalExp = *req.TotalExp
	}
	if req.CurrentLevelID != nil {
		e.CurrentLevelID = *req.CurrentLevelID
	}
	if req.Rating != nil {
		e.Rating = *req.Rating
	}

	if err := s.userStatsRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("user stats updated", zap.Int("user_stats_id", e.ID), zap.Int("user_id", e.UserID))
	return mapper.ToUserStatsResponse(e), nil
}

func (s *userStatsService) Delete(ctx context.Context, id int) error {
	exists, err := s.userStatsRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, constant.MsgUserStatsNotFound, nil)
	}
	if err := s.userStatsRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("user stats deleted", zap.Int("user_stats_id", id))
	return nil
}
