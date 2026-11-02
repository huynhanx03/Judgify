package http

import (
	"time"

	"github.com/google/uuid"
)

type publishContestCommandRequest struct {
	Reason string `json:"reason" validate:"required,max=1024"`
}
type contestLifecycleResponse struct {
	ContestID        string    `json:"contest_id"`
	From             string    `json:"from"`
	To               string    `json:"to"`
	Version          int64     `json:"version"`
	OccurredAt       time.Time `json:"occurred_at"`
	CommandID        string    `json:"command_id"`
	IdempotentReplay bool      `json:"idempotent_replay"`
	CID              string    `json:"cid"`
}
type getContestRequest struct {
	ID uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
}
type getContestProblemRequest struct {
	ContestID        uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
	ContestProblemID uuid.UUID `uri:"contest_problem_id,parser=encoding.TextUnmarshaler" validate:"required"`
}
type contestDraftProblemRequest struct {
	ProblemID          uuid.UUID `json:"problem_id" validate:"required"`
	Alias              string    `json:"alias" validate:"required,max=16"`
	Points             *int      `json:"points,omitempty" validate:"omitempty,min=1"`
	VisibleBeforeStart bool      `json:"visible_before_start"`
}
type contestDraftRequest struct {
	Reason               string                       `json:"reason" validate:"required,max=1024"`
	Title                string                       `json:"title" validate:"required,max=300"`
	Description          string                       `json:"description" validate:"max=100000"`
	StartTime            time.Time                    `json:"start_time" validate:"required"`
	EndTime              time.Time                    `json:"end_time" validate:"required"`
	RegistrationMode     string                       `json:"registration_mode" validate:"required,oneof=registered open"`
	RegistrationOpensAt  *time.Time                   `json:"registration_opens_at,omitempty"`
	RegistrationClosesAt *time.Time                   `json:"registration_closes_at,omitempty"`
	MaxParticipants      *int                         `json:"max_participants,omitempty" validate:"omitempty,min=1,max=1000000"`
	FreezeStandingsAt    *time.Time                   `json:"freeze_standings_at,omitempty"`
	Rated                *bool                        `json:"rated,omitempty"`
	Problems             []contestDraftProblemRequest `json:"problems" validate:"max=500,dive"`
}
type createContestRequest struct{ contestDraftRequest }
type updateContestRequest struct {
	contestDraftRequest
	ID              uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
	ExpectedVersion int64     `json:"expected_version" validate:"required,min=1"`
}
type deleteContestRequest struct {
	ID     uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
	Reason string    `json:"reason" validate:"required,max=1024"`
}
