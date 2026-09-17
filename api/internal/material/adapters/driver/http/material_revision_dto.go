package http

// ListMaterialRevisionsRequest is the HTTP input for a material revision page.
type ListMaterialRevisionsRequest struct {
	ID     string `uri:"id" validate:"required,uuid"`
	Cursor string `form:"cursor" validate:"omitempty,max=32"`
	Limit  int    `form:"limit" validate:"omitempty,min=1,max=100"`
}

// GetMaterialRevisionRequest is the HTTP path input for one revision.
type GetMaterialRevisionRequest struct {
	ID         string `uri:"id" validate:"required,uuid"`
	RevisionID string `uri:"revision_id" validate:"required,uuid"`
}
