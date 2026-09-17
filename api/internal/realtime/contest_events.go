package realtime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
)

type ContestAudience uint8

const (
	ContestAudiencePublic ContestAudience = iota + 1
	ContestAudienceParticipants
	ContestAudienceStaff
	ContestAudiencePrivateUser
)

type ContestResource string

const (
	ContestResourceLifecycle     ContestResource = "lifecycle"
	ContestResourceAnnouncement  ContestResource = "announcement"
	ContestResourceClarification ContestResource = "clarification"
	ContestResourceRating        ContestResource = "rating"
)

// ContestInvalidationV1 is deliberately content-free. A WebSocket event only
// tells the browser which authoritative REST projection is stale.
type ContestInvalidationV1 struct {
	ContestID  uuid.UUID       `json:"contest_id"`
	Resource   ContestResource `json:"resource"`
	ResourceID uuid.UUID       `json:"resource_id"`
	Status     string          `json:"status"`
	Version    int64           `json:"version"`
	ChangedAt  time.Time       `json:"changed_at"`
}

func (event ContestInvalidationV1) Validate() error {
	if event.ContestID == uuid.Nil || event.ResourceID == uuid.Nil ||
		event.Version < 1 || event.ChangedAt.IsZero() ||
		!validUTCTime(event.ChangedAt) {
		return ErrInvalidChange
	}
	switch event.Resource {
	case ContestResourceLifecycle:
		if event.ResourceID != event.ContestID ||
			!validContestLifecycleState(event.Status) {
			return ErrInvalidChange
		}
	case ContestResourceAnnouncement:
		if event.Status != "published" && event.Status != "withdrawn" {
			return ErrInvalidChange
		}
	case ContestResourceClarification:
		if event.Status != "open" && event.Status != "answered" &&
			event.Status != "closed" &&
			event.Status != "withdrawn" {
			return ErrInvalidChange
		}
	case ContestResourceRating:
		if event.Status != "applied" {
			return ErrInvalidChange
		}
	default:
		return ErrInvalidChange
	}
	return nil
}

type ContestChange struct {
	EventID      uuid.UUID
	EventType    string
	Audience     ContestAudience
	RecipientID  uuid.UUID
	IncludeStaff bool
	Event        ContestInvalidationV1
}

func (change ContestChange) Validate() error {
	if change.EventID == uuid.Nil || change.Event.Validate() != nil ||
		!contestEventMatchesProjection(change.EventType, change.Event) {
		return ErrInvalidChange
	}
	switch change.Audience {
	case ContestAudiencePublic:
		if change.RecipientID != uuid.Nil ||
			change.Event.Resource == ContestResourceClarification {
			return ErrInvalidChange
		}
	case ContestAudienceParticipants:
		if change.RecipientID != uuid.Nil ||
			change.Event.Resource == ContestResourceLifecycle {
			return ErrInvalidChange
		}
	case ContestAudienceStaff:
		if change.RecipientID != uuid.Nil {
			return ErrInvalidChange
		}
	case ContestAudiencePrivateUser:
		if change.RecipientID == uuid.Nil ||
			change.Event.Resource != ContestResourceClarification {
			return ErrInvalidChange
		}
	default:
		return ErrInvalidChange
	}
	return nil
}

func MessagesForContestChange(change ContestChange) ([]RoutedMessage, error) {
	if err := change.Validate(); err != nil {
		return nil, err
	}
	primaryTopic, err := contestChangeTopic(
		change.Event.ContestID,
		change.Audience,
		change.RecipientID,
	)
	if err != nil {
		return nil, err
	}
	topics := []string{primaryTopic}
	if change.Audience == ContestAudiencePublic &&
		change.Event.Resource == ContestResourceLifecycle {
		topics = []string{ContestLifecycleTopic(), primaryTopic}
	}
	if change.IncludeStaff && change.Audience != ContestAudienceStaff {
		staffTopic, err := ContestStaffTopic(change.Event.ContestID)
		if err != nil {
			return nil, err
		}
		topics = append(topics, staffTopic)
	}
	data, err := json.Marshal(change.Event)
	if err != nil {
		return nil, fmt.Errorf("encode contest realtime invalidation: %w", err)
	}
	cursor := string(change.Event.Resource) + ":" +
		change.Event.ResourceID.String() + ":" +
		strconv.FormatInt(change.Event.Version, 10)
	messages := make([]RoutedMessage, 0, len(topics))
	for _, topic := range topics {
		messages = append(messages, RoutedMessage{
			Topic: topic,
			Message: commonwebsocket.Message{
				Envelope: commonwebsocket.Envelope{
					Version:          commonwebsocket.ProtocolVersion,
					Operation:        commonwebsocket.OperationEvent,
					ID:               change.EventID.String(),
					Cursor:           cursor,
					Topic:            topic,
					Type:             change.EventType,
					AggregateVersion: change.Event.Version,
					OccurredAt:       change.Event.ChangedAt,
					Data:             data,
				},
				Class: commonwebsocket.DeliveryLatestState,
				CoalesceKey: topic + ":" + string(change.Event.Resource) +
					":" + change.Event.ResourceID.String(),
			},
		})
	}
	return messages, nil
}

func contestChangeTopic(
	contestID uuid.UUID,
	audience ContestAudience,
	recipientID uuid.UUID,
) (string, error) {
	switch audience {
	case ContestAudiencePublic:
		return ContestPublicTopic(contestID)
	case ContestAudienceParticipants:
		return ContestParticipantsTopic(contestID)
	case ContestAudienceStaff:
		return ContestStaffTopic(contestID)
	case ContestAudiencePrivateUser:
		return ContestPrivateUserTopic(contestID, recipientID)
	default:
		return "", ErrInvalidChange
	}
}

func contestEventMatchesProjection(
	eventType string,
	event ContestInvalidationV1,
) bool {
	switch eventType {
	case contestcontract.EventLifecycleChangedV1:
		return event.Resource == ContestResourceLifecycle
	case contestcontract.EventAnnouncementPublishedV1:
		return event.Resource == ContestResourceAnnouncement &&
			event.Status == "published"
	case contestcontract.EventAnnouncementWithdrawnV1:
		return event.Resource == ContestResourceAnnouncement &&
			event.Status == "withdrawn"
	case contestcontract.EventClarificationAnsweredV1:
		return event.Resource == ContestResourceClarification &&
			event.Status == "answered"
	case contestcontract.EventClarificationSubmittedV1:
		return event.Resource == ContestResourceClarification &&
			event.Status == "open"
	case contestcontract.EventClarificationClosedV1:
		return event.Resource == ContestResourceClarification &&
			event.Status == "closed"
	case contestcontract.EventClarificationWithdrawnV1:
		return event.Resource == ContestResourceClarification &&
			event.Status == "withdrawn"
	case contestcontract.EventRatingCalculatedV1:
		return event.Resource == ContestResourceRating &&
			event.Status == "applied"
	case contestcontract.EventRatingProjectionActivatedV1:
		return event.Resource == ContestResourceRating &&
			event.Status == "applied"
	default:
		return false
	}
}

func validContestLifecycleState(value string) bool {
	switch value {
	case "draft", "upcoming", "running", "ended", "cancelled":
		return true
	default:
		return false
	}
}
