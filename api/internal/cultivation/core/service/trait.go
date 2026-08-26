package service

import (
	"context"
	"fmt"

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

type traitService struct {
	traitRepo    ports.TraitRepository
	gachaService ports.GachaService
}

// NewTraitService creates a new TraitService instance.
func NewTraitService(traitRepo ports.TraitRepository, gachaService ports.GachaService) ports.TraitService {
	return &traitService{traitRepo: traitRepo, gachaService: gachaService}
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

func (s *traitService) FindAll(ctx context.Context) ([]*dto.TraitResponse, error) {
	entities, err := s.traitRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.TraitResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToTraitResponse(e)
	}
	return responses, nil
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
	s.invalidateGachaPool(ctx)
	logger.FromContext(ctx).Info("trait created", zap.Int("trait_id", e.ID))
	// Re-fetch to get full rarity data from eager loading.
	full, err := s.traitRepo.Get(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	return mapper.ToTraitResponse(full), nil
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
	if req.RarityID != nil {
		e.RarityID = *req.RarityID
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
	s.invalidateGachaPool(ctx)
	logger.FromContext(ctx).Info("trait updated", zap.Int("trait_id", e.ID))
	// Re-fetch to get full rarity data from eager loading.
	full, err := s.traitRepo.Get(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	return mapper.ToTraitResponse(full), nil
}

func (s *traitService) Delete(ctx context.Context, id int) error {
	exists, err := s.traitRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjTrait), nil)
	}
	if err := s.traitRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateGachaPool(ctx)
	logger.FromContext(ctx).Info("trait deleted", zap.Int("trait_id", id))
	return nil
}

func (s *traitService) invalidateGachaPool(ctx context.Context) {
	if s.gachaService == nil {
		return
	}
	if err := s.gachaService.InvalidatePool(ctx); err != nil {
		logger.FromContext(ctx).Warn("failed to invalidate gacha pool after trait change", zap.Error(err))
	}
}
