package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/ports"
)

// TagHandler defines the tag HTTP handler interface.
type TagHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.TagResponse], error)
	Get(ctx context.Context, req *dto.GetTagRequest) (*dto.TagResponse, error)
	Create(ctx context.Context, req *dto.CreateTagRequest) (*dto.TagResponse, error)
	Update(ctx context.Context, req *dto.UpdateTagRequest) (*dto.TagResponse, error)
	Delete(ctx context.Context, req *dto.DeleteTagRequest) (*dto.TagResponse, error)
}

type tagHandler struct {
	handler.BaseHandler
	tagService ports.TagService
}

// NewTagHandler creates a new TagHandler instance.
func NewTagHandler(tagService ports.TagService) TagHandler {
	return &tagHandler{tagService: tagService}
}

func (h *tagHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.TagResponse], error) {
	return h.tagService.Find(ctx, req)
}

func (h *tagHandler) Get(ctx context.Context, req *dto.GetTagRequest) (*dto.TagResponse, error) {
	return h.tagService.Get(ctx, req.ID)
}

func (h *tagHandler) Create(ctx context.Context, req *dto.CreateTagRequest) (*dto.TagResponse, error) {
	return h.tagService.Create(ctx, req)
}

func (h *tagHandler) Update(ctx context.Context, req *dto.UpdateTagRequest) (*dto.TagResponse, error) {
	return h.tagService.Update(ctx, req.ID, req)
}

func (h *tagHandler) Delete(ctx context.Context, req *dto.DeleteTagRequest) (*dto.TagResponse, error) {
	return nil, h.tagService.Delete(ctx, req.ID)
}
