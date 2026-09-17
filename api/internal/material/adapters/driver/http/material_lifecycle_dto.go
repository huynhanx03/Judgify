package http

// publishMaterialRequest is the HTTP representation of a publish lifecycle command.
type publishMaterialRequest struct {
	ID                string `uri:"id" validate:"required,uuid"`
	CommandID         string `json:"command_id" validate:"required,uuid"`
	DraftRevisionID   string `json:"draft_revision_id" validate:"required,uuid"`
	ExpectedVersion   int64  `json:"expected_version" validate:"required,min=1"`
	Reason            string `json:"reason" validate:"required,min=3,max=500"`
	ConfirmationToken string `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

// archiveMaterialRequest is shared by lifecycle actions without a revision target.
type archiveMaterialRequest struct {
	ID                string `uri:"id" validate:"required,uuid"`
	CommandID         string `json:"command_id" validate:"required,uuid"`
	ExpectedVersion   int64  `json:"expected_version" validate:"required,min=1"`
	Reason            string `json:"reason" validate:"required,min=3,max=500"`
	ConfirmationToken string `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type rollbackMaterialRequest struct {
	ID                string `uri:"id" validate:"required,uuid"`
	CommandID         string `json:"command_id" validate:"required,uuid"`
	TargetRevisionID  string `json:"target_revision_id" validate:"required,uuid"`
	ExpectedVersion   int64  `json:"expected_version" validate:"required,min=1"`
	Reason            string `json:"reason" validate:"required,min=3,max=500"`
	ConfirmationToken string `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type renameMaterialRequest struct {
	ID              string `uri:"id" validate:"required,uuid"`
	CommandID       string `json:"command_id" validate:"required,uuid"`
	Slug            string `json:"slug" validate:"required,min=1,max=160"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,min=1"`
	Reason          string `json:"reason" validate:"required,min=3,max=500"`
}
