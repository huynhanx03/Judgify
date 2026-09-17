package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidJobTransition = errors.New("submission: invalid job transition")

type JobState string

const (
	JobReady     JobState = "ready"
	JobLeased    JobState = "leased"
	JobRetryWait JobState = "retry_wait"
	JobDone      JobState = "done"
	JobCancelled JobState = "cancelled"
	JobDead      JobState = "dead"
)

type AttemptState string

const (
	AttemptCreated              AttemptState = "created"
	AttemptPreparing            AttemptState = "preparing"
	AttemptCompiling            AttemptState = "compiling"
	AttemptRunning              AttemptState = "running"
	AttemptFinalizing           AttemptState = "finalizing"
	AttemptSucceeded            AttemptState = "succeeded"
	AttemptInfrastructureFailed AttemptState = "infrastructure_failed"
	AttemptLeaseLost            AttemptState = "lease_lost"
	AttemptCancelled            AttemptState = "cancelled"
)

// LeaseTokenHash is the only lease-secret representation repositories may
// persist or compare. The raw capability must never enter a domain record.
type LeaseTokenHash [32]byte

func (hash LeaseTokenHash) Valid() bool { return hash != (LeaseTokenHash{}) }

type JudgeJob struct {
	ID           uuid.UUID
	SubmissionID uuid.UUID
	Generation   int64
	JudgeSpecID  uuid.UUID
	Lane         string
	State        JobState
	AvailableAt  time.Time
	Attempts     uint32
	MaxAttempts  uint32
	OriginCID    string
}

type JudgeAttempt struct {
	ID             uuid.UUID
	JobID          uuid.UUID
	SubmissionID   uuid.UUID
	Generation     int64
	JudgeSpecID    uuid.UUID
	AttemptNumber  uint32
	LeaseTokenHash LeaseTokenHash
	WorkerID       uuid.UUID
	WorkerBootID   uuid.UUID
	State          AttemptState
	StartedAt      time.Time
	FinishedAt     *time.Time
}

// TransitionJobState accepts exactly the durable state-machine edges.
func TransitionJobState(current, next JobState) (JobState, error) {
	valid := false
	switch current {
	case JobReady:
		valid = next == JobLeased || next == JobCancelled
	case JobLeased:
		valid = next == JobDone || next == JobRetryWait ||
			next == JobCancelled || next == JobDead
	case JobRetryWait:
		valid = next == JobReady || next == JobCancelled || next == JobDead
	}
	if !valid {
		return "", ErrInvalidJobTransition
	}
	return next, nil
}

// TransitionAttemptState keeps infrastructure failures distinct from every
// contestant verdict.
func TransitionAttemptState(current, next AttemptState) (AttemptState, error) {
	valid := false
	switch current {
	case AttemptCreated:
		valid = next == AttemptPreparing || attemptFailureState(next)
	case AttemptPreparing:
		valid = next == AttemptCompiling || attemptFailureState(next)
	case AttemptCompiling:
		valid = next == AttemptRunning || next == AttemptFinalizing || attemptFailureState(next)
	case AttemptRunning:
		valid = next == AttemptFinalizing || attemptFailureState(next)
	case AttemptFinalizing:
		valid = next == AttemptSucceeded || attemptFailureState(next)
	}
	if !valid {
		return "", ErrInvalidJobTransition
	}
	return next, nil
}

func attemptFailureState(state AttemptState) bool {
	return state == AttemptInfrastructureFailed || state == AttemptLeaseLost ||
		state == AttemptCancelled
}
