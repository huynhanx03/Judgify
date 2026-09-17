package http

type createElementRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Code        string `json:"code" validate:"required,min=1,max=20"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

type updateElementRequest struct {
	ID          string  `uri:"id" validate:"required,uuid"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	Code        *string `json:"code" validate:"omitempty,min=1,max=20"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

type elementIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

type elementResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Version     int64  `json:"version"`
}
