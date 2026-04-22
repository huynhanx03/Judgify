package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/core/mapper"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)


type attributeDefinitionService struct {
	attrDefRepo ports.AttributeDefinitionRepository
	cache       cache.LocalCache[string, any]
}

// NewAttributeDefinitionService creates a new AttributeDefinitionService instance.
func NewAttributeDefinitionService(
	attrDefRepo ports.AttributeDefinitionRepository,
	cache cache.LocalCache[string, any],
) ports.AttributeDefinitionService {
	return &attributeDefinitionService{
		attrDefRepo: attrDefRepo,
		cache:       cache,
	}
}

// Find retrieves attribute definitions with pagination.
func (s *attributeDefinitionService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.AttributeDefinitionResponse], error) {
	result, err := s.attrDefRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	if result.Records == nil {
		return &d.Paginated[*dto.AttributeDefinitionResponse]{
			Records:    &[]*dto.AttributeDefinitionResponse{},
			Pagination: result.Pagination,
		}, nil
	}

	entities := *result.Records
	responses := make([]*dto.AttributeDefinitionResponse, len(entities))
	for i, e := range entities {
		responses[i] = mapper.ToAttributeDefinitionResponse(e)
	}

	return &d.Paginated[*dto.AttributeDefinitionResponse]{
		Records:    &responses,
		Pagination: result.Pagination,
	}, nil
}

// Get retrieves an attribute definition by ID.
func (s *attributeDefinitionService) Get(ctx context.Context, id int) (*dto.AttributeDefinitionResponse, error) {
	cacheKey := constant.CacheKeyPrefixAttrID + strconv.Itoa(id)
	if d, found := cache.Get[*entity.AttributeDefinition](s.cache, cacheKey); found {
		return mapper.ToAttributeDefinitionResponse(d), nil
	}

	attrDef, err := s.attrDefRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	cache.Set(s.cache, cacheKey, attrDef)
	return mapper.ToAttributeDefinitionResponse(attrDef), nil
}

// Create creates a new attribute definition.
func (s *attributeDefinitionService) Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	attrDef := mapper.ToAttributeDefinitionEntityFromCreate(req)
	if err := s.attrDefRepo.Create(ctx, attrDef); err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("attribute definition created", zap.Int("attribute_definition_id", attrDef.ID))
	return mapper.ToAttributeDefinitionResponse(attrDef), nil
}

// Update updates an existing attribute definition.
func (s *attributeDefinitionService) Update(ctx context.Context, id int, req *dto.UpdateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	attrDef, err := s.attrDefRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Key != nil {
		attrDef.Key = *req.Key
	}
	if req.DataType != nil {
		attrDef.DataType = *req.DataType
	}
	if req.Description != nil {
		attrDef.Description = *req.Description
	}

	attrDef.ID = id
	if err := s.attrDefRepo.Update(ctx, attrDef); err != nil {
		return nil, err
	}

	// Invalidate Cache
	cacheKeyID := constant.CacheKeyPrefixAttrID + strconv.Itoa(id)
	cacheKeyKey := constant.CacheKeyPrefixAttrKey + attrDef.Key
	cache.Set(s.cache, cacheKeyID, attrDef)
	cache.Del(s.cache, cacheKeyKey)

	logger.FromContext(ctx).Info("attribute definition updated", zap.Int("attribute_definition_id", attrDef.ID))
	return mapper.ToAttributeDefinitionResponse(attrDef), nil
}

// Delete removes an attribute definition by ID.
func (s *attributeDefinitionService) Delete(ctx context.Context, id int) error {
	exists, err := s.attrDefRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjAttributeDefinition), nil)
	}

	if err := s.attrDefRepo.Delete(ctx, id); err != nil {
		return err
	}

	cacheKeyID := constant.CacheKeyPrefixAttrID + strconv.Itoa(id)
	cache.Del(s.cache, cacheKeyID)

	logger.FromContext(ctx).Info("attribute definition deleted", zap.Int("attribute_definition_id", id))
	return nil
}
