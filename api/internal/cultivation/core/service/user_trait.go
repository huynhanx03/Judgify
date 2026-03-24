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


type userTraitService struct {
	userTraitRepo ports.UserTraitRepository
}

// NewUserTraitService creates a new UserTraitService instance.
func NewUserTraitService(userTraitRepo ports.UserTraitRepository) ports.UserTraitService {
	return &userTraitService{userTraitRepo: userTraitRepo}
}

func (s *userTraitService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserTraitResponse], error) {
	result, err := s.userTraitRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.UserTraitResponse]{Records: &[]*dto.UserTraitResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.UserTraitResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToUserTraitResponse(e)
	}

	return &d.Paginated[*dto.UserTraitResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *userTraitService) Get(ctx context.Context, id int) (*dto.UserTraitResponse, error) {
	e, err := s.userTraitRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToUserTraitResponse(e), nil
}

func (s *userTraitService) Create(ctx context.Context, req *dto.CreateUserTraitRequest) (*dto.UserTraitResponse, error) {
	e := mapper.ToUserTraitEntityFromCreate(req)
	if err := s.userTraitRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("user assigned new trait", zap.Int("user_trait_id", e.ID), zap.Int("user_id", e.UserID), zap.Int("trait_id", e.TraitID))
	return mapper.ToUserTraitResponse(e), nil
}

func (s *userTraitService) Delete(ctx context.Context, id int) error {
	exists, err := s.userTraitRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, constant.MsgUserTraitNotFound, nil)
	}
	if err := s.userTraitRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("user trait deleted", zap.Int("user_trait_id", id))
	return nil
}
