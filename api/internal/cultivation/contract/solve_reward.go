// Package contract defines Cultivation's transport-independent interpretation
// of canonical solve transitions. Forge and outbox mechanics stay outside this
// package; every accepted value is complete enough for deterministic replay.
package contract

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

const maximumRewardFactsBytes = 64 << 10

var ErrInvalidSolveRewardTransition = errors.New("cultivation: invalid solve reward transition")

type SolveRewardTransitionKind string

const (
	SolveRewardGrant       SolveRewardTransitionKind = "grant"
	SolveRewardReversal    SolveRewardTransitionKind = "reversal"
	SolveRewardPointerMove SolveRewardTransitionKind = "pointer_move"
)

// SolveRewardState is a validated immutable side of a solve transition.
// Reward is nil exactly when Status is unsolved.
type SolveRewardState struct {
	Version             int64
	Status              string
	SubmissionID        *uuid.UUID
	JudgementID         *uuid.UUID
	Generation          *int64
	ProblemRevisionID   *uuid.UUID
	RewardGrantSourceID *uuid.UUID
	RewardProvenance    *string
	Reward              *problementity.RewardSnapshot
	SolvedAt            *time.Time
}

// SolveRewardTransition is the normalized immutable authority consumed by the
// progression projector. Digest binds every state pointer plus the checksummed
// reward facts, so stale duplicates can be distinguished from conflicts.
type SolveRewardTransition struct {
	EventID       uuid.UUID
	TransitionID  uuid.UUID
	SolveID       uuid.UUID
	UserID        uuid.UUID
	ProblemID     uuid.UUID
	Kind          SolveRewardTransitionKind
	From          SolveRewardState
	To            SolveRewardState
	Provenance    string
	OccurredAt    time.Time
	CorrelationID string
	Digest        [32]byte
}

func DecodeSolveRewardTransition(
	event submissioncontract.ProblemSolveStatusChangedV1,
) (SolveRewardTransition, error) {
	if event.EventID == uuid.Nil || event.TransitionID == uuid.Nil ||
		event.SolveID == uuid.Nil || event.UserID == uuid.Nil ||
		event.ProblemID == uuid.Nil || event.Provenance != "live" ||
		event.OccurredAt.IsZero() || correlation.Validate(event.CorrelationID) != nil {
		return SolveRewardTransition{}, ErrInvalidSolveRewardTransition
	}
	from, err := decodeSolveRewardState(event.From)
	if err != nil {
		return SolveRewardTransition{}, err
	}
	to, err := decodeSolveRewardState(event.To)
	if err != nil {
		return SolveRewardTransition{}, err
	}
	if to.Version != from.Version+1 {
		return SolveRewardTransition{}, ErrInvalidSolveRewardTransition
	}

	kind, err := transitionKind(event.TransitionID, from, to)
	if err != nil {
		return SolveRewardTransition{}, err
	}
	transition := SolveRewardTransition{
		EventID: event.EventID, TransitionID: event.TransitionID,
		SolveID: event.SolveID, UserID: event.UserID, ProblemID: event.ProblemID,
		Kind: kind, From: from, To: to, Provenance: event.Provenance,
		OccurredAt: event.OccurredAt.UTC(), CorrelationID: event.CorrelationID,
	}
	digest, err := transitionDigest(transition)
	if err != nil || digest == ([32]byte{}) {
		return SolveRewardTransition{}, ErrInvalidSolveRewardTransition
	}
	transition.Digest = digest
	return transition, nil
}

func decodeSolveRewardState(wire submissioncontract.SolveStateV1) (SolveRewardState, error) {
	if wire.Version < 0 || (wire.Status != "solved" && wire.Status != "unsolved") {
		return SolveRewardState{}, ErrInvalidSolveRewardTransition
	}
	state := SolveRewardState{
		Version: wire.Version, Status: wire.Status,
		SubmissionID: cloneUUID(wire.SubmissionID), JudgementID: cloneUUID(wire.JudgementID),
		Generation: cloneInt64(wire.Generation), ProblemRevisionID: cloneUUID(wire.ProblemRevisionID),
		RewardGrantSourceID: cloneUUID(wire.RewardGrantSourceID),
		RewardProvenance:    cloneString(wire.RewardProvenance), SolvedAt: cloneTime(wire.SolvedAt),
	}
	if wire.Status == "unsolved" {
		if wire.SubmissionID != nil || wire.JudgementID != nil || wire.Generation != nil ||
			wire.ProblemRevisionID != nil || wire.RewardGrantSourceID != nil ||
			wire.RewardProvenance != nil || !emptyRewardFacts(wire.RewardFacts) ||
			wire.RewardFactsChecksum != nil || wire.SolvedAt != nil {
			return SolveRewardState{}, ErrInvalidSolveRewardTransition
		}
		return state, nil
	}
	if nilOrZeroUUID(wire.SubmissionID) || nilOrZeroUUID(wire.JudgementID) ||
		wire.Generation == nil || *wire.Generation < 1 ||
		nilOrZeroUUID(wire.ProblemRevisionID) || nilOrZeroUUID(wire.RewardGrantSourceID) ||
		wire.RewardProvenance == nil || *wire.RewardProvenance != "live" ||
		wire.RewardFactsChecksum == nil || wire.SolvedAt == nil || wire.SolvedAt.IsZero() {
		return SolveRewardState{}, ErrInvalidSolveRewardTransition
	}
	reward, err := decodeImmutableRewardFacts(wire.RewardFacts, *wire.RewardFactsChecksum)
	if err != nil {
		return SolveRewardState{}, err
	}
	state.Reward = &reward
	solvedAt := wire.SolvedAt.UTC()
	state.SolvedAt = &solvedAt
	return state, nil
}

func transitionKind(
	transitionID uuid.UUID,
	from SolveRewardState,
	to SolveRewardState,
) (SolveRewardTransitionKind, error) {
	switch {
	case from.Status == "unsolved" && to.Status == "solved":
		if to.RewardGrantSourceID == nil || *to.RewardGrantSourceID != transitionID {
			return "", ErrInvalidSolveRewardTransition
		}
		return SolveRewardGrant, nil
	case from.Status == "solved" && to.Status == "unsolved":
		return SolveRewardReversal, nil
	case from.Status == "solved" && to.Status == "solved":
		if from.RewardGrantSourceID == nil || to.RewardGrantSourceID == nil ||
			*from.RewardGrantSourceID != *to.RewardGrantSourceID ||
			from.RewardProvenance == nil || to.RewardProvenance == nil ||
			*from.RewardProvenance != *to.RewardProvenance ||
			sameSolveRewardSelection(from, to) {
			return "", ErrInvalidSolveRewardTransition
		}
		return SolveRewardPointerMove, nil
	default:
		return "", ErrInvalidSolveRewardTransition
	}
}

func sameSolveRewardSelection(left, right SolveRewardState) bool {
	if left.Reward == nil || right.Reward == nil ||
		left.SubmissionID == nil || right.SubmissionID == nil ||
		left.JudgementID == nil || right.JudgementID == nil ||
		left.Generation == nil || right.Generation == nil ||
		left.ProblemRevisionID == nil || right.ProblemRevisionID == nil ||
		left.RewardGrantSourceID == nil || right.RewardGrantSourceID == nil ||
		left.RewardProvenance == nil || right.RewardProvenance == nil ||
		left.SolvedAt == nil || right.SolvedAt == nil {
		return false
	}
	return *left.SubmissionID == *right.SubmissionID &&
		*left.JudgementID == *right.JudgementID &&
		*left.Generation == *right.Generation &&
		*left.ProblemRevisionID == *right.ProblemRevisionID &&
		*left.RewardGrantSourceID == *right.RewardGrantSourceID &&
		*left.RewardProvenance == *right.RewardProvenance &&
		left.Reward.Checksum == right.Reward.Checksum &&
		left.SolvedAt.Equal(*right.SolvedAt)
}

func decodeImmutableRewardFacts(
	raw stdjson.RawMessage,
	checksumHex string,
) (problementity.RewardSnapshot, error) {
	if len(raw) == 0 || len(raw) > maximumRewardFactsBytes || checksumHex == "" {
		return problementity.RewardSnapshot{}, ErrInvalidSolveRewardTransition
	}
	var wire struct {
		DifficultyID       uuid.UUID                             `json:"difficulty_id"`
		DifficultyRevision int64                                 `json:"difficulty_revision"`
		BaseExperience     int64                                 `json:"base_exp"`
		Tags               problementity.TagMappingSnapshot      `json:"tags"`
		Elements           *problementity.ElementMappingSnapshot `json:"elements"`
	}
	if err := commonjson.UnmarshalStrictLimit(raw, &wire, maximumRewardFactsBytes); err != nil {
		return problementity.RewardSnapshot{}, fmt.Errorf(
			"%w: decode immutable reward facts: %v",
			ErrInvalidSolveRewardTransition,
			err,
		)
	}
	checksum, err := decodeChecksum(checksumHex)
	if err != nil {
		return problementity.RewardSnapshot{}, err
	}
	reward := problementity.RewardSnapshot{
		DifficultyID: wire.DifficultyID, DifficultyRevision: wire.DifficultyRevision,
		BaseExperience: wire.BaseExperience, Tags: wire.Tags,
		Elements: wire.Elements, Checksum: checksum,
	}
	if reward.Validate() != nil {
		return problementity.RewardSnapshot{}, ErrInvalidSolveRewardTransition
	}
	return reward, nil
}

func transitionDigest(value SolveRewardTransition) ([32]byte, error) {
	type stateDocument struct {
		Version             int64      `json:"version"`
		Status              string     `json:"status"`
		SubmissionID        *uuid.UUID `json:"submission_id"`
		JudgementID         *uuid.UUID `json:"judgement_id"`
		Generation          *int64     `json:"generation"`
		ProblemRevisionID   *uuid.UUID `json:"problem_revision_id"`
		RewardGrantSourceID *uuid.UUID `json:"reward_grant_source_id"`
		RewardProvenance    *string    `json:"reward_provenance"`
		RewardChecksum      string     `json:"reward_checksum"`
		SolvedAt            *time.Time `json:"solved_at"`
	}
	state := func(input SolveRewardState) stateDocument {
		checksum := ""
		if input.Reward != nil {
			checksum = hexChecksum(input.Reward.Checksum)
		}
		return stateDocument{
			Version: input.Version, Status: input.Status,
			SubmissionID: input.SubmissionID, JudgementID: input.JudgementID,
			Generation: input.Generation, ProblemRevisionID: input.ProblemRevisionID,
			RewardGrantSourceID: input.RewardGrantSourceID,
			RewardProvenance:    input.RewardProvenance, RewardChecksum: checksum,
			SolvedAt: input.SolvedAt,
		}
	}
	document := struct {
		Domain        string                    `json:"domain"`
		EventID       uuid.UUID                 `json:"event_id"`
		TransitionID  uuid.UUID                 `json:"transition_id"`
		SolveID       uuid.UUID                 `json:"solve_id"`
		UserID        uuid.UUID                 `json:"user_id"`
		ProblemID     uuid.UUID                 `json:"problem_id"`
		Kind          SolveRewardTransitionKind `json:"kind"`
		From          stateDocument             `json:"from"`
		To            stateDocument             `json:"to"`
		Provenance    string                    `json:"provenance"`
		OccurredAt    time.Time                 `json:"occurred_at"`
		CorrelationID string                    `json:"correlation_id"`
	}{
		Domain:  "judgify.cultivation.solve-reward-transition.v1",
		EventID: value.EventID, TransitionID: value.TransitionID,
		SolveID: value.SolveID, UserID: value.UserID, ProblemID: value.ProblemID,
		Kind: value.Kind, From: state(value.From), To: state(value.To),
		Provenance: value.Provenance, OccurredAt: value.OccurredAt.UTC(),
		CorrelationID: value.CorrelationID,
	}
	encoded, err := stdjson.Marshal(document)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func decodeChecksum(value string) ([32]byte, error) {
	var checksum [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(checksum) {
		return checksum, ErrInvalidSolveRewardTransition
	}
	copy(checksum[:], decoded)
	if subtle.ConstantTimeCompare(checksum[:], make([]byte, len(checksum))) == 1 {
		return [32]byte{}, ErrInvalidSolveRewardTransition
	}
	return checksum, nil
}

func emptyRewardFacts(value stdjson.RawMessage) bool {
	return len(value) == 0 || string(value) == "null"
}

func nilOrZeroUUID(value *uuid.UUID) bool { return value == nil || *value == uuid.Nil }

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func hexChecksum(value [32]byte) string { return hex.EncodeToString(value[:]) }
