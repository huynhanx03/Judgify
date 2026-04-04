package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

// AttributeDefinitionHandler defines the attribute definition HTTP handler interface.
type AttributeDefinitionHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.AttributeDefinitionResponse], error)
	Get(ctx context.Context, req *dto.GetAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	Update(ctx context.Context, req *dto.UpdateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
	Delete(ctx context.Context, req *dto.DeleteAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error)
}

type attributeDefinitionHandler struct {
	handler.BaseHandler
	attrDefService ports.AttributeDefinitionService
}

// NewAttributeDefinitionHandler creates a new AttributeDefinitionHandler instance.
func NewAttributeDefinitionHandler(attrDefService ports.AttributeDefinitionService) AttributeDefinitionHandler {
	return &attributeDefinitionHandler{
		attrDefService: attrDefService,
	}
}

// Find retrieves attribute definitions with pagination.
func (h *attributeDefinitionHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.AttributeDefinitionResponse], error) {
	return h.attrDefService.Find(ctx, req)
}

// Get retrieves an attribute definition by ID.
func (h *attributeDefinitionHandler) Get(ctx context.Context, req *dto.GetAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	return h.attrDefService.Get(ctx, req.ID)
}

// Create creates a new attribute definition.
func (h *attributeDefinitionHandler) Create(ctx context.Context, req *dto.CreateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	return h.attrDefService.Create(ctx, req)
}

// Update updates an existing attribute definition.
func (h *attributeDefinitionHandler) Update(ctx context.Context, req *dto.UpdateAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	return h.attrDefService.Update(ctx, req.ID, req)
}

// Delete removes an attribute definition by ID.
func (h *attributeDefinitionHandler) Delete(ctx context.Context, req *dto.DeleteAttributeDefinitionRequest) (*dto.AttributeDefinitionResponse, error) {
	return nil, h.attrDefService.Delete(ctx, req.ID)
}
