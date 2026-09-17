package http

// GetMaterialBySlugRequest is the HTTP path input for resolving a slug.
type GetMaterialBySlugRequest struct {
	Slug string `uri:"slug" validate:"required,min=1,max=160"`
}

// RelatedMaterialRequest is the HTTP input for related-material lookup.
type RelatedMaterialRequest struct {
	Slug  string `uri:"slug" validate:"required,min=1,max=160"`
	Limit int    `form:"limit" validate:"omitempty,min=1,max=20"`
}
