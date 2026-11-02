package realtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
)

func TestPrivateContestInvalidationContainsNoPrivateContentOrIdentity(t *testing.T) {
	eventID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	clarificationID := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	requesterID := uuid.MustParse("00000000-0000-7000-8000-000000000004")
	changedAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	messages, err := MessagesForContestChange(ContestChange{
		EventID:      eventID,
		EventType:    contestcontract.EventClarificationAnsweredV1,
		Audience:     ContestAudiencePrivateUser,
		RecipientID:  requesterID,
		IncludeStaff: true,
		Event: ContestInvalidationV1{
			ContestID:  contestID,
			Resource:   ContestResourceClarification,
			ResourceID: clarificationID,
			Status:     "answered",
			Version:    2,
			ChangedAt:  changedAt,
		},
	})
	require.NoError(t, err)
	require.Len(t, messages, 2)

	privateTopic, err := ContestPrivateUserTopic(contestID, requesterID)
	require.NoError(t, err)
	staffTopic, err := ContestStaffTopic(contestID)
	require.NoError(t, err)
	require.Equal(t, privateTopic, messages[0].Topic)
	require.Equal(t, staffTopic, messages[1].Topic)

	for _, message := range messages {
		var payload map[string]any
		require.NoError(t, json.Unmarshal(message.Message.Envelope.Data, &payload))
		require.NotContains(t, payload, "requester_id")
		require.NotContains(t, payload, "question")
		require.NotContains(t, payload, "question_markdown")
		require.NotContains(t, payload, "answer")
		require.NotContains(t, strings.ToLower(string(message.Message.Envelope.Data)), "markdown")
	}
}

func TestContestInvalidationCoalescesPerResourceInsteadOfWholeContest(t *testing.T) {
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	firstID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	secondID := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	changedAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	build := func(eventID uuid.UUID, resourceID uuid.UUID) string {
		messages, err := MessagesForContestChange(ContestChange{
			EventID:   eventID,
			EventType: contestcontract.EventAnnouncementPublishedV1,
			Audience:  ContestAudiencePublic,
			Event: ContestInvalidationV1{
				ContestID:  contestID,
				Resource:   ContestResourceAnnouncement,
				ResourceID: resourceID,
				Status:     "published",
				Version:    2,
				ChangedAt:  changedAt,
			},
		})
		require.NoError(t, err)
		require.Len(t, messages, 1)
		return messages[0].Message.CoalesceKey
	}
	require.NotEqual(t, build(firstID, firstID), build(secondID, secondID))
}

func TestPublicLifecycleInvalidationReachesDiscoveryContestAndStaffTopics(t *testing.T) {
	eventID := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	contestID := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	messages, err := MessagesForContestChange(ContestChange{
		EventID:      eventID,
		EventType:    contestcontract.EventLifecycleChangedV1,
		Audience:     ContestAudiencePublic,
		IncludeStaff: true,
		Event: ContestInvalidationV1{
			ContestID:  contestID,
			Resource:   ContestResourceLifecycle,
			ResourceID: contestID,
			Status:     "running",
			Version:    3,
			ChangedAt:  time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	require.Len(t, messages, 3)
	require.Equal(t, ContestLifecycleTopic(), messages[0].Topic)

	publicTopic, err := ContestPublicTopic(contestID)
	require.NoError(t, err)
	staffTopic, err := ContestStaffTopic(contestID)
	require.NoError(t, err)
	require.Equal(t, publicTopic, messages[1].Topic)
	require.Equal(t, staffTopic, messages[2].Topic)
}
