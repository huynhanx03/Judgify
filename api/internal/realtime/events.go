package realtime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"

	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
)

const (
	EventSubmissionJudgedV1                 = "submission.judged.v1"
	EventContestStandingsChangedV1          = "contest.standings.changed.v1"
	EventAuthorizationCapabilitiesChangedV1 = "authorization.capabilities.changed.v1"
)

type ContestStandingsChangedV1 struct {
	ContestID      uuid.UUID `json:"contest_id"`
	SubmissionID   uuid.UUID `json:"submission_id"`
	VerdictVersion int64     `json:"verdict_version"`
	ChangedAt      time.Time `json:"changed_at"`
}

// AuthorizationCapabilitiesChangedV1 is a coalescible invalidation only. It
// deliberately contains no subjects, roles, policies, or capability grants;
// authenticated clients fetch their own authoritative projection over REST.
type AuthorizationCapabilitiesChangedV1 struct {
	Revision  int64     `json:"revision"`
	ChangedAt time.Time `json:"changed_at"`
}

func (event AuthorizationCapabilitiesChangedV1) Validate() error {
	if event.Revision <= 0 || event.ChangedAt.IsZero() || !validUTCTime(event.ChangedAt) {
		return ErrInvalidChange
	}
	return nil
}

type AuthorizationRevisionChange struct {
	Event AuthorizationCapabilitiesChangedV1
}

func (change AuthorizationRevisionChange) Validate() error {
	return change.Event.Validate()
}

// MessageForAuthorizationChange maps the durable revision fence to one
// latest-state WebSocket message. Intermediate revisions may be coalesced:
// the revision is only a lower bound telling clients when to refetch REST.
func MessageForAuthorizationChange(
	change AuthorizationRevisionChange,
) (RoutedMessage, error) {
	if err := change.Validate(); err != nil {
		return RoutedMessage{}, err
	}
	topic := AuthorizationCapabilitiesTopic()
	data, err := json.Marshal(change.Event)
	if err != nil {
		return RoutedMessage{}, fmt.Errorf("encode authorization realtime event: %w", err)
	}
	revision := strconv.FormatInt(change.Event.Revision, 10)
	return RoutedMessage{
		Topic: topic,
		Message: commonwebsocket.Message{
			Envelope: commonwebsocket.Envelope{
				Version:          commonwebsocket.ProtocolVersion,
				Operation:        commonwebsocket.OperationEvent,
				ID:               "authorization:" + revision,
				Cursor:           revision,
				Topic:            topic,
				Type:             EventAuthorizationCapabilitiesChangedV1,
				AggregateVersion: change.Event.Revision,
				OccurredAt:       change.Event.ChangedAt,
				Data:             data,
			},
			Class:       commonwebsocket.DeliveryLatestState,
			CoalesceKey: topic,
		},
	}, nil
}

func (event ContestStandingsChangedV1) Validate() error {
	if event.ContestID == uuid.Nil || event.SubmissionID == uuid.Nil ||
		event.VerdictVersion <= 0 || event.ChangedAt.IsZero() || !validUTCTime(event.ChangedAt) {
		return ErrInvalidChange
	}
	return nil
}

// SubmissionJudgedProjectionV1 is the closed browser-safe projection of the
// committed Submission outbox contract. It deliberately is not the legacy
// table-tail projection: language and measured resource usage are not part of
// the immutable outbox event and therefore cannot be reconstructed here.
// Clients treat it as an invalidation and reload the canonical submission
// snapshot rather than inventing those missing fields.
type SubmissionJudgedProjectionV1 struct {
	SubmissionID     uuid.UUID  `json:"submission_id"`
	UserID           uuid.UUID  `json:"user_id"`
	ProblemID        uuid.UUID  `json:"problem_id"`
	ContestID        *uuid.UUID `json:"contest_id,omitempty"`
	Generation       int64      `json:"generation"`
	Verdict          string     `json:"verdict"`
	PassedCount      int        `json:"passed_count"`
	TotalCount       int        `json:"total_count"`
	AggregateVersion int64      `json:"aggregate_version"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	JudgedAt         time.Time  `json:"judged_at"`
}

func SubmissionProjectionFromOutbox(
	event submissioncontract.SubmissionJudgedV1,
) (SubmissionJudgedProjectionV1, error) {
	if event.EventID == uuid.Nil || event.SubmissionID == uuid.Nil ||
		event.UserID == uuid.Nil || event.ProblemID == uuid.Nil ||
		event.Generation < 1 || event.AggregateVersion < 1 ||
		event.Verdict == "" || event.PassedCount < 0 ||
		event.TotalCount < 0 || event.PassedCount > event.TotalCount ||
		event.SubmittedAt.IsZero() || event.OccurredAt.IsZero() ||
		!validUTCTime(event.SubmittedAt) || !validUTCTime(event.OccurredAt) ||
		event.OccurredAt.Before(event.SubmittedAt) {
		return SubmissionJudgedProjectionV1{}, ErrInvalidChange
	}
	verdict := submissionentity.Verdict(event.Verdict)
	if !verdict.Valid() ||
		(verdict == submissionentity.VerdictAccepted &&
			(event.TotalCount == 0 || event.PassedCount != event.TotalCount)) {
		return SubmissionJudgedProjectionV1{}, ErrInvalidChange
	}
	if event.Contest != nil {
		if event.Contest.ContestID == uuid.Nil ||
			event.Contest.ContestProblemID == uuid.Nil ||
			event.Contest.ScoringPolicyVersion < 1 ||
			event.Contest.ElapsedMilliseconds < 0 ||
			event.Contest.SubmissionSequence < 1 {
			return SubmissionJudgedProjectionV1{}, ErrInvalidChange
		}
	}
	projection := SubmissionJudgedProjectionV1{
		SubmissionID:     event.SubmissionID,
		UserID:           event.UserID,
		ProblemID:        event.ProblemID,
		Generation:       event.Generation,
		Verdict:          event.Verdict,
		PassedCount:      event.PassedCount,
		TotalCount:       event.TotalCount,
		AggregateVersion: event.AggregateVersion,
		SubmittedAt:      event.SubmittedAt.UTC(),
		JudgedAt:         event.OccurredAt.UTC(),
	}
	if event.Contest != nil {
		contestID := event.Contest.ContestID
		projection.ContestID = &contestID
	}
	return projection, nil
}

// MessagesForSubmissionOutbox maps one committed outbox fact to the bounded
// user projection and, when applicable, the public standings invalidation.
// The caller must be the durable outbox route; this helper never observes
// mutable submission rows or performs its own delivery.
func MessagesForSubmissionOutbox(
	event submissioncontract.SubmissionJudgedV1,
) ([]RoutedMessage, error) {
	projection, err := SubmissionProjectionFromOutbox(event)
	if err != nil {
		return nil, err
	}
	topic, err := UserSubmissionsTopic(projection.UserID)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(projection)
	if err != nil {
		return nil, fmt.Errorf("encode submission outbox projection: %w", err)
	}
	messageID := eventID(projection.SubmissionID, projection.AggregateVersion)
	messages := []RoutedMessage{{
		Topic: topic,
		Message: commonwebsocket.Message{
			Envelope: commonwebsocket.Envelope{
				Version:          commonwebsocket.ProtocolVersion,
				Operation:        commonwebsocket.OperationEvent,
				ID:               messageID,
				Topic:            topic,
				Type:             EventSubmissionJudgedV1,
				AggregateVersion: projection.AggregateVersion,
				OccurredAt:       projection.JudgedAt,
				Data:             payload,
			},
			Class: commonwebsocket.DeliveryCritical,
		},
	}}
	if projection.ContestID == nil {
		return messages, nil
	}
	standingsTopic, err := ContestStandingsTopic(*projection.ContestID)
	if err != nil {
		return nil, err
	}
	standings := ContestStandingsChangedV1{
		ContestID:      *projection.ContestID,
		SubmissionID:   projection.SubmissionID,
		VerdictVersion: projection.AggregateVersion,
		ChangedAt:      projection.JudgedAt,
	}
	if err := standings.Validate(); err != nil {
		return nil, err
	}
	standingsPayload, err := json.Marshal(standings)
	if err != nil {
		return nil, fmt.Errorf("encode standings outbox projection: %w", err)
	}
	messages = append(messages, RoutedMessage{
		Topic: standingsTopic,
		Message: commonwebsocket.Message{
			Envelope: commonwebsocket.Envelope{
				Version:          commonwebsocket.ProtocolVersion,
				Operation:        commonwebsocket.OperationEvent,
				ID:               messageID,
				Topic:            standingsTopic,
				Type:             EventContestStandingsChangedV1,
				AggregateVersion: projection.AggregateVersion,
				OccurredAt:       projection.JudgedAt,
				Data:             standingsPayload,
			},
			Class:       commonwebsocket.DeliveryLatestState,
			CoalesceKey: "contest-standings:" + projection.ContestID.String(),
		},
	})
	return messages, nil
}

type RoutedMessage struct {
	Topic   string
	Message commonwebsocket.Message
}

func eventID(submissionID uuid.UUID, version int64) string {
	return fmt.Sprintf("%s:%d", submissionID.String(), version)
}

func validUTCTime(value time.Time) bool {
	_, offset := value.Zone()
	return offset == 0
}
