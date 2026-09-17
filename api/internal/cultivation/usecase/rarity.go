package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

type rarityService struct {
	rarityRepo ports.RarityRepository
	catalog    ports.TraitCatalogInvalidator
	cache      ports.RarityCache
	journal    platformmutation.Executor
}

// NewRarityService creates a new RarityService instance.
func NewRarityService(rarityRepo ports.RarityRepository, catalog ports.TraitCatalogInvalidator, cache ports.RarityCache, journals ...platformmutation.Executor) ports.RarityService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &rarityService{rarityRepo: rarityRepo, catalog: catalog, cache: cache, journal: journal}
}

func (s *rarityService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.RarityResponse], error) {
	result, err := s.rarityRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*contract.RarityResponse]{Records: &[]*contract.RarityResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*contract.RarityResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToRarityResponse(e)
	}

	return &d.Paginated[*contract.RarityResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *rarityService) Get(ctx context.Context, id uuid.UUID) (*contract.RarityResponse, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.RarityResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading rarity from db", zap.String("rarity_id", id.String()))
		e, err := s.rarityRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToRarityResponse(e), nil
	})
}

func (s *rarityService) Create(ctx context.Context, command *contract.CreateRarityCommand) (*contract.RarityResponse, error) {
	e := ToRarityEntityFromCreate(command)
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.rarityRepo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		if err := s.refreshCatalog(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.rarity.created.v1", "cultivation_rarity", e.ID, 0,
			map[string]any{}, rarityDocument(e)), nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, e.ID); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("rarity created", zap.String("rarity_id", e.ID.String()))
	return ToRarityResponse(e), nil
}

func (s *rarityService) Update(ctx context.Context, id uuid.UUID, command *contract.UpdateRarityCommand) (*contract.RarityResponse, error) {
	var e *entity.Rarity
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var err error
		e, err = s.rarityRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		before := rarityDocument(e)
		if command.Name != nil {
			e.Name = *command.Name
		}
		if command.Code != nil {
			e.Code = *command.Code
		}
		if command.Weight != nil {
			e.Weight = *command.Weight
		}
		if command.Description != nil {
			e.Description = *command.Description
		}
		if err := s.rarityRepo.Update(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		if err := s.refreshCatalog(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.rarity.updated.v1", "cultivation_rarity", id, 0,
			before, rarityDocument(e)), nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("rarity updated", zap.String("rarity_id", e.ID.String()))
	return ToRarityResponse(e), nil
}

func (s *rarityService) Delete(ctx context.Context, id uuid.UUID) error {
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		exists, err := s.rarityRepo.Exists(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if !exists {
			return platformmutation.Command{}, apperr.For(constant.CultivationObjRarity).NotFound(nil)
		}
		before := map[string]any{}
		if s.journal != nil {
			stored, err := s.rarityRepo.Get(txCtx, id)
			if err != nil {
				return platformmutation.Command{}, err
			}
			before = rarityDocument(stored)
		}
		if err := s.rarityRepo.Delete(txCtx, id); err != nil {
			return platformmutation.Command{}, err
		}
		if err := s.refreshCatalog(txCtx); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.rarity.deleted.v1", "cultivation_rarity", id, 0,
			before, map[string]any{}), nil
	})
	if err != nil {
		return err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("rarity deleted", zap.String("rarity_id", id.String()))
	return nil
}

// FindAll retrieves all rarities without pagination.
func (s *rarityService) FindAll(ctx context.Context) ([]*contract.RarityResponse, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.RarityResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading rarities from db")
		rarities, err := s.rarityRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*contract.RarityResponse, len(rarities))
		for i, e := range rarities {
			responses[i] = ToRarityResponse(e)
		}
		return responses, nil
	})
}

func (s *rarityService) refreshCatalog(ctx context.Context) error {
	if s.catalog == nil {
		return entity.ErrTraitOfferUnavailable
	}
	return s.catalog.InvalidatePool(ctx)
}
