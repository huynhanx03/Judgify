package http

// CreateTagRequest is the HTTP payload for creating a tag.
type CreateTagRequest struct {
	Name       string   `json:"name" validate:"required,min=1,max=50"`
	ElementIDs []string `json:"element_ids"`
}

// UpdateTagRequest is the HTTP payload for updating a tag.
type UpdateTagRequest struct {
	ID         string    `json:"-" uri:"id" validate:"required,uuid"`
	Name       *string   `json:"name" validate:"omitempty,min=1,max=50"`
	ElementIDs *[]string `json:"element_ids"`
}

type tagIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

// TagElementResponse is the HTTP representation of an element embedded in a tag.
type TagElementResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// TagResponse is the HTTP representation of a tag.
type TagResponse struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Elements []*TagElementResponse `json:"elements"`
	Version  int64                 `json:"version"`
}
