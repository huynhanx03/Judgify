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

type levelService struct {
	levelRepo ports.LevelRepository
	cache     ports.LevelCache
	journal   platformmutation.Executor
}

// NewLevelService creates a new LevelService instance.
func NewLevelService(levelRepo ports.LevelRepository, cache ports.LevelCache, journals ...platformmutation.Executor) ports.LevelService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &levelService{levelRepo: levelRepo, cache: cache, journal: journal}
}

func (s *levelService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.LevelResponse], error) {
	result, err := s.levelRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*contract.LevelResponse]{Records: &[]*contract.LevelResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*contract.LevelResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToLevelResponse(e)
	}

	return &d.Paginated[*contract.LevelResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *levelService) Get(ctx context.Context, id uuid.UUID) (*contract.LevelResponse, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.LevelResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading level from db", zap.String("level_id", id.String()))
		e, err := s.levelRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToLevelResponse(e), nil
	})
}

func (s *levelService) Create(ctx context.Context, command *contract.CreateLevelCommand) (*contract.LevelResponse, error) {
	e := ToLevelEntityFromCreate(command)
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.levelRepo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.level.created.v1", "cultivation_level", e.ID, 0,
			map[string]any{}, levelDocument(e)), nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, e.ID); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("level created", zap.String("level_id", e.ID.String()))
	return ToLevelResponse(e), nil
}

func (s *levelService) Update(ctx context.Context, id uuid.UUID, command *contract.UpdateLevelCommand) (*contract.LevelResponse, error) {
	var e *entity.Level
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var err error
		e, err = s.levelRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		before := levelDocument(e)
		if command.Name != nil {
			e.Name = *command.Name
		}
		if command.MinExp != nil {
			e.MinExp = *command.MinExp
		}
		if command.Description != nil {
			e.Description = *command.Description
		}
		if err := s.levelRepo.Update(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.level.updated.v1", "cultivation_level", id, 0,
			before, levelDocument(e)), nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("level updated", zap.String("level_id", e.ID.String()))
	return ToLevelResponse(e), nil
}

func (s *levelService) Delete(ctx context.Context, id uuid.UUID) error {
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		exists, err := s.levelRepo.Exists(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if !exists {
			return platformmutation.Command{}, apperr.For(constant.CultivationObjLevel).NotFound(nil)
		}
		before := map[string]any{}
		if s.journal != nil {
			stored, err := s.levelRepo.Get(txCtx, id)
			if err != nil {
				return platformmutation.Command{}, err
			}
			before = levelDocument(stored)
		}
		if err := s.levelRepo.Delete(txCtx, id); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.level.deleted.v1", "cultivation_level", id, 0,
			before, map[string]any{}), nil
	})
	if err != nil {
		return err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("level deleted", zap.String("level_id", id.String()))
	return nil
}

// FindAll retrieves all levels without pagination.
func (s *levelService) FindAll(ctx context.Context) ([]*contract.LevelResponse, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.LevelResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading levels from db")
		levels, err := s.levelRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*contract.LevelResponse, len(levels))
		for i, e := range levels {
			responses[i] = ToLevelResponse(e)
		}
		return responses, nil
	})
}
