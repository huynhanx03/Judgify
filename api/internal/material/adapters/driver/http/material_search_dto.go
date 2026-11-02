package http

type materialSearchRequest struct {
	Text         string   `json:"text" validate:"omitempty,max=256"`
	CategoryID   *string  `json:"category_id,omitempty" validate:"omitempty,uuid"`
	TagIDs       []string `json:"tag_ids,omitempty" validate:"max=64,dive,uuid"`
	DifficultyID *string  `json:"difficulty_id,omitempty" validate:"omitempty,uuid"`
	Sort         string   `json:"sort" validate:"omitempty,oneof=relevance published popularity"`
	Cursor       string   `json:"cursor" validate:"omitempty,max=512"`
	Limit        int      `json:"limit" validate:"omitempty,min=1,max=100"`
}

type adminMaterialSearchRequest struct {
	Text         string   `json:"text" validate:"omitempty,max=256"`
	CategoryID   *string  `json:"category_id,omitempty" validate:"omitempty,uuid"`
	TagIDs       []string `json:"tag_ids,omitempty" validate:"max=64,dive,uuid"`
	DifficultyID *string  `json:"difficulty_id,omitempty" validate:"omitempty,uuid"`
	Status       *string  `json:"status,omitempty" validate:"omitempty,oneof=draft in_review published archived"`
	Cursor       string   `json:"cursor" validate:"omitempty,max=512"`
	Limit        int      `json:"limit" validate:"omitempty,min=1,max=100"`
}
