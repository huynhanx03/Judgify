package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToRejudgeCommandMapsHTTPPayload(t *testing.T) {
	submissionID := uuid.New()
	key := uuid.New()
	command := toRejudgeCommand(&rejudgeSubmissionRequest{ExpectedVersion: 4, Reason: "refresh judge"}, submissionID, key)

	require.Equal(t, submissionID, command.SubmissionID)
	require.Equal(t, int64(4), command.ExpectedVersion)
	require.Equal(t, key, command.IdempotencyKey)
}
