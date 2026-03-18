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

const traitServiceName = "TraitService"

type traitService struct {
	traitRepo ports.TraitRepository
}

// NewTraitService creates a new TraitService instance.
func NewTraitService(traitRepo ports.TraitRepository) ports.TraitService {
	return &traitService{traitRepo: traitRepo}
}

func (s *traitService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.TraitResponse], error) {
	result, err := s.traitRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.TraitResponse]{Records: &[]*dto.TraitResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.TraitResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToTraitResponse(e)
	}

	return &d.Paginated[*dto.TraitResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *traitService) Get(ctx context.Context, id int) (*dto.TraitResponse, error) {
	e, err := s.traitRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToTraitResponse(e), nil
}

func (s *traitService) Create(ctx context.Context, req *dto.CreateTraitRequest) (*dto.TraitResponse, error) {
	e := mapper.ToTraitEntityFromCreate(req)
	if err := s.traitRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	return mapper.ToTraitResponse(e), nil
}

func (s *traitService) Update(ctx context.Context, id int, req *dto.UpdateTraitRequest) (*dto.TraitResponse, error) {
	e, err := s.traitRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Type != nil {
		e.Type = *req.Type
	}
	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.Rarity != nil {
		e.Rarity = *req.Rarity
	}
	if req.Weight != nil {
		e.Weight = *req.Weight
	}
	if req.Description != nil {
		e.Description = *req.Description
	}
	if req.Metadata != nil {
		e.Metadata = *req.Metadata
	}

	if err := s.traitRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	return mapper.ToTraitResponse(e), nil
}

func (s *traitService) Delete(ctx context.Context, id int) error {
	exists, err := s.traitRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.NewError(traitServiceName, response.CodeNotFound, constant.MsgTraitNotFound, http.StatusNotFound, nil)
	}
	return s.traitRepo.Delete(ctx, id)
}
