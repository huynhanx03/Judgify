package db

import (
	"testing"
	"time"

	"github.com/google/uuid"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestValidateImmutableResultPlanRequiresStopOnFailureForOmittedCases(t *testing.T) {
	groupID := uuid.New()
	expected := []expectedTestcaseResult{
		{testcaseRevisionID: uuid.New(), groupID: groupID, groupIndex: 0, caseOrder: 0, kind: problemcontract.TestKindHidden, stopOnFailure: true},
		{testcaseRevisionID: uuid.New(), groupID: groupID, groupIndex: 0, caseOrder: 1, kind: problemcontract.TestKindHidden, stopOnFailure: true},
	}
	results := []submissionports.JudgeTestResult{{
		TestcaseRevisionID: expected[0].testcaseRevisionID,
		GroupID:            groupID, GroupIndex: 0, CaseOrder: 0,
		ExecutionOrder: 0, Kind: problemcontract.TestKindHidden,
		StopOnFailure: true, Verdict: submissionentity.VerdictWrongAnswer,
		ExecutionStartedAt: time.Now().UTC(), ExecutionEndedAt: time.Now().UTC(),
	}}

	if err := validateImmutableResultPlan(expected, results); err != nil {
		t.Fatalf("validateImmutableResultPlan() error = %v", err)
	}

	results[0].StopOnFailure = false
	if err := validateImmutableResultPlan(expected, results); err == nil {
		t.Fatal("validateImmutableResultPlan() accepted an unjustified omission")
	}
}

func TestValidateImmutableResultPlanRejectsForgedMembership(t *testing.T) {
	expected := []expectedTestcaseResult{{
		testcaseRevisionID: uuid.New(), groupID: uuid.New(), groupIndex: 0,
		caseOrder: 0, kind: problemcontract.TestKindHidden,
	}}
	results := []submissionports.JudgeTestResult{{
		TestcaseRevisionID: uuid.New(),
		GroupID:            expected[0].groupID, GroupIndex: 0, CaseOrder: 0,
		ExecutionOrder: 0, Kind: problemcontract.TestKindHidden,
		StopOnFailure: false, Verdict: submissionentity.VerdictAccepted,
	}}
	if err := validateImmutableResultPlan(expected, results); err == nil {
		t.Fatal("validateImmutableResultPlan() accepted a testcase outside judge spec")
	}
}

func TestLegacyStatusForVerdictPreservesProcessLimit(t *testing.T) {
	if got := submissionStatusForVerdict(
		submissionentity.VerdictProcessLimitExceeded,
	); got != string(submissionentity.VerdictProcessLimitExceeded) {
		t.Fatalf("submissionStatusForVerdict() = %q", got)
	}
}
