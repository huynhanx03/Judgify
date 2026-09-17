package http

type createRarityRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Code        string `json:"code" validate:"required,min=1,max=20"`
	Weight      int    `json:"weight" validate:"omitempty,min=1"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

type updateRarityRequest struct {
	ID          string  `uri:"id" validate:"required,uuid"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	Code        *string `json:"code" validate:"omitempty,min=1,max=20"`
	Weight      *int    `json:"weight" validate:"omitempty,min=1"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

type rarityIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

type rarityResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Weight      int    `json:"weight"`
	Description string `json:"description,omitempty"`
}
