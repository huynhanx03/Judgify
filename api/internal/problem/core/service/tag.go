package service

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/mapper"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)


type tagService struct {
	tagRepo ports.TagRepository
}

// NewTagService creates a new TagService instance.
func NewTagService(tagRepo ports.TagRepository) ports.TagService {
	return &tagService{tagRepo: tagRepo}
}

// Find retrieves tags with pagination.
func (s *tagService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.TagResponse], error) {
	tags, err := s.tagRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if tags.Records == nil {
		return &d.Paginated[*dto.TagResponse]{
			Records:    &[]*dto.TagResponse{},
			Pagination: tags.Pagination,
		}, nil
	}

	entities := *tags.Records
	responses := make([]*dto.TagResponse, len(entities))
	for i, t := range entities {
		responses[i] = mapper.ToTagResponse(t)
	}

	return &d.Paginated[*dto.TagResponse]{
		Records:    &responses,
		Pagination: tags.Pagination,
	}, nil
}

// Get retrieves a tag by ID.
func (s *tagService) Get(ctx context.Context, id int) (*dto.TagResponse, error) {
	tag, err := s.tagRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToTagResponse(tag), nil
}

// Create creates a new tag.
func (s *tagService) Create(ctx context.Context, req *dto.CreateTagRequest) (*dto.TagResponse, error) {
	tag := mapper.ToTagEntityFromCreate(req)
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("tag created", zap.Int("tag_id", tag.ID))
	return mapper.ToTagResponse(tag), nil
}

// Update updates an existing tag.
func (s *tagService) Update(ctx context.Context, id int, req *dto.UpdateTagRequest) (*dto.TagResponse, error) {
	tag, err := s.tagRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		tag.Name = *req.Name
	}

	tag.ID = id
	if err := s.tagRepo.Update(ctx, tag); err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("tag updated", zap.Int("tag_id", tag.ID))
	return mapper.ToTagResponse(tag), nil
}

// Delete removes a tag by ID.
func (s *tagService) Delete(ctx context.Context, id int) error {
	exists, err := s.tagRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, apperr.MsgNotFound, nil)
	}

	if err := s.tagRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("tag deleted", zap.Int("tag_id", id))
	return nil
}
