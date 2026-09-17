package domain

import (
	"time"

	"github.com/google/uuid"
)

const RatingAlgorithmExpectedRankV1 = "expected-rank-v1"

// Contest represents a programming contest.
type Contest struct {
	ID                         uuid.UUID
	Title                      string
	Description                string
	StartTime                  time.Time
	EndTime                    time.Time
	Status                     string // compatibility projection; State is authoritative after cutover.
	State                      ContestState
	Version                    int64
	AuthorID                   uuid.UUID
	MaxParticipants            int // compatibility projection; ParticipantLimit is authoritative.
	ParticipantLimit           *int
	RegistrationMode           RegistrationMode
	RegistrationOpensAt        *time.Time
	RegistrationClosesAt       *time.Time
	ScoringPolicy              ScoringPolicy
	ScoringPolicyVersion       int64
	StandingsProjectionVersion int64
	FreezeStandingsAt          *time.Time
	Rated                      bool
	RatingAlgorithmVersion     string
	NextSubmissionSequence     int64
	PublishedAt                *time.Time
	StartedAt                  *time.Time
	EndedAt                    *time.Time
	CancelledAt                *time.Time
	RatingCompletedAt          *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// EffectiveState keeps old readers functional during the generated-client
// cutover while ensuring new command code uses the closed state type.
func (contest Contest) EffectiveState() ContestState {
	if contest.State != "" {
		return contest.State
	}
	return ContestState(contest.Status)
}

// AcceptsOfficialSubmissionAt implements the authoritative start-inclusive,
// end-exclusive window. Callers supply PostgreSQL time, never client time.
func (contest Contest) AcceptsOfficialSubmissionAt(now time.Time) bool {
	return contest.EffectiveState() == ContestRunning &&
		!now.IsZero() && !contest.StartTime.IsZero() && !contest.EndTime.IsZero() &&
		!now.Before(contest.StartTime) && now.Before(contest.EndTime)
}
