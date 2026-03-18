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

const userElementExpServiceName = "UserElementExpService"

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
	return mapper.ToUserElementExpResponse(e), nil
}

func (s *userElementExpService) Delete(ctx context.Context, id int) error {
	exists, err := s.userElementExpRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.NewError(userElementExpServiceName, response.CodeNotFound, constant.MsgUserElementExpNotFound, http.StatusNotFound, nil)
	}
	return s.userElementExpRepo.Delete(ctx, id)
}
