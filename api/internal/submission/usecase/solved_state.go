package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

var ErrInvalidSolvedStatePlan = errors.New(
	"submission finalization: invalid solved-state plan",
)

// AcceptedSolveCandidate is the earliest active Accepted judgement selected
// by persistence in deterministic (submitted_at, submission_id) order.
type AcceptedSolveCandidate struct {
	SubmissionID        uuid.UUID
	JudgementID         uuid.UUID
	Generation          int64
	ProblemRevisionID   uuid.UUID
	RewardFacts         json.RawMessage
	RewardFactsChecksum [32]byte
	SolvedAt            time.Time
}

// DesiredSolveSelection converts the database-selected Accepted candidate into
// the domain target. A nil candidate means no active Accepted judgement.
func DesiredSolveSelection(
	candidate *AcceptedSolveCandidate,
) (entity.SolveSelection, error) {
	if candidate == nil {
		return entity.SolveSelection{Status: entity.SolveUnsolved}, nil
	}
	if candidate.SubmissionID == uuid.Nil ||
		candidate.JudgementID == uuid.Nil ||
		candidate.Generation < 1 ||
		candidate.ProblemRevisionID == uuid.Nil ||
		len(candidate.RewardFacts) == 0 ||
		len(candidate.RewardFacts) > 64<<10 ||
		!json.Valid(candidate.RewardFacts) ||
		candidate.RewardFactsChecksum == ([32]byte{}) ||
		candidate.SolvedAt.IsZero() {
		return entity.SolveSelection{}, ErrInvalidSolvedStatePlan
	}
	submissionID := candidate.SubmissionID
	judgementID := candidate.JudgementID
	generation := candidate.Generation
	problemRevisionID := candidate.ProblemRevisionID
	rewardChecksum := candidate.RewardFactsChecksum
	solvedAt := candidate.SolvedAt.UTC()
	return entity.SolveSelection{
		Status:              entity.SolveSolved,
		SubmissionID:        &submissionID,
		JudgementID:         &judgementID,
		Generation:          &generation,
		ProblemRevisionID:   &problemRevisionID,
		RewardFacts:         append(json.RawMessage(nil), candidate.RewardFacts...),
		RewardFactsChecksum: &rewardChecksum,
		SolvedAt:            &solvedAt,
	}, nil
}

// PlanSolvedState appends exactly one contiguous transition when any canonical
// projection field changes. Grant identity/provenance survive solved→solved
// pointer movement and are allocated only by unsolved→solved.
func PlanSolvedState(
	current entity.UserProblemSolve,
	desired entity.SolveSelection,
	metadata entity.SolveTransitionMetadata,
) (entity.UserProblemSolve, *entity.SolveTransition, bool, error) {
	if current.Validate() != nil {
		return entity.UserProblemSolve{}, nil, false, ErrInvalidSolvedStatePlan
	}
	normalized, err := normalizeDesiredSolve(current.Selection, desired)
	if err != nil {
		return entity.UserProblemSolve{}, nil, false, err
	}
	if solveSelectionEqual(current.Selection, normalized) {
		return current, nil, false, nil
	}
	next, transition, err := entity.TransitionSolve(current, normalized, metadata)
	if err != nil {
		return entity.UserProblemSolve{}, nil, false, err
	}
	return next, &transition, true, nil
}

// SolvedStateChanges lets persistence avoid taking the shared reward timeline
// lock and reading its linearized clock when the canonical projection is
// already identical.
func SolvedStateChanges(
	current entity.UserProblemSolve,
	desired entity.SolveSelection,
) (bool, error) {
	if current.Validate() != nil {
		return false, ErrInvalidSolvedStatePlan
	}
	normalized, err := normalizeDesiredSolve(current.Selection, desired)
	if err != nil {
		return false, err
	}
	return !solveSelectionEqual(current.Selection, normalized), nil
}

func normalizeDesiredSolve(
	current entity.SolveSelection,
	desired entity.SolveSelection,
) (entity.SolveSelection, error) {
	result := cloneServiceSolveSelection(desired)
	switch result.Status {
	case entity.SolveUnsolved:
		if result.SubmissionID != nil || result.JudgementID != nil ||
			result.Generation != nil || result.ProblemRevisionID != nil ||
			result.RewardFacts != nil || result.RewardFactsChecksum != nil ||
			result.RewardGrantSourceID != nil || result.RewardProvenance != nil ||
			result.SolvedAt != nil {
			return entity.SolveSelection{}, ErrInvalidSolvedStatePlan
		}
	case entity.SolveSolved:
		if current.Status == entity.SolveSolved {
			result.RewardGrantSourceID = cloneServiceUUID(
				current.RewardGrantSourceID,
			)
			result.RewardProvenance = cloneServiceRewardProvenance(
				current.RewardProvenance,
			)
		}
	default:
		return entity.SolveSelection{}, ErrInvalidSolvedStatePlan
	}
	return result, nil
}

func cloneServiceSolveSelection(
	value entity.SolveSelection,
) entity.SolveSelection {
	result := value
	result.SubmissionID = cloneServiceUUID(value.SubmissionID)
	result.JudgementID = cloneServiceUUID(value.JudgementID)
	result.Generation = cloneServiceInt64(value.Generation)
	result.ProblemRevisionID = cloneServiceUUID(value.ProblemRevisionID)
	result.RewardFacts = append(json.RawMessage(nil), value.RewardFacts...)
	result.RewardFactsChecksum = cloneServiceChecksum(
		value.RewardFactsChecksum,
	)
	result.RewardGrantSourceID = cloneServiceUUID(
		value.RewardGrantSourceID,
	)
	result.RewardProvenance = cloneServiceRewardProvenance(
		value.RewardProvenance,
	)
	result.SolvedAt = cloneServiceTime(value.SolvedAt)
	return result
}

func solveSelectionEqual(
	left entity.SolveSelection,
	right entity.SolveSelection,
) bool {
	return left.Status == right.Status &&
		serviceUUIDEqual(left.SubmissionID, right.SubmissionID) &&
		serviceUUIDEqual(left.JudgementID, right.JudgementID) &&
		serviceInt64Equal(left.Generation, right.Generation) &&
		serviceUUIDEqual(left.ProblemRevisionID, right.ProblemRevisionID) &&
		bytes.Equal(left.RewardFacts, right.RewardFacts) &&
		serviceChecksumEqual(
			left.RewardFactsChecksum,
			right.RewardFactsChecksum,
		) &&
		serviceUUIDEqual(
			left.RewardGrantSourceID,
			right.RewardGrantSourceID,
		) &&
		serviceRewardProvenanceEqual(
			left.RewardProvenance,
			right.RewardProvenance,
		) &&
		serviceTimeEqual(left.SolvedAt, right.SolvedAt)
}

func cloneServiceUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneServiceInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneServiceChecksum(value *[32]byte) *[32]byte {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneServiceRewardProvenance(
	value *entity.RewardProvenance,
) *entity.RewardProvenance {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneServiceTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func serviceUUIDEqual(left, right *uuid.UUID) bool {
	return left == nil && right == nil ||
		left != nil && right != nil && *left == *right
}

func serviceInt64Equal(left, right *int64) bool {
	return left == nil && right == nil ||
		left != nil && right != nil && *left == *right
}

func serviceChecksumEqual(left, right *[32]byte) bool {
	return left == nil && right == nil ||
		left != nil && right != nil && *left == *right
}

func serviceRewardProvenanceEqual(
	left *entity.RewardProvenance,
	right *entity.RewardProvenance,
) bool {
	return left == nil && right == nil ||
		left != nil && right != nil && *left == *right
}

func serviceTimeEqual(left, right *time.Time) bool {
	return left == nil && right == nil ||
		left != nil && right != nil && left.Equal(*right)
}
