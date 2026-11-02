package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToSaveDraftCommandParsesTransportIDs(t *testing.T) {
	problemID := uuid.New()
	difficultyID := uuid.New()
	tagID := uuid.New()
	testCaseID := uuid.New()

	command, err := toSaveDraftCommand(&SaveProblemDraftRequest{
		ID: problemID.String(), DifficultyID: difficultyID.String(), TagIDs: []string{tagID.String()},
		ExpectedVersion: 1, CatalogVersion: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Title: "Two Sum", StatementMarkdown: "Find a pair.", CPUTimeMs: 1000, WallTimeMs: 1000,
		MemoryLimitKb: 1024, OutputLimitBytes: 1024, ProcessLimit: 1, CheckerKey: "exact", CheckerVersion: "v1",
		CheckerConfig: []byte(`{}`), AllowedRuntimeKeys: []string{"go"}, Reason: "initial draft",
		Groups: []DraftTestcaseGroupRequest{{Cases: []DraftTestcaseRequest{{TestCaseID: ptr(testCaseID.String()), Kind: "sample"}}}},
	})

	require.NoError(t, err)
	require.Equal(t, problemID, command.ProblemID)
	require.Equal(t, difficultyID, command.Reward.DifficultyID)
	require.Equal(t, []uuid.UUID{tagID}, command.Reward.TagIDs)
	require.Equal(t, &testCaseID, command.Groups[0].Cases[0].TestCaseID)
}

func TestToSaveDraftCommandRejectsInvalidNestedID(t *testing.T) {
	_, err := toSaveDraftCommand(&SaveProblemDraftRequest{ID: uuid.NewString(), DifficultyID: uuid.NewString(), TagIDs: []string{"invalid"}})
	require.Error(t, err)
}

func ptr(value string) *string { return &value }
