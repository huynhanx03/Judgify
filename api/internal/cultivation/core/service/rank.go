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

const rankServiceName = "RankService"

type rankService struct {
	rankRepo ports.RankRepository
}

// NewRankService creates a new RankService instance.
func NewRankService(rankRepo ports.RankRepository) ports.RankService {
	return &rankService{rankRepo: rankRepo}
}

func (s *rankService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RankResponse], error) {
	result, err := s.rankRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.RankResponse]{Records: &[]*dto.RankResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.RankResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToRankResponse(e)
	}

	return &d.Paginated[*dto.RankResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *rankService) Get(ctx context.Context, id int) (*dto.RankResponse, error) {
	e, err := s.rankRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToRankResponse(e), nil
}

func (s *rankService) Create(ctx context.Context, req *dto.CreateRankRequest) (*dto.RankResponse, error) {
	e := mapper.ToRankEntityFromCreate(req)
	if err := s.rankRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	return mapper.ToRankResponse(e), nil
}

func (s *rankService) Update(ctx context.Context, id int, req *dto.UpdateRankRequest) (*dto.RankResponse, error) {
	e, err := s.rankRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.Order != nil {
		e.Order = *req.Order
	}
	if req.MinRating != nil {
		e.MinRating = *req.MinRating
	}
	if req.Description != nil {
		e.Description = *req.Description
	}

	if err := s.rankRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	return mapper.ToRankResponse(e), nil
}

func (s *rankService) Delete(ctx context.Context, id int) error {
	exists, err := s.rankRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.NewError(rankServiceName, response.CodeNotFound, constant.MsgRankNotFound, http.StatusNotFound, nil)
	}
	return s.rankRepo.Delete(ctx, id)
}
