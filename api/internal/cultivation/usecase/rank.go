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

type rankService struct {
	rankRepo ports.RankRepository
	cache    ports.RankCache
	journal  platformmutation.Executor
}

// NewRankService creates a new RankService instance.
func NewRankService(rankRepo ports.RankRepository, cache ports.RankCache, journals ...platformmutation.Executor) ports.RankService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &rankService{rankRepo: rankRepo, cache: cache, journal: journal}
}

func (s *rankService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.RankResponse], error) {
	result, err := s.rankRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*contract.RankResponse]{Records: &[]*contract.RankResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*contract.RankResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToRankResponse(e)
	}

	return &d.Paginated[*contract.RankResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *rankService) Get(ctx context.Context, id uuid.UUID) (*contract.RankResponse, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.RankResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading rank from db", zap.String("rank_id", id.String()))
		e, err := s.rankRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToRankResponse(e), nil
	})
}

func (s *rankService) Create(ctx context.Context, command *contract.CreateRankCommand) (*contract.RankResponse, error) {
	e := ToRankEntityFromCreate(command)
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.rankRepo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.rank.created.v1", "cultivation_rank", e.ID, 0,
			map[string]any{}, rankDocument(e)), nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, e.ID); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("rank created", zap.String("rank_id", e.ID.String()))
	return ToRankResponse(e), nil
}

func (s *rankService) Update(ctx context.Context, id uuid.UUID, command *contract.UpdateRankCommand) (*contract.RankResponse, error) {
	var e *entity.Rank
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var err error
		e, err = s.rankRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		before := rankDocument(e)
		if command.Name != nil {
			e.Name = *command.Name
		}
		if command.MinRating != nil {
			e.MinRating = *command.MinRating
		}
		if command.Description != nil {
			e.Description = *command.Description
		}
		if err := s.rankRepo.Update(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.rank.updated.v1", "cultivation_rank", id, 0,
			before, rankDocument(e)), nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("rank updated", zap.String("rank_id", e.ID.String()))
	return ToRankResponse(e), nil
}

func (s *rankService) Delete(ctx context.Context, id uuid.UUID) error {
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		exists, err := s.rankRepo.Exists(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if !exists {
			return platformmutation.Command{}, apperr.For(constant.CultivationObjRank).NotFound(nil)
		}
		before := map[string]any{}
		if s.journal != nil {
			stored, err := s.rankRepo.Get(txCtx, id)
			if err != nil {
				return platformmutation.Command{}, err
			}
			before = rankDocument(stored)
		}
		if err := s.rankRepo.Delete(txCtx, id); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.rank.deleted.v1", "cultivation_rank", id, 0,
			before, map[string]any{}), nil
	})
	if err != nil {
		return err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("rank deleted", zap.String("rank_id", id.String()))
	return nil
}

// FindAll retrieves all ranks without pagination.
func (s *rankService) FindAll(ctx context.Context) ([]*contract.RankResponse, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.RankResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading ranks from db")
		ranks, err := s.rankRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*contract.RankResponse, len(ranks))
		for i, e := range ranks {
			responses[i] = ToRankResponse(e)
		}
		return responses, nil
	})
}
