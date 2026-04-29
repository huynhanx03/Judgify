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

type rarityService struct {
	rarityRepo   ports.RarityRepository
	gachaService ports.GachaService
}

// NewRarityService creates a new RarityService instance.
func NewRarityService(rarityRepo ports.RarityRepository, gachaService ports.GachaService) ports.RarityService {
	return &rarityService{rarityRepo: rarityRepo, gachaService: gachaService}
}

func (s *rarityService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RarityResponse], error) {
	result, err := s.rarityRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.RarityResponse]{Records: &[]*dto.RarityResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*dto.RarityResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToRarityResponse(e)
	}

	return &d.Paginated[*dto.RarityResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *rarityService) Get(ctx context.Context, id int) (*dto.RarityResponse, error) {
	e, err := s.rarityRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToRarityResponse(e), nil
}

func (s *rarityService) Create(ctx context.Context, req *dto.CreateRarityRequest) (*dto.RarityResponse, error) {
	e := mapper.ToRarityEntityFromCreate(req)
	if err := s.rarityRepo.Create(ctx, e); err != nil {
		return nil, err
	}
	s.invalidateGachaPool(ctx)
	logger.FromContext(ctx).Info("rarity created", zap.Int("rarity_id", e.ID))
	return mapper.ToRarityResponse(e), nil
}

func (s *rarityService) Update(ctx context.Context, id int, req *dto.UpdateRarityRequest) (*dto.RarityResponse, error) {
	e, err := s.rarityRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		e.Name = *req.Name
	}
	if req.Code != nil {
		e.Code = *req.Code
	}
	if req.Weight != nil {
		e.Weight = *req.Weight
	}
	if req.Description != nil {
		e.Description = *req.Description
	}
	if err := s.rarityRepo.Update(ctx, e); err != nil {
		return nil, err
	}
	s.invalidateGachaPool(ctx)
	logger.FromContext(ctx).Info("rarity updated", zap.Int("rarity_id", e.ID))
	return mapper.ToRarityResponse(e), nil
}

func (s *rarityService) Delete(ctx context.Context, id int) error {
	exists, err := s.rarityRepo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjRarity), nil)
	}
	if err := s.rarityRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateGachaPool(ctx)
	logger.FromContext(ctx).Info("rarity deleted", zap.Int("rarity_id", id))
	return nil
}

// FindAll retrieves all rarities without pagination.
func (s *rarityService) FindAll(ctx context.Context) ([]*dto.RarityResponse, error) {
	rarities, err := s.rarityRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.RarityResponse, len(rarities))
	for i, e := range rarities {
		responses[i] = mapper.ToRarityResponse(e)
	}
	return responses, nil
}

func (s *rarityService) invalidateGachaPool(ctx context.Context) {
	if s.gachaService == nil {
		return
	}
	if err := s.gachaService.InvalidatePool(ctx); err != nil {
		logger.FromContext(ctx).Warn("failed to invalidate gacha pool after rarity change", zap.Error(err))
	}
}
