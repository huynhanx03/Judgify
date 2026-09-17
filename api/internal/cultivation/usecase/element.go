package usecase

import (
	"context"
	"errors"

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

type elementService struct {
	elementRepo ports.ElementRepository
	cache       ports.ElementCache
	journal     platformmutation.Executor
}

// NewElementService creates a new ElementService instance.
func NewElementService(elementRepo ports.ElementRepository, cache ports.ElementCache, journals ...platformmutation.Executor) ports.ElementService {
	var journal platformmutation.Executor
	if len(journals) > 0 {
		journal = journals[0]
	}
	return &elementService{elementRepo: elementRepo, cache: cache, journal: journal}
}

func (s *elementService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.ElementResponse], error) {
	result, err := s.elementRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*contract.ElementResponse]{Records: &[]*contract.ElementResponse{}, Pagination: result.Pagination}, nil
	}

	entities := *result.Records
	responses := make([]*contract.ElementResponse, len(entities))
	for i, e := range entities {
		responses[i] = ToElementResponse(e)
	}

	return &d.Paginated[*contract.ElementResponse]{Records: &responses, Pagination: result.Pagination}, nil
}

func (s *elementService) Get(ctx context.Context, id uuid.UUID) (*contract.ElementResponse, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.ElementResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading element from db", zap.String("element_id", id.String()))
		e, err := s.elementRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToElementResponse(e), nil
	})
}

func (s *elementService) Create(ctx context.Context, req *contract.CreateElementCommand) (*contract.ElementResponse, error) {
	e := ToElementEntityFromCreate(req)
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		if err := s.elementRepo.Create(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.element.created.v1", "cultivation_element", e.ID,
			e.Version, map[string]any{}, elementDocument(e)), nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrElementVersionConflict) {
			return nil, elementVersionConflict(err)
		}
		return nil, err
	}
	if err := s.invalidateAfterCommit(ctx, e.ID); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("element created", zap.String("element_id", e.ID.String()))
	return ToElementResponse(e), nil
}

func (s *elementService) Update(ctx context.Context, id uuid.UUID, req *contract.UpdateElementCommand) (*contract.ElementResponse, error) {
	var e *entity.Element
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var err error
		e, err = s.elementRepo.Get(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		before := elementDocument(e)
		if req.Name != nil {
			e.Name = *req.Name
		}
		if req.Code != nil {
			e.Code = *req.Code
		}
		if req.Description != nil {
			e.Description = *req.Description
		}
		if err := s.elementRepo.Update(txCtx, e); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.element.updated.v1", "cultivation_element", e.ID,
			e.Version, before, elementDocument(e)), nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrElementVersionConflict) {
			return nil, elementVersionConflict(err)
		}
		return nil, err
	}
	if err := s.invalidateAfterCommit(ctx, id); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("element updated", zap.String("element_id", e.ID.String()))
	return ToElementResponse(e), nil
}

func (s *elementService) invalidateAfterCommit(ctx context.Context, id uuid.UUID) error {
	return s.cache.Invalidate(ctx, id)
}

func elementVersionConflict(cause error) error {
	return apperr.New(apperr.CodeConflict, "element changed; reload and try again", cause).
		WithDetails(map[string]any{"reason": "version_conflict"})
}

func (s *elementService) Delete(ctx context.Context, id uuid.UUID) error {
	err := executeCultivationMutation(s.journal, ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		exists, err := s.elementRepo.Exists(txCtx, id)
		if err != nil {
			return platformmutation.Command{}, err
		}
		if !exists {
			return platformmutation.Command{}, apperr.For(constant.CultivationObjElement).NotFound(nil)
		}
		before := map[string]any{}
		version := int64(0)
		if s.journal != nil {
			stored, err := s.elementRepo.Get(txCtx, id)
			if err != nil {
				return platformmutation.Command{}, err
			}
			before = elementDocument(stored)
			version = stored.Version
		}
		if err := s.elementRepo.Delete(txCtx, id); err != nil {
			return platformmutation.Command{}, err
		}
		return cultivationCommand("cultivation.element.deleted.v1", "cultivation_element", id,
			version, before, map[string]any{}), nil
	})
	if err != nil {
		return err
	}
	if err := s.invalidateAfterCommit(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("element deleted", zap.String("element_id", id.String()))
	return nil
}

// FindAll retrieves all elements without pagination.
func (s *elementService) FindAll(ctx context.Context) ([]*contract.ElementResponse, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.ElementResponse, error) {
		logger.FromContext(ctx).Debug("cache miss, loading elements from db")
		elements, err := s.elementRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*contract.ElementResponse, len(elements))
		for i, e := range elements {
			responses[i] = ToElementResponse(e)
		}
		return responses, nil
	})
}
