package http

type createMaterialRequest struct {
	CommandID     string   `json:"command_id" validate:"required,uuid"`
	Slug          string   `json:"slug" validate:"required,min=1,max=160"`
	Title         string   `json:"title" validate:"required,min=1,max=255"`
	Description   string   `json:"description" validate:"omitempty,max=1000"`
	Content       string   `json:"content"`
	DifficultyID  string   `json:"difficulty_id" validate:"required,uuid"`
	CategoryID    string   `json:"category_id" validate:"required,uuid"`
	TagIDs        []string `json:"tag_ids" validate:"dive,uuid"`
	ChangeSummary string   `json:"change_summary" validate:"required,min=3,max=500"`
	Status        string   `json:"status" validate:"omitempty,oneof=draft in_review"`
	Visibility    string   `json:"visibility" validate:"omitempty,oneof=public authenticated"`
}

type updateMaterialRequest struct {
	ID              string    `uri:"id" validate:"required,uuid"`
	CommandID       string    `json:"command_id" validate:"required,uuid"`
	Title           *string   `json:"title" validate:"omitempty,min=1,max=255"`
	Description     *string   `json:"description" validate:"omitempty,max=1000"`
	Content         *string   `json:"content"`
	DifficultyID    *string   `json:"difficulty_id" validate:"omitempty,uuid"`
	CategoryID      *string   `json:"category_id" validate:"omitempty,uuid"`
	TagIDs          *[]string `json:"tag_ids" validate:"omitempty,dive,uuid"`
	ExpectedVersion int64     `json:"expected_version" validate:"required,min=1"`
	ChangeSummary   string    `json:"change_summary" validate:"required,min=3,max=500"`
	Status          *string   `json:"status" validate:"omitempty,oneof=draft in_review"`
	Visibility      *string   `json:"visibility" validate:"omitempty,oneof=public authenticated"`
}
