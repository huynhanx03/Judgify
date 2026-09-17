package http

type createLevelRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	MinExp      int64  `json:"min_exp" validate:"min=0"`
	Description string `json:"description" validate:"omitempty,max=500"`
}

type updateLevelRequest struct {
	ID          string  `uri:"id" validate:"required,uuid"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	MinExp      *int64  `json:"min_exp" validate:"omitempty,min=0"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

type levelIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

type levelResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MinExp      int64  `json:"min_exp"`
	Description string `json:"description,omitempty"`
}
