package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidSolveState      = errors.New("submission: invalid solve state")
	ErrInvalidSolveTransition = errors.New("submission: invalid solve transition")
)

type SolveStatus string

const (
	SolveUnsolved SolveStatus = "unsolved"
	SolveSolved   SolveStatus = "solved"
)

type RewardProvenance string

const (
	RewardProvenanceLive RewardProvenance = "live"
)

type SolveTransitionProvenance string

const (
	SolveTransitionLive SolveTransitionProvenance = "live"
)

// SolveSelection identifies the earliest currently active accepted result.
// RewardGrantSourceID is the first-solve transition whose grant remains live.
type SolveSelection struct {
	Status              SolveStatus
	SubmissionID        *uuid.UUID
	JudgementID         *uuid.UUID
	Generation          *int64
	ProblemRevisionID   *uuid.UUID
	RewardFacts         json.RawMessage
	RewardFactsChecksum *[32]byte
	RewardGrantSourceID *uuid.UUID
	RewardProvenance    *RewardProvenance
	SolvedAt            *time.Time
}

type UserProblemSolve struct {
	ID                       uuid.UUID
	UserID                   uuid.UUID
	ProblemID                uuid.UUID
	Version                  int64
	Selection                SolveSelection
	ActiveRewardTransitionID *uuid.UUID
}

type SolveTransition struct {
	ID            uuid.UUID
	SolveID       uuid.UUID
	UserID        uuid.UUID
	ProblemID     uuid.UUID
	FromVersion   int64
	ToVersion     int64
	From          SolveSelection
	To            SolveSelection
	Provenance    SolveTransitionProvenance
	OccurredAt    time.Time
	CorrelationID string
}

type SolveTransitionMetadata struct {
	ID            uuid.UUID
	Provenance    SolveTransitionProvenance
	OccurredAt    time.Time
	CorrelationID string
}

func NewUnsolvedSolve(id, userID, problemID uuid.UUID) (UserProblemSolve, error) {
	solve := UserProblemSolve{
		ID: id, UserID: userID, ProblemID: problemID,
		Selection: SolveSelection{Status: SolveUnsolved},
	}
	return solve, solve.Validate()
}

func (solve UserProblemSolve) Validate() error {
	if solve.ID == uuid.Nil || solve.UserID == uuid.Nil || solve.ProblemID == uuid.Nil ||
		solve.Version < 0 || solve.Selection.Validate() != nil {
		return ErrInvalidSolveState
	}
	if solve.Selection.Status == SolveSolved {
		if solve.Version < 1 || solve.ActiveRewardTransitionID == nil ||
			*solve.ActiveRewardTransitionID == uuid.Nil {
			return ErrInvalidSolveState
		}
	} else if solve.ActiveRewardTransitionID != nil {
		return ErrInvalidSolveState
	}
	return nil
}

func (selection SolveSelection) Validate() error {
	if selection.Status != SolveSolved && selection.Status != SolveUnsolved {
		return ErrInvalidSolveState
	}
	present := 0
	if selection.SubmissionID != nil {
		present++
	}
	if selection.JudgementID != nil {
		present++
	}
	if selection.Generation != nil {
		present++
	}
	if selection.ProblemRevisionID != nil {
		present++
	}
	if selection.RewardFacts != nil {
		present++
	}
	if selection.RewardFactsChecksum != nil {
		present++
	}
	if selection.RewardGrantSourceID != nil {
		present++
	}
	if selection.RewardProvenance != nil {
		present++
	}
	if selection.SolvedAt != nil {
		present++
	}
	if selection.Status == SolveUnsolved {
		if present != 0 {
			return ErrInvalidSolveState
		}
		return nil
	}
	if present != 9 || *selection.SubmissionID == uuid.Nil ||
		*selection.JudgementID == uuid.Nil || *selection.Generation < 1 ||
		*selection.ProblemRevisionID == uuid.Nil || len(selection.RewardFacts) == 0 ||
		len(selection.RewardFacts) > 64<<10 || !json.Valid(selection.RewardFacts) ||
		!jsonObject(selection.RewardFacts) ||
		*selection.RewardFactsChecksum == ([32]byte{}) ||
		*selection.RewardGrantSourceID == uuid.Nil || selection.SolvedAt.IsZero() {
		return ErrInvalidSolveState
	}
	if *selection.RewardProvenance != RewardProvenanceLive {
		return ErrInvalidSolveState
	}
	return nil
}

// TransitionSolve produces the next projection and its single immutable,
// contiguous transition. Callers persist both in the same transaction.
func TransitionSolve(
	current UserProblemSolve,
	desired SolveSelection,
	metadata SolveTransitionMetadata,
) (UserProblemSolve, SolveTransition, error) {
	if err := current.Validate(); err != nil {
		return UserProblemSolve{}, SolveTransition{}, err
	}
	to := cloneSolveSelection(desired)
	if current.Selection.Status == SolveUnsolved && to.Status == SolveSolved {
		if to.RewardGrantSourceID != nil && *to.RewardGrantSourceID != metadata.ID {
			return UserProblemSolve{}, SolveTransition{}, ErrInvalidSolveTransition
		}
		grantSource := metadata.ID
		to.RewardGrantSourceID = &grantSource
		rewardProvenance := RewardProvenanceLive
		if to.RewardProvenance != nil && *to.RewardProvenance != rewardProvenance {
			return UserProblemSolve{}, SolveTransition{}, ErrInvalidSolveTransition
		}
		to.RewardProvenance = &rewardProvenance
	}
	if current.Selection.Status == SolveSolved && to.Status == SolveSolved {
		grantSource := *current.Selection.RewardGrantSourceID
		provenance := *current.Selection.RewardProvenance
		to.RewardGrantSourceID = &grantSource
		to.RewardProvenance = &provenance
	}
	if to.Validate() != nil || metadata.ID == uuid.Nil || metadata.OccurredAt.IsZero() ||
		metadata.CorrelationID == "" ||
		metadata.Provenance != SolveTransitionLive {
		return UserProblemSolve{}, SolveTransition{}, ErrInvalidSolveTransition
	}
	if current.Selection.Status == SolveUnsolved && desired.Status == SolveUnsolved {
		return UserProblemSolve{}, SolveTransition{}, ErrInvalidSolveTransition
	}
	if solveSelectionsEqual(current.Selection, to) {
		return UserProblemSolve{}, SolveTransition{}, ErrInvalidSolveTransition
	}

	next := current
	next.Version = current.Version + 1
	next.Selection = cloneSolveSelection(to)
	if to.Status == SolveSolved {
		transitionID := metadata.ID
		next.ActiveRewardTransitionID = &transitionID
	} else {
		next.ActiveRewardTransitionID = nil
	}
	if err := next.Validate(); err != nil {
		return UserProblemSolve{}, SolveTransition{}, err
	}
	transition := SolveTransition{
		ID: metadata.ID, SolveID: current.ID,
		UserID: current.UserID, ProblemID: current.ProblemID,
		FromVersion: current.Version, ToVersion: next.Version,
		From: cloneSolveSelection(current.Selection), To: cloneSolveSelection(to),
		Provenance: metadata.Provenance, OccurredAt: metadata.OccurredAt,
		CorrelationID: metadata.CorrelationID,
	}
	return next, transition, nil
}

func cloneSolveSelection(value SolveSelection) SolveSelection {
	result := value
	result.RewardFacts = append(json.RawMessage(nil), value.RewardFacts...)
	result.SubmissionID = cloneUUID(value.SubmissionID)
	result.JudgementID = cloneUUID(value.JudgementID)
	result.Generation = cloneInt64(value.Generation)
	result.ProblemRevisionID = cloneUUID(value.ProblemRevisionID)
	result.RewardFactsChecksum = cloneChecksum(value.RewardFactsChecksum)
	result.RewardGrantSourceID = cloneUUID(value.RewardGrantSourceID)
	result.RewardProvenance = cloneRewardProvenance(value.RewardProvenance)
	result.SolvedAt = cloneTime(value.SolvedAt)
	return result
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneChecksum(value *[32]byte) *[32]byte {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneRewardProvenance(value *RewardProvenance) *RewardProvenance {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func solveSelectionsEqual(first, second SolveSelection) bool {
	return first.Status == second.Status && equalUUID(first.SubmissionID, second.SubmissionID) &&
		equalUUID(first.JudgementID, second.JudgementID) &&
		equalInt64(first.Generation, second.Generation) &&
		equalUUID(first.ProblemRevisionID, second.ProblemRevisionID) &&
		bytes.Equal(first.RewardFacts, second.RewardFacts) &&
		equalChecksum(first.RewardFactsChecksum, second.RewardFactsChecksum) &&
		equalUUID(first.RewardGrantSourceID, second.RewardGrantSourceID) &&
		equalRewardProvenance(first.RewardProvenance, second.RewardProvenance) &&
		equalTime(first.SolvedAt, second.SolvedAt)
}

func equalUUID(first, second *uuid.UUID) bool {
	return first == nil && second == nil || first != nil && second != nil && *first == *second
}

func equalInt64(first, second *int64) bool {
	return first == nil && second == nil || first != nil && second != nil && *first == *second
}

func equalRewardProvenance(first, second *RewardProvenance) bool {
	return first == nil && second == nil || first != nil && second != nil && *first == *second
}

func equalTime(first, second *time.Time) bool {
	return first == nil && second == nil || first != nil && second != nil && first.Equal(*second)
}

func equalChecksum(first, second *[32]byte) bool {
	return first == nil && second == nil || first != nil && second != nil && *first == *second
}
