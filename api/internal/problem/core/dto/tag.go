package dto

// CreateTagRequest represents request to create a tag.
type CreateTagRequest struct {
	Name       string `json:"name" validate:"required,min=1,max=50"`
	ElementIDs []int  `json:"element_ids"`
}

// UpdateTagRequest represents request to update a tag.
type UpdateTagRequest struct {
	ID         int     `json:"-" uri:"id"`
	Name       *string `json:"name" validate:"omitempty,min=1,max=50"`
	ElementIDs *[]int  `json:"element_ids"`
}

// GetTagRequest represents request to get a tag by ID.
type GetTagRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteTagRequest represents request to delete a tag.
type DeleteTagRequest struct {
	ID int `uri:"id" validate:"required"`
}

// FindAllTagsRequest is an empty request for GET /tags.
type FindAllTagsRequest struct{}

// TagElementInfo represents element data nested inside a tag response.
type TagElementInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// TagResponse represents tag data in API response.
type TagResponse struct {
	ID       int               `json:"id"`
	Name     string            `json:"name"`
	Elements []*TagElementInfo `json:"elements"`
}
