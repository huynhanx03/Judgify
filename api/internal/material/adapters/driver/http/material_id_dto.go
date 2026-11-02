package http

// materialIDRequest is the HTTP path input shared by material ID endpoints.
type materialIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}
