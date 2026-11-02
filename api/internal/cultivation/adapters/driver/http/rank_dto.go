package http

type createRankRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	MinRating   int    `json:"min_rating" validate:"min=0"`
	Description string `json:"description" validate:"omitempty,max=500"`
}

type updateRankRequest struct {
	ID          string  `uri:"id" validate:"required,uuid"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	MinRating   *int    `json:"min_rating" validate:"omitempty,min=0"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

type rankIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

type rankResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MinRating   int    `json:"min_rating"`
	Description string `json:"description,omitempty"`
}
