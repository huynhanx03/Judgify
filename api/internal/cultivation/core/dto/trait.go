package dto

// CreateTraitRequest represents request to create a trait.
type CreateTraitRequest struct {
	Type        string                 `json:"type" validate:"required,oneof=root_bone talent"`
	Name        string                 `json:"name" validate:"required,min=1,max=100"`
	RarityID    int                    `json:"rarity_id" validate:"required,min=1"`
	Description string                 `json:"description" validate:"omitempty,max=500"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UpdateTraitRequest represents request to update a trait.
type UpdateTraitRequest struct {
	ID          int                     `json:"-" uri:"id"`
	Type        *string                 `json:"type" validate:"omitempty,oneof=root_bone talent"`
	Name        *string                 `json:"name" validate:"omitempty,min=1,max=100"`
	RarityID    *int                    `json:"rarity_id" validate:"omitempty,min=1"`
	Description *string                 `json:"description" validate:"omitempty,max=500"`
	Metadata    *map[string]interface{} `json:"metadata"`
}

// GetTraitRequest represents request to get a trait by ID.
type GetTraitRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteTraitRequest represents request to delete a trait.
type DeleteTraitRequest struct {
	ID int `uri:"id" validate:"required"`
}

// TraitResponse represents trait data in API response.
type TraitResponse struct {
	ID          int                    `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	RarityID    int                    `json:"rarity_id"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
