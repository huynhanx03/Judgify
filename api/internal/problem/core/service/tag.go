package service

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
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
	// Set elements from request IDs (only IDs needed for DB insert)
	tag.Elements = make([]entity.TagElement, len(req.ElementIDs))
	for i, id := range req.ElementIDs {
		tag.Elements[i] = entity.TagElement{ID: id}
	}
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, err
	}
	// Re-fetch to get full element data (name, code)
	created, err := s.tagRepo.Get(ctx, tag.ID)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("tag created", zap.Int("tag_id", tag.ID))
	return mapper.ToTagResponse(created), nil
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

	if req.ElementIDs != nil {
		elements := make([]entity.TagElement, len(*req.ElementIDs))
		for i, eid := range *req.ElementIDs {
			elements[i] = entity.TagElement{ID: eid}
		}
		tag.Elements = elements
	}

	tag.ID = id
	if err := s.tagRepo.Update(ctx, tag); err != nil {
		return nil, err
	}

	// Re-fetch to get full element data
	updated, err := s.tagRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info("tag updated", zap.Int("tag_id", tag.ID))
	return mapper.ToTagResponse(updated), nil
}

// Delete removes a tag by ID.
func (s *tagService) Delete(ctx context.Context, id int) error {
	exists, err := s.tagRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjTag), nil)
	}

	if err := s.tagRepo.Delete(ctx, id); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("tag deleted", zap.Int("tag_id", id))
	return nil
}

// FindAll retrieves all tags without pagination.
func (s *tagService) FindAll(ctx context.Context) ([]*dto.TagResponse, error) {
	tags, err := s.tagRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.TagResponse, len(tags))
	for i, t := range tags {
		responses[i] = mapper.ToTagResponse(t)
	}
	return responses, nil
}
