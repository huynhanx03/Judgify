package service

import (
	"context"
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/constant"
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/mapper"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

const userTraitServiceName = "UserTraitService"

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
	return mapper.ToUserTraitResponse(e), nil
}

func (s *userTraitService) Delete(ctx context.Context, id int) error {
	exists, err := s.userTraitRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.NewError(userTraitServiceName, response.CodeNotFound, constant.MsgUserTraitNotFound, http.StatusNotFound, nil)
	}
	return s.userTraitRepo.Delete(ctx, id)
}
