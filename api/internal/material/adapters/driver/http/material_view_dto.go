package http

// RecordMaterialViewRequest is the HTTP path input for a material view.
type RecordMaterialViewRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

// RecordMaterialViewResponse is the HTTP representation of view attribution.
type RecordMaterialViewResponse struct {
	Counted bool `json:"counted"`
}
