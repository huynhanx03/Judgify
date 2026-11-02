package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStateMachineAcceptsOnlyClosedSubmissionPhaseEdges(t *testing.T) {
	base := SubmissionState{
		ID: uuid.New(), LatestGeneration: 1,
		LatestPhase: PhaseQueued, AggregateVersion: 0,
	}
	claimed, err := TransitionPhase(base, 1, PhaseClaimed)
	if err != nil || claimed.LatestPhase != PhaseClaimed || claimed.AggregateVersion != 1 {
		t.Fatalf("queued to claimed = (%+v, %v)", claimed, err)
	}
	compiling, err := TransitionPhase(claimed, 1, PhaseCompiling)
	if err != nil {
		t.Fatalf("claimed to compiling: %v", err)
	}
	finalizing, err := TransitionPhase(compiling, 1, PhaseFinalizing)
	if err != nil {
		t.Fatalf("compile-error path to finalizing: %v", err)
	}
	if _, err := TransitionPhase(finalizing, 1, PhaseRunning); err == nil {
		t.Fatal("finalizing to running was accepted")
	}
	if _, err := TransitionPhase(base, 2, PhaseClaimed); err != ErrStaleGeneration {
		t.Fatalf("stale generation error = %v", err)
	}
}

func TestStateRejudgePreservesActiveJudgementThroughSystemErrorAndCancellation(t *testing.T) {
	submissionID := uuid.New()
	activeID := uuid.New()
	activeGeneration := int64(1)
	activeVerdict := VerdictAccepted
	current := SubmissionState{
		ID: submissionID, LatestGeneration: 1,
		LatestPhase: PhaseTerminal, LatestOutcome: string(VerdictAccepted),
		ActiveGeneration: &activeGeneration, ActiveJudgementID: &activeID,
		ActiveVerdict: &activeVerdict, AggregateVersion: 7,
	}
	rejudge, err := StartGeneration(current, 2)
	if err != nil {
		t.Fatalf("StartGeneration: %v", err)
	}
	systemJudgement := validDomainJudgement(submissionID, 2, VerdictSystemError)
	failed, err := FinishGeneration(rejudge, GenerationResult{
		Generation: 2, Outcome: string(VerdictSystemError), Judgement: &systemJudgement,
	})
	if err != nil {
		t.Fatalf("FinishGeneration system error: %v", err)
	}
	assertActiveJudgement(t, failed, activeGeneration, activeID, activeVerdict)
	if failed.LatestGeneration != 2 || failed.LatestOutcome != string(VerdictSystemError) {
		t.Fatalf("failed latest state = %+v", failed)
	}

	third, err := StartGeneration(failed, 3)
	if err != nil {
		t.Fatalf("StartGeneration after failed rejudge: %v", err)
	}
	cancelled, err := FinishGeneration(third, GenerationResult{Generation: 3, Outcome: "cancelled"})
	if err != nil {
		t.Fatalf("FinishGeneration cancellation: %v", err)
	}
	assertActiveJudgement(t, cancelled, activeGeneration, activeID, activeVerdict)
}

func TestStateAppliesOnlyBoundContestantJudgement(t *testing.T) {
	submissionID := uuid.New()
	state := SubmissionState{
		ID: submissionID, LatestGeneration: 1,
		LatestPhase: PhaseFinalizing, AggregateVersion: 3,
	}
	judgement := validDomainJudgement(submissionID, 1, VerdictWrongAnswer)
	next, err := ApplyJudgement(state, judgement)
	if err != nil {
		t.Fatalf("ApplyJudgement: %v", err)
	}
	assertActiveJudgement(t, next, 1, judgement.ID, VerdictWrongAnswer)
	if next.LatestPhase != PhaseTerminal || next.LatestOutcome != string(VerdictWrongAnswer) {
		t.Fatalf("latest state = %+v", next)
	}

	system := validDomainJudgement(submissionID, 1, VerdictSystemError)
	if _, err := ApplyJudgement(state, system); err == nil {
		t.Fatal("system-error judgement became active")
	}
	foreign := validDomainJudgement(uuid.New(), 1, VerdictSystemError)
	if _, err := FinishGeneration(state, GenerationResult{
		Generation: 1, Outcome: string(VerdictSystemError), Judgement: &foreign,
	}); err == nil {
		t.Fatal("foreign system-error judgement closed the submission")
	}
}

func TestStateJudgementValidationRejectsMissingImmutableProvenance(t *testing.T) {
	valid := validDomainJudgement(uuid.New(), 1, VerdictWrongAnswer)
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid judgement: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Judgement)
	}{
		{name: "spec checksum", mutate: func(value *Judgement) { value.JudgeSpecChecksum = [32]byte{} }},
		{name: "reward checksum", mutate: func(value *Judgement) { value.RewardSnapshotChecksum = [32]byte{} }},
		{name: "reward snapshot", mutate: func(value *Judgement) { value.RewardSnapshot = nil }},
		{name: "runtime provenance", mutate: func(value *Judgement) { value.RuntimeProfileVersion = "" }},
		{name: "image digest", mutate: func(value *Judgement) { value.SandboxImageDigest = "sha256:bad" }},
		{name: "manifest digest", mutate: func(value *Judgement) { value.ArtifactManifestChecksum = "" }},
		{name: "completion time", mutate: func(value *Judgement) { value.CompletedAt = time.Time{} }},
		{name: "correlation", mutate: func(value *Judgement) { value.OriginCID = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid judgement was accepted")
			}
		})
	}
}

func TestStateJobAndAttemptTransitionsAreClosed(t *testing.T) {
	jobEdges := [][2]JobState{
		{JobReady, JobLeased}, {JobLeased, JobDone},
		{JobLeased, JobRetryWait}, {JobRetryWait, JobReady},
		{JobLeased, JobDead}, {JobReady, JobCancelled},
	}
	for _, edge := range jobEdges {
		if _, err := TransitionJobState(edge[0], edge[1]); err != nil {
			t.Errorf("job transition %s to %s: %v", edge[0], edge[1], err)
		}
	}
	if _, err := TransitionJobState(JobDone, JobReady); err == nil {
		t.Fatal("terminal job reopened")
	}

	attemptEdges := [][2]AttemptState{
		{AttemptCreated, AttemptPreparing},
		{AttemptPreparing, AttemptCompiling},
		{AttemptCompiling, AttemptRunning},
		{AttemptCompiling, AttemptFinalizing},
		{AttemptRunning, AttemptFinalizing},
		{AttemptFinalizing, AttemptSucceeded},
		{AttemptRunning, AttemptInfrastructureFailed},
	}
	for _, edge := range attemptEdges {
		if _, err := TransitionAttemptState(edge[0], edge[1]); err != nil {
			t.Errorf("attempt transition %s to %s: %v", edge[0], edge[1], err)
		}
	}
	if _, err := TransitionAttemptState(AttemptSucceeded, AttemptRunning); err == nil {
		t.Fatal("terminal attempt reopened")
	}
}

func TestStateSolveTransitionsAreContiguousAndPreserveGrantSource(t *testing.T) {
	solve, err := NewUnsolvedSolve(uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("NewUnsolvedSolve: %v", err)
	}
	firstTransitionID := uuid.New()
	firstSelection := validSolvedSelection()
	first, transition, err := TransitionSolve(solve, firstSelection, SolveTransitionMetadata{
		ID: firstTransitionID, Provenance: SolveTransitionLive,
		OccurredAt: time.Now().UTC(), CorrelationID: "cid-first",
	})
	if err != nil {
		t.Fatalf("first TransitionSolve: %v", err)
	}
	if transition.FromVersion != 0 || transition.ToVersion != 1 || first.Version != 1 {
		t.Fatalf("first versions = %d/%d projection=%d", transition.FromVersion, transition.ToVersion, first.Version)
	}
	if *first.Selection.RewardGrantSourceID != firstTransitionID ||
		*first.ActiveRewardTransitionID != firstTransitionID {
		t.Fatal("first solve did not use transition identity for grant and active pointer")
	}

	replacement := validSolvedSelection()
	replacement.RewardGrantSourceID = nil
	replacement.RewardProvenance = nil
	secondTransitionID := uuid.New()
	second, moved, err := TransitionSolve(first, replacement, SolveTransitionMetadata{
		ID: secondTransitionID, Provenance: SolveTransitionLive,
		OccurredAt: time.Now().UTC().Add(time.Second), CorrelationID: "cid-second",
	})
	if err != nil {
		t.Fatalf("solved-to-solved TransitionSolve: %v", err)
	}
	if moved.FromVersion != 1 || moved.ToVersion != 2 || second.Version != 2 {
		t.Fatalf("second versions = %d/%d projection=%d", moved.FromVersion, moved.ToVersion, second.Version)
	}
	if *second.Selection.RewardGrantSourceID != firstTransitionID ||
		*second.ActiveRewardTransitionID != secondTransitionID {
		t.Fatal("solved-to-solved transition changed the original grant or stale active transition")
	}
	callerJudgementID := *replacement.JudgementID
	*replacement.JudgementID = uuid.New()
	if *moved.To.JudgementID != callerJudgementID || *second.Selection.JudgementID != callerJudgementID {
		t.Fatal("returned solve state aliases caller-owned selection pointers")
	}

	unsolved, reversal, err := TransitionSolve(second, SolveSelection{Status: SolveUnsolved}, SolveTransitionMetadata{
		ID: uuid.New(), Provenance: SolveTransitionLive,
		OccurredAt: time.Now().UTC().Add(2 * time.Second), CorrelationID: "cid-third",
	})
	if err != nil {
		t.Fatalf("solved-to-unsolved TransitionSolve: %v", err)
	}
	if unsolved.Version != 3 || reversal.FromVersion != 2 || reversal.ToVersion != 3 ||
		unsolved.ActiveRewardTransitionID != nil {
		t.Fatalf("reversal = %+v projection=%+v", reversal, unsolved)
	}
}

func validDomainJudgement(submissionID uuid.UUID, generation int64, verdict Verdict) Judgement {
	checksum := [32]byte{1}
	judgement := Judgement{
		ID: uuid.New(), SubmissionID: submissionID, Generation: generation,
		JudgeSpecID: uuid.New(), ProblemRevisionID: uuid.New(), TestsetRevisionID: uuid.New(),
		Verdict: verdict, PassedCount: 0, TotalCount: 1,
		JudgeSpecChecksum: checksum, RewardSnapshot: json.RawMessage(`{"checksum":"snapshot"}`),
		RewardSnapshotChecksum: checksum, RuntimeProfileVersion: "v1", CompilerVersion: "v1",
		RuntimeBootstrapVersion: "v1", SupervisorProtocolVersion: "v1",
		SandboxImageDigest:              "sha256:" + strings.Repeat("a", 64),
		ArtifactManifestChecksum:        "sha256:" + strings.Repeat("b", 64),
		ReleaseID:                       "judge-release.test",
		ApprovedReleaseManifestChecksum: "sha256:" + strings.Repeat("c", 64),
		CheckerImplementationChecksum:   "sha256:" + strings.Repeat("d", 64),
		CompletedAt:                     time.Now().UTC(), OriginCID: "cid-test",
	}
	if verdict.Contestant() {
		attemptID := uuid.New()
		judgement.WinningAttemptID = &attemptID
	}
	if verdict == VerdictAccepted {
		judgement.PassedCount = 1
	}
	return judgement
}

func validSolvedSelection() SolveSelection {
	submissionID := uuid.New()
	judgementID := uuid.New()
	generation := int64(1)
	problemRevisionID := uuid.New()
	solvedAt := time.Now().UTC()
	checksum := [32]byte{1}
	return SolveSelection{
		Status: SolveSolved, SubmissionID: &submissionID, JudgementID: &judgementID,
		Generation: &generation, ProblemRevisionID: &problemRevisionID,
		RewardFacts:         json.RawMessage(`{"checksum":"reward"}`),
		RewardFactsChecksum: &checksum,
		RewardGrantSourceID: nil, RewardProvenance: nil, SolvedAt: &solvedAt,
	}
}

func assertActiveJudgement(
	t *testing.T,
	state SubmissionState,
	generation int64,
	judgementID uuid.UUID,
	verdict Verdict,
) {
	t.Helper()
	if state.ActiveGeneration == nil || *state.ActiveGeneration != generation ||
		state.ActiveJudgementID == nil || *state.ActiveJudgementID != judgementID ||
		state.ActiveVerdict == nil || *state.ActiveVerdict != verdict {
		t.Fatalf("active judgement = %+v", state)
	}
}
