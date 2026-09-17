package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/pkg/judging"
)

// Phase describes only the newest generation's execution lifecycle.
type Phase string

const (
	PhaseQueued     Phase = "queued"
	PhaseClaimed    Phase = "claimed"
	PhaseCompiling  Phase = "compiling"
	PhaseRunning    Phase = "running"
	PhaseFinalizing Phase = "finalizing"
	PhaseTerminal   Phase = "terminal"
	PhaseCancelled  Phase = "cancelled"
)

// Verdict is a closed terminal outcome. SystemError is durable operational
// history but is deliberately not a contestant verdict.
type Verdict string

const (
	VerdictAccepted             Verdict = "accepted"
	VerdictWrongAnswer          Verdict = "wrong_answer"
	VerdictCompileError         Verdict = "compile_error"
	VerdictRuntimeError         Verdict = "runtime_error"
	VerdictTimeLimitExceeded    Verdict = "time_limit_exceeded"
	VerdictMemoryLimitExceeded  Verdict = "memory_limit_exceeded"
	VerdictProcessLimitExceeded Verdict = "process_limit_exceeded"
	VerdictOutputLimitExceeded  Verdict = "output_limit_exceeded"
	VerdictSystemError          Verdict = "system_error"
)

var (
	ErrInvalidSubmissionState = errors.New("submission: invalid aggregate state")
	ErrInvalidStateTransition = errors.New("submission: invalid state transition")
	ErrStaleGeneration        = errors.New("submission: stale generation")
)

// SubmissionState is the optimistic state-transition surface. Latest work
// and the active contestant judgement are intentionally separate.
type SubmissionState struct {
	ID                uuid.UUID
	LatestGeneration  int64
	LatestPhase       Phase
	LatestOutcome     string
	ActiveGeneration  *int64
	ActiveJudgementID *uuid.UUID
	ActiveVerdict     *Verdict
	AggregateVersion  int64
}

// GenerationResult closes one generation. Contestant outcomes carry their
// immutable judgement; system error and cancellation never become active.
type GenerationResult struct {
	Generation int64
	Outcome    string
	Judgement  *Judgement
}

func (phase Phase) valid() bool {
	switch phase {
	case PhaseQueued, PhaseClaimed, PhaseCompiling, PhaseRunning,
		PhaseFinalizing, PhaseTerminal, PhaseCancelled:
		return true
	default:
		return false
	}
}

func (verdict Verdict) Valid() bool {
	switch verdict {
	case VerdictAccepted, VerdictWrongAnswer, VerdictCompileError,
		VerdictRuntimeError, VerdictTimeLimitExceeded,
		VerdictMemoryLimitExceeded, VerdictProcessLimitExceeded,
		VerdictOutputLimitExceeded, VerdictSystemError:
		return true
	default:
		return false
	}
}

func (verdict Verdict) Contestant() bool {
	return verdict.Valid() && verdict != VerdictSystemError
}

func (state SubmissionState) Validate() error {
	if state.ID == uuid.Nil || state.LatestGeneration < 1 || !state.LatestPhase.valid() ||
		state.AggregateVersion < 0 {
		return ErrInvalidSubmissionState
	}
	terminal := state.LatestPhase == PhaseTerminal
	cancelled := state.LatestPhase == PhaseCancelled
	switch {
	case terminal:
		outcome := Verdict(state.LatestOutcome)
		if !outcome.Valid() {
			return ErrInvalidSubmissionState
		}
	case cancelled:
		if state.LatestOutcome != "cancelled" {
			return ErrInvalidSubmissionState
		}
	case state.LatestOutcome != "":
		return ErrInvalidSubmissionState
	}
	activeCount := 0
	if state.ActiveGeneration != nil {
		activeCount++
	}
	if state.ActiveJudgementID != nil {
		activeCount++
	}
	if state.ActiveVerdict != nil {
		activeCount++
	}
	if activeCount != 0 && activeCount != 3 {
		return ErrInvalidSubmissionState
	}
	if activeCount == 3 && (*state.ActiveGeneration < 1 ||
		*state.ActiveGeneration > state.LatestGeneration ||
		*state.ActiveJudgementID == uuid.Nil || !state.ActiveVerdict.Contestant()) {
		return ErrInvalidSubmissionState
	}
	return nil
}

// StartGeneration creates only the next contiguous generation and preserves
// the previously active judgement during a rejudge.
func StartGeneration(current SubmissionState, generation int64) (SubmissionState, error) {
	if current.ID == uuid.Nil || current.AggregateVersion < 0 ||
		generation < 1 || generation != current.LatestGeneration+1 {
		return SubmissionState{}, ErrInvalidStateTransition
	}
	if current.LatestGeneration > 0 {
		if err := current.Validate(); err != nil {
			return SubmissionState{}, err
		}
		if current.LatestPhase != PhaseTerminal && current.LatestPhase != PhaseCancelled {
			return SubmissionState{}, ErrInvalidStateTransition
		}
	} else if current.LatestPhase != "" || current.LatestOutcome != "" ||
		current.ActiveGeneration != nil || current.ActiveJudgementID != nil ||
		current.ActiveVerdict != nil {
		return SubmissionState{}, ErrInvalidStateTransition
	}
	next := current
	next.LatestGeneration = generation
	next.LatestPhase = PhaseQueued
	next.LatestOutcome = ""
	next.AggregateVersion++
	return next, next.Validate()
}

// TransitionPhase applies one edge from the closed generation state machine.
func TransitionPhase(current SubmissionState, generation int64, nextPhase Phase) (SubmissionState, error) {
	if err := current.Validate(); err != nil {
		return SubmissionState{}, err
	}
	if generation != current.LatestGeneration {
		return SubmissionState{}, ErrStaleGeneration
	}
	if !validPhaseTransition(current.LatestPhase, nextPhase) {
		return SubmissionState{}, fmt.Errorf(
			"%w: %s to %s", ErrInvalidStateTransition, current.LatestPhase, nextPhase,
		)
	}
	next := current
	next.LatestPhase = nextPhase
	next.LatestOutcome = ""
	next.AggregateVersion++
	return next, next.Validate()
}

func validPhaseTransition(from, to Phase) bool {
	switch from {
	case PhaseQueued:
		return to == PhaseClaimed || to == PhaseCancelled
	case PhaseClaimed:
		return to == PhaseCompiling || to == PhaseQueued || to == PhaseCancelled
	case PhaseCompiling:
		return to == PhaseRunning || to == PhaseFinalizing || to == PhaseQueued || to == PhaseCancelled
	case PhaseRunning:
		return to == PhaseFinalizing || to == PhaseQueued || to == PhaseCancelled
	case PhaseFinalizing:
		return to == PhaseQueued || to == PhaseCancelled
	default:
		return false
	}
}

// FinishGeneration closes cancellation/system-error history without changing
// the active result, or atomically applies a valid contestant judgement.
func FinishGeneration(current SubmissionState, result GenerationResult) (SubmissionState, error) {
	if err := current.Validate(); err != nil {
		return SubmissionState{}, err
	}
	if result.Generation != current.LatestGeneration {
		return SubmissionState{}, ErrStaleGeneration
	}
	switch result.Outcome {
	case "cancelled":
		if result.Judgement != nil || current.LatestPhase == PhaseTerminal ||
			current.LatestPhase == PhaseCancelled {
			return SubmissionState{}, ErrInvalidStateTransition
		}
		next := current
		next.LatestPhase = PhaseCancelled
		next.LatestOutcome = "cancelled"
		next.AggregateVersion++
		return next, next.Validate()
	case string(VerdictSystemError):
		if current.LatestPhase == PhaseTerminal || current.LatestPhase == PhaseCancelled {
			return SubmissionState{}, ErrInvalidStateTransition
		}
		if result.Judgement == nil || result.Judgement.SubmissionID != current.ID ||
			result.Judgement.Generation != result.Generation ||
			result.Judgement.Verdict != VerdictSystemError ||
			result.Judgement.Validate() != nil {
			return SubmissionState{}, ErrInvalidStateTransition
		}
		next := current
		next.LatestPhase = PhaseTerminal
		next.LatestOutcome = string(VerdictSystemError)
		next.AggregateVersion++
		return next, next.Validate()
	default:
		if result.Judgement == nil || result.Outcome != string(result.Judgement.Verdict) {
			return SubmissionState{}, ErrInvalidStateTransition
		}
		return ApplyJudgement(current, *result.Judgement)
	}
}

// ApplyJudgement makes a contestant evaluation active. Infrastructure and
// cancellation outcomes cannot call this path.
func ApplyJudgement(current SubmissionState, judgement Judgement) (SubmissionState, error) {
	if err := current.Validate(); err != nil {
		return SubmissionState{}, err
	}
	if judgement.SubmissionID != current.ID || judgement.Generation != current.LatestGeneration {
		return SubmissionState{}, ErrStaleGeneration
	}
	if current.LatestPhase != PhaseFinalizing || judgement.Validate() != nil ||
		!judgement.Verdict.Contestant() {
		return SubmissionState{}, ErrInvalidStateTransition
	}
	generation := judgement.Generation
	judgementID := judgement.ID
	verdict := judgement.Verdict
	next := current
	next.LatestPhase = PhaseTerminal
	next.LatestOutcome = string(verdict)
	next.ActiveGeneration = &generation
	next.ActiveJudgementID = &judgementID
	next.ActiveVerdict = &verdict
	next.AggregateVersion++
	return next, next.Validate()
}

// Submission represents a code submission for a problem.
type Submission struct {
	ID                          uuid.UUID  `json:"id"`
	ProblemID                   uuid.UUID  `json:"problem_id"`
	UserID                      uuid.UUID  `json:"user_id"`
	ContestID                   *uuid.UUID `json:"contest_id,omitempty"`
	ContestProblemID            *uuid.UUID `json:"contest_problem_id,omitempty"`
	ContestScoringPolicyVersion *int64     `json:"contest_scoring_policy_version,omitempty"`
	SubmittedAt                 time.Time  `json:"submitted_at"`
	ContestElapsedMilliseconds  *int64     `json:"contest_elapsed_ms,omitempty"`
	ContestSubmissionSequence   *int64     `json:"contest_submission_sequence,omitempty"`
	Language                    string     `json:"language"`
	RuntimeKey                  string     `json:"runtime_key"`
	SourceCode                  string     `json:"source_code"`
	SourceSHA256                [32]byte   `json:"-"`
	Version                     int64      `json:"version"`
	LatestGeneration            int64      `json:"latest_generation"`
	LatestGenerationPhase       Phase      `json:"latest_generation_phase"`
	LatestGenerationOutcome     string     `json:"latest_generation_outcome,omitempty"`
	ActiveJudgementID           *uuid.UUID `json:"active_judgement_id,omitempty"`
	ActiveJudgementGeneration   *int64     `json:"active_judgement_generation,omitempty"`
	ActiveVerdict               *Verdict   `json:"active_verdict,omitempty"`
	IdempotencyKey              string     `json:"-"`
	RequestHash                 [32]byte   `json:"-"`
	OriginCID                   string     `json:"-"`
	LatestGenerationQueuedAt    time.Time  `json:"latest_generation_queued_at"`
	LatestGenerationStartedAt   *time.Time `json:"latest_generation_started_at,omitempty"`
	LatestGenerationCompletedAt *time.Time `json:"latest_generation_completed_at,omitempty"`
	Status                      string     `json:"status"`
	JudgedAt                    *time.Time `json:"judged_at,omitempty"`
	VerdictVersion              int64      `json:"verdict_version"`
	PassedCount                 int        `json:"passed_count"`
	TotalCount                  int        `json:"total_count"`
	TimeMs                      *int       `json:"time_ms"`
	MemoryKb                    *int       `json:"memory_kb"`
	ErrorMessage                *string    `json:"error_message"`
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}

// ValidateSourceCode applies the same byte contract used by persistence and
// the sandbox without coupling the submission domain to either adapter.
func (submission *Submission) ValidateSourceCode() error {
	if submission == nil {
		return &judging.ContentViolation{
			Field: judging.FieldSourceCode, Kind: judging.ViolationRequired,
			MaximumBytes: judging.MaximumSourceCodeBytes,
		}
	}
	return judging.ValidateSourceCode(submission.SourceCode)
}
