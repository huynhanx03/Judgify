package domain

import "errors"

var (
	ErrContestNotDraft             = errors.New("contest: transition requires draft status")
	ErrContestPublishWindow        = errors.New("contest: publish requires a future valid time window")
	ErrContestPublishProblems      = errors.New("contest: publish requires at least one published problem")
	ErrContestNotFound             = errors.New("contest: not found")
	ErrContestRegistrationNotOpen  = errors.New("contest: registration is not open")
	ErrContestRegistrationStarted  = errors.New("contest: registration has closed because the contest started")
	ErrContestAlreadyRegistered    = errors.New("contest: user is already registered")
	ErrContestRegistrationFull     = errors.New("contest: registration capacity reached")
	ErrContestRegistrationNotFound = errors.New("contest: user registration not found")
	ErrContestRegistrationRequired = errors.New("contest: active registration required")
)
