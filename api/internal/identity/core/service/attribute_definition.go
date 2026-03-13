package service

import (
	"context"
	"net/http"
	"strconv"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/core/mapper"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

const attrDefServiceName = "AttributeDefinitionService"

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
	if d, found := cache.GetLocal[*entity.AttributeDefinition](s.cache, cacheKey); found {
		return mapper.ToAttributeDefinitionResponse(d), nil
	}

	attrDef, err := s.attrDefRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	cache.SetLocal(s.cache, cacheKey, attrDef, constant.CacheCostID)
	return mapper.ToAttributeDefinitionResponse(attrDef), nil
}

// Create creates a new attribute definition.
func (s *attributeDefinitionService) Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	attrDef := mapper.ToAttributeDefinitionEntityFromCreate(req)
	if err := s.attrDefRepo.Create(ctx, attrDef); err != nil {
		return nil, err
	}

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
	cache.SetLocal(s.cache, cacheKeyID, attrDef, constant.CacheCostID)
	cache.DeleteLocal(s.cache, cacheKeyKey)

	return mapper.ToAttributeDefinitionResponse(attrDef), nil
}

// Delete removes an attribute definition by ID.
func (s *attributeDefinitionService) Delete(ctx context.Context, id int) error {
	exists, err := s.attrDefRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.NewError(attrDefServiceName, response.CodeNotFound, apperr.MsgNotFound, http.StatusNotFound, nil)
	}

	if err := s.attrDefRepo.Delete(ctx, id); err != nil {
		return err
	}

	cacheKeyID := constant.CacheKeyPrefixAttrID + strconv.Itoa(id)
	cache.DeleteLocal(s.cache, cacheKeyID)

	return nil
}
