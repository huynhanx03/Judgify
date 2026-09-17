package contract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func TestDecodeSolveRewardTransitionAcceptsCanonicalGrant(t *testing.T) {
	event := validSolveRewardEvent(t)
	transition, err := DecodeSolveRewardTransition(event)
	if err != nil {
		t.Fatalf("DecodeSolveRewardTransition() error = %v", err)
	}
	if transition.Kind != SolveRewardGrant || transition.TransitionID != event.TransitionID ||
		transition.UserID != event.UserID || transition.To.Reward == nil ||
		transition.To.Reward.BaseExperience != 100 || transition.Digest == ([32]byte{}) {
		t.Fatalf("DecodeSolveRewardTransition() = %#v", transition)
	}
	if transition.From.Reward != nil {
		t.Fatalf("unsolved source has reward = %#v", transition.From.Reward)
	}
	if transition.To.RewardGrantSourceID == nil ||
		*transition.To.RewardGrantSourceID != event.TransitionID {
		t.Fatalf("grant source = %v, want transition", transition.To.RewardGrantSourceID)
	}
}

func TestDecodeSolveRewardTransitionAcceptsReversalAndPointerMove(t *testing.T) {
	grantEvent := validSolveRewardEvent(t)

	reversal := grantEvent
	reversal.EventID = uuid.New()
	reversal.TransitionID = uuid.New()
	reversal.From = grantEvent.To
	reversal.To = submissioncontract.SolveStateV1{Version: 2, Status: "unsolved"}
	decoded, err := DecodeSolveRewardTransition(reversal)
	if err != nil || decoded.Kind != SolveRewardReversal || decoded.From.Reward == nil ||
		decoded.To.Reward != nil {
		t.Fatalf("decode reversal = %#v, %v", decoded, err)
	}

	pointerMove := grantEvent
	pointerMove.EventID = uuid.New()
	pointerMove.TransitionID = uuid.New()
	pointerMove.From = grantEvent.To
	pointerMove.To = grantEvent.To
	pointerMove.To.Version = 2
	pointerMove.To.SubmissionID = uuidPointer(uuid.New())
	pointerMove.To.JudgementID = uuidPointer(uuid.New())
	pointerMove.To.Generation = int64Pointer(2)
	pointerMove.To.ProblemRevisionID = uuidPointer(uuid.New())
	decoded, err = DecodeSolveRewardTransition(pointerMove)
	if err != nil || decoded.Kind != SolveRewardPointerMove || decoded.From.Reward == nil ||
		decoded.To.Reward == nil || decoded.From.RewardGrantSourceID == nil ||
		decoded.To.RewardGrantSourceID == nil ||
		*decoded.From.RewardGrantSourceID != *decoded.To.RewardGrantSourceID {
		t.Fatalf("decode pointer move = %#v, %v", decoded, err)
	}
}

func TestDecodeSolveRewardTransitionRejectsConflictingProvenance(t *testing.T) {
	event := validSolveRewardEvent(t)

	event.To.RewardGrantSourceID = uuidPointer(uuid.New())
	if _, err := DecodeSolveRewardTransition(event); err == nil {
		t.Fatal("DecodeSolveRewardTransition() accepted a conflicting grant source")
	}

	event = validSolveRewardEvent(t)
	legacy := "legacy_reconstructed"
	event.To.RewardProvenance = &legacy
	if _, err := DecodeSolveRewardTransition(event); err == nil {
		t.Fatal("DecodeSolveRewardTransition() accepted reconstructed provenance")
	}

	event = validSolveRewardEvent(t)
	event.To.Version = event.From.Version + 2
	if _, err := DecodeSolveRewardTransition(event); err == nil {
		t.Fatal("DecodeSolveRewardTransition() accepted a version gap inside one event")
	}

	event = validSolveRewardEvent(t)
	event.EventID = uuid.New()
	event.TransitionID = uuid.New()
	event.From = event.To
	event.To = event.From
	event.To.Version++
	if _, err := DecodeSolveRewardTransition(event); err == nil {
		t.Fatal("DecodeSolveRewardTransition() accepted a no-op solved pointer move")
	}
}

func validSolveRewardEvent(t *testing.T) submissioncontract.ProblemSolveStatusChangedV1 {
	t.Helper()
	grantID := uuid.New()
	reward := problementity.RewardSnapshot{
		DifficultyID: uuid.New(), DifficultyRevision: 1, BaseExperience: 100,
		Tags: problementity.TagMappingSnapshot{
			MappingID: uuid.New(), Revision: 1,
			Items: []problementity.TagSnapshotItem{{TagID: uuid.New(), TagRevision: 1}},
		},
	}
	checksum, err := reward.ComputeChecksum()
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}
	reward.Checksum = checksum
	facts, err := json.Marshal(struct {
		DifficultyID       uuid.UUID                             `json:"difficulty_id"`
		DifficultyRevision int64                                 `json:"difficulty_revision"`
		BaseExperience     int64                                 `json:"base_exp"`
		Tags               problementity.TagMappingSnapshot      `json:"tags"`
		Elements           *problementity.ElementMappingSnapshot `json:"elements"`
	}{
		DifficultyID: reward.DifficultyID, DifficultyRevision: reward.DifficultyRevision,
		BaseExperience: reward.BaseExperience, Tags: reward.Tags, Elements: reward.Elements,
	})
	if err != nil {
		t.Fatalf("marshal reward facts: %v", err)
	}
	provenance := "live"
	now := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)
	submissionID, judgementID, problemRevisionID := uuid.New(), uuid.New(), uuid.New()
	generation := int64(1)
	return submissioncontract.ProblemSolveStatusChangedV1{
		EventID: uuid.New(), TransitionID: grantID, SolveID: uuid.New(),
		UserID: uuid.New(), ProblemID: uuid.New(), Provenance: "live",
		OccurredAt: now, CorrelationID: "cid-reward-grant",
		From: submissioncontract.SolveStateV1{Version: 0, Status: "unsolved"},
		To: submissioncontract.SolveStateV1{
			Version: 1, Status: "solved", SubmissionID: &submissionID,
			JudgementID: &judgementID, Generation: &generation,
			ProblemRevisionID: &problemRevisionID, RewardGrantSourceID: &grantID,
			RewardProvenance: &provenance, RewardFacts: facts,
			RewardFactsChecksum: stringPointer(hexChecksum(checksum)), SolvedAt: &now,
		},
	}
}

func uuidPointer(value uuid.UUID) *uuid.UUID { return &value }
func int64Pointer(value int64) *int64        { return &value }
func stringPointer(value string) *string     { return &value }
