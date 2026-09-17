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
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type tagService struct {
	tagRepo ports.TagRepository
	cache   ports.TagCache
}

// NewTagService creates a new TagService instance.
func NewTagService(tagRepo ports.TagRepository, cache ports.TagCache) ports.TagService {
	return &tagService{
		tagRepo: tagRepo,
		cache:   cache,
	}
}

// Find retrieves tags with pagination.
func (s *tagService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*contract.TagResult], error) {
	tags, err := s.tagRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if tags.Records == nil {
		return &d.Paginated[*contract.TagResult]{
			Records:    &[]*contract.TagResult{},
			Pagination: tags.Pagination,
		}, nil
	}

	entities := *tags.Records
	responses := make([]*contract.TagResult, len(entities))
	for i, t := range entities {
		responses[i] = ToTagResult(t)
	}

	return &d.Paginated[*contract.TagResult]{
		Records:    &responses,
		Pagination: tags.Pagination,
	}, nil
}

// Get retrieves a tag by ID.
func (s *tagService) Get(ctx context.Context, id uuid.UUID) (*contract.TagResult, error) {
	return s.cache.Get(ctx, id, func(ctx context.Context) (*contract.TagResult, error) {
		logger.FromContext(ctx).Debug("cache miss, loading tag from db", zap.String("tag_id", id.String()))
		tag, err := s.tagRepo.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return ToTagResult(tag), nil
	})
}

// Create creates a new tag.
func (s *tagService) Create(ctx context.Context, req *contract.CreateTagCommand) (*contract.TagResult, error) {
	tag := ToTagEntityFromCreate(req)
	// Set elements from request IDs (only IDs needed for DB insert)
	tag.Elements = make([]entity.TagElement, len(req.ElementIDs))
	for i, id := range req.ElementIDs {
		tag.Elements[i] = entity.TagElement{ID: id}
	}
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		if errors.Is(err, ports.ErrTagVersionConflict) {
			return nil, tagVersionConflict(err)
		}
		return nil, err
	}
	if err := s.cache.Invalidate(ctx, tag.ID); err != nil {
		return nil, err
	}
	// Re-fetch to get full element data (name, code)
	created, err := s.tagRepo.Get(ctx, tag.ID)
	if err != nil {
		logger.FromContext(ctx).Warn("tag created but re-fetch failed", zap.String("tag_id", tag.ID.String()), zap.Error(err))
		return nil, err
	}
	logger.FromContext(ctx).Info("tag created", zap.String("tag_id", tag.ID.String()))
	return ToTagResult(created), nil
}

// Update updates an existing tag.
func (s *tagService) Update(ctx context.Context, id uuid.UUID, req *contract.UpdateTagCommand) (*contract.TagResult, error) {
	tag, err := s.tagRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		tag.Name = *req.Name
	}

	if req.ElementIDs != nil {
		elements := make([]entity.TagElement, len(*req.ElementIDs))
		for i, eid := range *req.ElementIDs {
			elements[i] = entity.TagElement{ID: eid}
		}
		tag.Elements = elements
	}

	tag.ID = id
	if err := s.tagRepo.Update(ctx, tag); err != nil {
		if errors.Is(err, ports.ErrTagVersionConflict) {
			return nil, tagVersionConflict(err)
		}
		return nil, err
	}
	// Schedule/invalidate before presentation re-fetch. A re-fetch failure must
	// never leave the committed tag list cached as stale.
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return nil, err
	}

	// Re-fetch to get full element data
	updated, err := s.tagRepo.Get(ctx, id)
	if err != nil {
		logger.FromContext(ctx).Warn("tag updated but re-fetch failed", zap.String("tag_id", tag.ID.String()), zap.Error(err))
		return nil, err
	}
	logger.FromContext(ctx).Info("tag updated", zap.String("tag_id", tag.ID.String()))
	return ToTagResult(updated), nil
}

func tagVersionConflict(cause error) error {
	return apperr.New(apperr.CodeConflict, "tag changed; reload and try again", cause).
		WithDetails(map[string]any{"reason": "version_conflict"})
}

// Delete removes a tag by ID.
func (s *tagService) Delete(ctx context.Context, id uuid.UUID) error {
	exists, err := s.tagRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.For(constant.ProblemObjTag).NotFound(nil)
	}

	if err := s.tagRepo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.cache.Invalidate(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("tag deleted", zap.String("tag_id", id.String()))
	return nil
}

// FindAll retrieves all tags without pagination.
func (s *tagService) FindAll(ctx context.Context) ([]*contract.TagResult, error) {
	return s.cache.FindAll(ctx, func(ctx context.Context) ([]*contract.TagResult, error) {
		logger.FromContext(ctx).Debug("cache miss, loading tags from db")
		tags, err := s.tagRepo.FindAll(ctx)
		if err != nil {
			return nil, err
		}
		responses := make([]*contract.TagResult, len(tags))
		for i, t := range tags {
			responses[i] = ToTagResult(t)
		}
		return responses, nil
	})
}
