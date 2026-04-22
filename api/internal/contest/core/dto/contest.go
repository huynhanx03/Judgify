package dto

import "time"

// CreateContestRequest represents request to create a contest.
type CreateContestRequest struct {
	Title           string    `json:"title" validate:"required,max=300"`
	Description     string    `json:"description"`
	StartTime       time.Time `json:"start_time" validate:"required"`
	EndTime         time.Time `json:"end_time" validate:"required"`
	MaxParticipants int       `json:"max_participants"`
	ProblemIDs      []int     `json:"problem_ids"`
}

// UpdateContestRequest represents request to update a contest.
type UpdateContestRequest struct {
	ID              int        `json:"-" uri:"id"`
	Title           *string    `json:"title,omitempty"`
	Description     *string    `json:"description,omitempty"`
	StartTime       *time.Time `json:"start_time,omitempty"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	MaxParticipants *int       `json:"max_participants,omitempty"`
	ProblemIDs      []int      `json:"problem_ids,omitempty"`
}

// GetContestRequest represents request to get a contest by ID.
type GetContestRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteContestRequest represents request to delete a contest.
type DeleteContestRequest struct {
	ID int `uri:"id" validate:"required"`
}

// ContestResponse represents contest data in API response.
type ContestResponse struct {
	ID               int       `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description,omitempty"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	Status           string    `json:"status"`
	AuthorID         int       `json:"author_id"`
	MaxParticipants  int       `json:"max_participants"`
	ParticipantCount int       `json:"participant_count"`
	ProblemIDs       []int     `json:"problem_ids,omitempty"`
	IsRegistered     *bool     `json:"is_registered,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
