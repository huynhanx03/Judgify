package realtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
	"github.com/stretchr/testify/require"

	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func TestMessagesForSubmissionOutboxAreVersionedMinimalAndRouted(t *testing.T) {
	event := validSubmissionOutboxEvent()
	messages, err := MessagesForSubmissionOutbox(event)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, EventSubmissionJudgedV1, messages[0].Message.Envelope.Type)
	require.Equal(t, EventContestStandingsChangedV1, messages[1].Message.Envelope.Type)
	for _, routed := range messages {
		require.Equal(t, routed.Topic, routed.Message.Envelope.Topic)
		require.Equal(t, commonwebsocket.ProtocolVersion, routed.Message.Envelope.Version)
		require.Equal(t, commonwebsocket.OperationEvent, routed.Message.Envelope.Operation)
		require.NoError(t, routed.Message.Validate())
	}
	require.Equal(t, commonwebsocket.DeliveryCritical, messages[0].Message.Class)
	require.Empty(t, messages[0].Message.CoalesceKey)
	require.Equal(t, commonwebsocket.DeliveryLatestState, messages[1].Message.Class)
	require.Equal(
		t,
		"contest-standings:"+event.Contest.ContestID.String(),
		messages[1].Message.CoalesceKey,
	)

	encoded, err := json.Marshal(messages)
	require.NoError(t, err)
	wire := string(encoded)
	for _, forbidden := range []string{
		"source_code", "testcase", "test_case", "worker", "lease",
		"maximum_time_ms", "maximum_memory_kb", "problem_revision_id",
	} {
		require.False(t, strings.Contains(wire, forbidden), forbidden)
	}
}

func TestMessagesForNonContestSubmissionHavePrivateEventOnly(t *testing.T) {
	event := validSubmissionOutboxEvent()
	event.Contest = nil
	messages, err := MessagesForSubmissionOutbox(event)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, EventSubmissionJudgedV1, messages[0].Message.Envelope.Type)
}

func TestSubmissionOutboxProjectionValidationFailsClosed(t *testing.T) {
	tests := []func(*submissioncontract.SubmissionJudgedV1){
		func(event *submissioncontract.SubmissionJudgedV1) { event.EventID = uuid.Nil },
		func(event *submissioncontract.SubmissionJudgedV1) { event.Verdict = "judging" },
		func(event *submissioncontract.SubmissionJudgedV1) { event.PassedCount = event.TotalCount + 1 },
		func(event *submissioncontract.SubmissionJudgedV1) { event.TotalCount = 0; event.PassedCount = 0 },
		func(event *submissioncontract.SubmissionJudgedV1) { event.AggregateVersion = 0 },
		func(event *submissioncontract.SubmissionJudgedV1) {
			event.OccurredAt = event.SubmittedAt.Add(-time.Second)
		},
		func(event *submissioncontract.SubmissionJudgedV1) {
			event.Contest.ContestID = uuid.Nil
		},
	}
	for _, mutate := range tests {
		event := validSubmissionOutboxEvent()
		mutate(&event)
		_, err := SubmissionProjectionFromOutbox(event)
		require.ErrorIs(t, err, ErrInvalidChange)
	}
}

func TestAuthorizationMessageIsMinimalRevisionInvalidation(t *testing.T) {
	change := authorizationChange(14)
	routed, err := MessageForAuthorizationChange(change)
	require.NoError(t, err)
	require.Equal(t, AuthorizationCapabilitiesTopic(), routed.Topic)
	require.Equal(t, EventAuthorizationCapabilitiesChangedV1, routed.Message.Envelope.Type)
	require.Equal(t, int64(14), routed.Message.Envelope.AggregateVersion)
	require.Equal(t, "14", routed.Message.Envelope.Cursor)
	require.Equal(t, commonwebsocket.DeliveryLatestState, routed.Message.Class)
	require.Equal(t, AuthorizationCapabilitiesTopic(), routed.Message.CoalesceKey)
	require.NoError(t, routed.Message.Validate())

	encoded, err := json.Marshal(routed)
	require.NoError(t, err)
	for _, forbidden := range []string{"user_id", "role", "policy", "capabilities"} {
		if forbidden == "capabilities" {
			var payload map[string]any
			require.NoError(t, json.Unmarshal(routed.Message.Envelope.Data, &payload))
			require.NotContains(t, payload, forbidden)
			continue
		}
		require.NotContains(t, string(encoded), forbidden)
	}
}

func TestAuthorizationChangeValidationFailsClosed(t *testing.T) {
	change := authorizationChange(1)
	change.Event.Revision = 0
	require.ErrorIs(t, change.Validate(), ErrInvalidChange)

	change = authorizationChange(1)
	change.Event.ChangedAt = change.Event.ChangedAt.In(time.FixedZone("offset", 3600))
	require.ErrorIs(t, change.Validate(), ErrInvalidChange)
}

func validSubmissionOutboxEvent() submissioncontract.SubmissionJudgedV1 {
	submittedAt := time.Date(2026, time.July, 19, 9, 0, 0, 0, time.UTC)
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000030")
	judgementID := uuid.MustParse("00000000-0000-7000-8000-000000000040")
	return submissioncontract.SubmissionJudgedV1{
		EventID:           uuid.MustParse("00000000-0000-7000-8000-000000000001"),
		SubmissionID:      uuid.MustParse("00000000-0000-7000-8000-000000000002"),
		UserID:            uuid.MustParse("00000000-0000-7000-8000-000000000010"),
		ProblemID:         uuid.MustParse("00000000-0000-7000-8000-000000000020"),
		Generation:        1,
		JudgementID:       judgementID,
		ProblemRevisionID: uuid.MustParse("00000000-0000-7000-8000-000000000021"),
		TestsetRevisionID: uuid.MustParse("00000000-0000-7000-8000-000000000022"),
		AggregateVersion:  1,
		LatestPhase:       "terminal",
		LatestOutcome:     "accepted",
		Verdict:           "accepted",
		PassedCount:       3,
		TotalCount:        3,
		BecameActive:      true,
		ResultingActive: &submissioncontract.ActiveJudgementV1{
			JudgementID: judgementID, Generation: 1, Verdict: "accepted",
		},
		Contest: &submissioncontract.ContestSubmissionSnapshotV1{
			ContestID:            contestID,
			ContestProblemID:     uuid.MustParse("00000000-0000-7000-8000-000000000031"),
			ScoringPolicyVersion: 1,
			ElapsedMilliseconds:  1_000,
			SubmissionSequence:   1,
		},
		SubmittedAt:   submittedAt,
		OccurredAt:    submittedAt.Add(time.Second),
		CorrelationID: "00000000-0000-7000-8000-000000000099",
	}
}
