package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// TagHandler defines the tag HTTP handler interface.
type TagHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*TagResponse], error)
	FindAll(ctx context.Context) ([]*TagResponse, error)
	Get(ctx context.Context, req *tagIDRequest) (*TagResponse, error)
	Create(ctx context.Context, req *CreateTagRequest) (*TagResponse, error)
	Update(ctx context.Context, req *UpdateTagRequest) (*TagResponse, error)
	Delete(ctx context.Context, req *tagIDRequest) (*TagResponse, error)
}

type tagHandler struct {
	handler.BaseHandler
	tagService ports.TagService
}

// NewTagHandler creates a new TagHandler instance.
func NewTagHandler(tagService ports.TagService) TagHandler {
	return &tagHandler{tagService: tagService}
}

func (h *tagHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*TagResponse], error) {
	results, err := h.tagService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	responses := make([]*TagResponse, 0)
	if results.Records != nil {
		responses = make([]*TagResponse, len(*results.Records))
		for index, result := range *results.Records {
			responses[index] = toTagResponse(result)
		}
	}
	return &d.Paginated[*TagResponse]{Records: &responses, Pagination: results.Pagination}, nil
}

func (h *tagHandler) FindAll(ctx context.Context) ([]*TagResponse, error) {
	results, err := h.tagService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*TagResponse, len(results))
	for index, result := range results {
		responses[index] = toTagResponse(result)
	}
	return responses, nil
}

func (h *tagHandler) Get(ctx context.Context, req *tagIDRequest) (*TagResponse, error) {
	id, err := toTagID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.tagService.Get(ctx, id)
	return toTagResponse(result), err
}

func (h *tagHandler) Create(ctx context.Context, req *CreateTagRequest) (*TagResponse, error) {
	command, err := toCreateTagCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.tagService.Create(ctx, &command)
	return toTagResponse(result), err
}

func (h *tagHandler) Update(ctx context.Context, req *UpdateTagRequest) (*TagResponse, error) {
	id, command, err := toUpdateTagCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.tagService.Update(ctx, id, &command)
	return toTagResponse(result), err
}

func (h *tagHandler) Delete(ctx context.Context, req *tagIDRequest) (*TagResponse, error) {
	id, err := toTagID(req)
	if err != nil {
		return nil, err
	}
	return nil, h.tagService.Delete(ctx, id)
}
