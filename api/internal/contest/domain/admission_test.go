package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestContestStateUsesClosedTransitionGraph(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateContestTransition(ContestDraft, ContestUpcoming))
	require.NoError(t, ValidateContestTransition(ContestUpcoming, ContestRunning))
	require.NoError(t, ValidateContestTransition(ContestRunning, ContestEnded))
	for _, state := range []ContestState{ContestDraft, ContestUpcoming, ContestRunning} {
		require.NoError(t, ValidateContestTransition(state, ContestCancelled))
	}

	for _, transition := range [][2]ContestState{
		{ContestDraft, ContestRunning},
		{ContestUpcoming, ContestEnded},
		{ContestEnded, ContestRunning},
		{ContestCancelled, ContestUpcoming},
		{ContestRunning, ContestUpcoming},
	} {
		require.ErrorIs(t, ValidateContestTransition(transition[0], transition[1]), ErrContestTransition)
	}
}

func TestContestOfficialWindowIsStartInclusiveAndEndExclusive(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 28, 2, 0, 0, 0, time.UTC)
	contest := Contest{State: ContestRunning, StartTime: start, EndTime: start.Add(time.Hour)}
	require.True(t, contest.AcceptsOfficialSubmissionAt(start))
	require.True(t, contest.AcceptsOfficialSubmissionAt(start.Add(time.Nanosecond)))
	require.False(t, contest.AcceptsOfficialSubmissionAt(start.Add(time.Hour)))
	require.False(t, contest.AcceptsOfficialSubmissionAt(start.Add(-time.Nanosecond)))
}

func TestContestRegistrationStateNeverReactivatesBan(t *testing.T) {
	t.Parallel()

	registration := ContestRegistration{
		ID: uuid.New(), ContestID: uuid.New(), UserID: uuid.New(),
		Status: RegistrationBanned, Source: RegistrationSourceAdmin, Version: 3,
	}
	require.ErrorIs(t, registration.CanTransitionTo(RegistrationRegistered), ErrContestRegistrationBanned)
	require.ErrorIs(t, registration.CanTransitionTo(RegistrationWithdrawn), ErrContestRegistrationBanned)
}

func TestContestProblemSnapshotRejectsIncompleteIdentity(t *testing.T) {
	t.Parallel()

	problem := ContestProblem{
		ID: uuid.New(), ContestID: uuid.New(), ProblemID: uuid.New(),
		ProblemRevisionID: uuid.New(), TestsetRevisionID: uuid.New(),
		Alias: "A", DisplayOrder: 1, CheckerKey: "exact", CheckerVersion: "v1",
		AllowedRuntimeKeys: []string{"cpp17"}, SnapshotChecksum: [32]byte{1},
	}
	require.NoError(t, problem.Validate())
	problem.TestsetRevisionID = uuid.Nil
	require.ErrorIs(t, problem.Validate(), ErrContestProblemSnapshot)
}
