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


type userElementExpService struct {
	userElementExpRepo ports.UserElementExpRepository
}

// NewUserElementExpService creates a new UserElementExpService instance.
func NewUserElementExpService(repo ports.UserElementExpRepository) ports.UserElementExpService {
	return &userElementExpService{userElementExpRepo: repo}
}

func (s *userElementExpService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserElementExpResponse], error) {
	result, err := s.userElementExpRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.UserElementExpResponse]{Records: &[]*dto.UserElementExpResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.UserElementExpResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToUserElementExpResponse(e)
	}

	return &d.Paginated[*dto.UserElementExpResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *userElementExpService) Get(ctx context.Context, id int) (*dto.UserElementExpResponse, error) {
	e, err := s.userElementExpRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToUserElementExpResponse(e), nil
}

func (s *userElementExpService) Create(ctx context.Context, req *dto.CreateUserElementExpRequest) (*dto.UserElementExpResponse, error) {
	e := mapper.ToUserElementExpEntityFromCreate(req)
	if err := s.userElementExpRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("user element exp created", zap.Int("user_element_exp_id", e.ID), zap.Int("user_id", e.UserID), zap.Int("element_id", e.ElementID))
	return mapper.ToUserElementExpResponse(e), nil
}

func (s *userElementExpService) Update(ctx context.Context, id int, req *dto.UpdateUserElementExpRequest) (*dto.UserElementExpResponse, error) {
	e, err := s.userElementExpRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Exp != nil {
		e.Exp = *req.Exp
	}

	if err := s.userElementExpRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("user element exp updated", zap.Int("user_element_exp_id", e.ID), zap.Int("user_id", e.UserID))
	return mapper.ToUserElementExpResponse(e), nil
}

func (s *userElementExpService) Delete(ctx context.Context, id int) error {
	exists, err := s.userElementExpRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, constant.MsgUserElementExpNotFound, nil)
	}
	if err := s.userElementExpRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("user element exp deleted", zap.Int("user_element_exp_id", id))
	return nil
}
