package infrastructure

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonwebsocket "github.com/huynhanx03/go-common/pkg/common/websocket"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"
	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	identitycontract "github.com/huynhanx03/judgify/internal/identity/contract"
	notificationcontract "github.com/huynhanx03/judgify/internal/notification/contract"
	notificationdto "github.com/huynhanx03/judgify/internal/notification/contract"
	notificationservice "github.com/huynhanx03/judgify/internal/notification/usecase"
	notificationports "github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/realtime"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

const (
	submissionNotificationHandler = "notification.submission-judged"
	securityNotificationHandler   = "notification.identity-security"
	notificationRealtimeHandler   = "realtime.notification-inbox"
	authorizationRealtimeHandler  = "realtime.authorization-revision"
	userLifecycleRealtimeHandler  = "realtime.identity-user-lifecycle"
)

const authorizationAggregateID = "00000000-0000-5000-8000-000000000001"

var authorizationRealtimeEvents = map[string]struct {
	Action   string
	Resource string
	Subject  bool
}{
	"identity.authorization.role_policies_replaced.v1": {
		Action:   "authorization.role_policies.replace",
		Resource: "authorization.role",
	},
	"identity.authorization.user_roles_replaced.v1": {
		Action:   "authorization.user_roles.replace",
		Resource: "authorization.user",
		Subject:  true,
	},
	"identity.authorization.role_deleted.v1": {
		Action:   "authorization.role.delete",
		Resource: "authorization.role",
	},
}

var authorizationRealtimeEventTypes = []string{
	"identity.authorization.role_policies_replaced.v1",
	"identity.authorization.user_roles_replaced.v1",
	"identity.authorization.role_deleted.v1",
}

type identitySecurityEvent struct {
	MutationID          string    `json:"mutation_id"`
	UserID              string    `json:"user_id"`
	SessionID           string    `json:"session_id,omitempty"`
	TargetSessionID     string    `json:"target_session_id,omitempty"`
	ContactID           string    `json:"contact_id,omitempty"`
	IntentID            string    `json:"intent_id,omitempty"`
	FederatedIdentityID string    `json:"federated_identity_id,omitempty"`
	Provider            string    `json:"provider,omitempty"`
	Status              string    `json:"status"`
	Version             uint64    `json:"version,omitempty"`
	SecurityVersion     uint64    `json:"security_version,omitempty"`
	OccurredAt          time.Time `json:"occurred_at"`
}

type securityNotificationDefinition struct {
	EventType      string
	ExpectedStatus string
	SessionCopy    bool
	Revocation     securityConnectionRevocation
}

type securityConnectionRevocation uint8

const (
	securityConnectionRevocationNone securityConnectionRevocation = iota
	securityConnectionRevocationSession
	securityConnectionRevocationSubject
)

var securityNotificationDefinitions = []securityNotificationDefinition{
	{EventType: "identity.user.registered.v1", ExpectedStatus: "active"},
	{EventType: "identity.oauth.linked.v1", ExpectedStatus: "linked"},
	{EventType: "identity.oauth.unlinked.v1", ExpectedStatus: "unlinked"},
	{EventType: "identity.recovery_contact.verified.v1", ExpectedStatus: "verified"},
	{EventType: "identity.recovery_contact.replacement_started.v1", ExpectedStatus: "pending"},
	{EventType: "identity.recovery_contact.verification_resent.v1", ExpectedStatus: "pending"},
	{EventType: "identity.recovery_contact.pending_cancelled.v1", ExpectedStatus: "revoked"},
	{EventType: "identity.recovery_contact.removed.v1", ExpectedStatus: "revoked"},
	{
		EventType: "identity.password_reset.consumed.v1", ExpectedStatus: "completed",
		Revocation: securityConnectionRevocationSubject,
	},
	{
		EventType: "identity.password.changed.v1", ExpectedStatus: "completed",
		Revocation: securityConnectionRevocationSubject,
	},
	{
		EventType: "identity.session.revoked.v1", ExpectedStatus: "revoked", SessionCopy: true,
		Revocation: securityConnectionRevocationSession,
	},
	{
		EventType: "identity.sessions.revoked_all.v1", ExpectedStatus: "revoked", SessionCopy: true,
		Revocation: securityConnectionRevocationSubject,
	},
}

func eventNotificationRoutes(
	consumer *notificationservice.Consumer,
	revoker realtime.ConnectionRevoker,
	publisher realtime.RoutedPublisher,
	authorizationSource realtime.AuthorizationRevisionSource,
) ([]commonoutbox.Route, error) {
	authorizationPublisher, ok := publisher.(realtime.AuthorizationChangePublisher)
	if consumer == nil || revoker == nil || publisher == nil ||
		authorizationSource == nil || !ok {
		return nil, ErrInvalidEventDelivery
	}
	routes := []commonoutbox.Route{
		strictJSONRoute(
			submissioncontract.EventSubmissionJudgedV1,
			submissionNotificationHandler,
			64<<10,
			validateSubmissionJudgedEvent,
			func(
				ctx context.Context,
				event submissioncontract.SubmissionJudgedV1,
			) error {
				messageID := event.EventID
				occurredAt := event.OccurredAt.UTC()
				titleKey := notificationcontract.MessageKeySubmissionJudgedTitle
				bodyKey := notificationcontract.MessageKeySubmissionJudgedBody
				if event.Verdict == "system_error" {
					titleKey = notificationcontract.MessageKeySubmissionSystemErrorTitle
					bodyKey = notificationcontract.MessageKeySubmissionSystemErrorBody
				}
				params, err := json.Marshal(map[string]any{
					"submission_id": event.SubmissionID.String(),
					"problem_id":    event.ProblemID.String(),
					"generation":    event.Generation,
					"verdict":       event.Verdict,
					"passed_count":  event.PassedCount,
					"total_count":   event.TotalCount,
				})
				if err != nil {
					return permanentEventFailure("notification_payload_failed", err)
				}
				actionPath := "/submissions/" + event.SubmissionID.String()
				_, created, err := consumer.Consume(ctx, notificationports.CreateCommand{
					UserID:        event.UserID,
					SourceEventID: messageID,
					Type:          submissioncontract.EventSubmissionJudgedV1,
					Category:      notificationcontract.CategorySubmission,
					TitleKey:      &titleKey,
					BodyKey:       &bodyKey,
					Params:        params,
					ActionPath:    &actionPath,
					OccurredAt:    occurredAt,
				})
				if err != nil {
					return err
				}
				if err := publishSubmissionOutboxProjection(
					ctx,
					publisher,
					event,
				); err != nil {
					return err
				}
				logger.FromContext(ctx).Debug(
					"submission outbox projections consumed",
					zap.String("user_id", event.UserID.String()),
					zap.String("submission_id", event.SubmissionID.String()),
					zap.Bool("notification_created", created),
				)
				return nil
			},
			classifyNotificationFailure,
		),
	}
	for _, definition := range securityNotificationDefinitions {
		definition := definition
		routes = append(routes, strictJSONRoute(
			definition.EventType,
			securityNotificationHandler,
			8<<10,
			func(
				message commonoutbox.Message,
				event identitySecurityEvent,
			) error {
				return validateIdentitySecurityEvent(message, event, definition)
			},
			func(ctx context.Context, event identitySecurityEvent) error {
				// Authentication invalidation is the security-critical effect. It
				// must not wait behind an optional inbox projection failure; both
				// effects remain replay-safe when this durable route retries.
				if err := revokeIdentitySecurityConnections(
					ctx,
					revoker,
					definition,
					event,
				); err != nil {
					return err
				}
				return consumeIdentitySecurityNotification(
					ctx,
					consumer,
					definition,
					event,
				)
			},
			classifyNotificationFailure,
		))
	}
	for _, definition := range []struct {
		eventType string
		action    string
	}{
		{eventType: identitycontract.EventUserSuspendedV1, action: "suspend"},
		{eventType: identitycontract.EventUserReactivatedV1, action: "reactivate"},
		{eventType: identitycontract.EventUserDeletedV1, action: "delete"},
	} {
		definition := definition
		routes = append(routes, strictJSONRoute(
			definition.eventType,
			userLifecycleRealtimeHandler,
			16<<10,
			func(
				message commonoutbox.Message,
				event identitycontract.UserLifecycleChangedV1,
			) error {
				return validateUserLifecycleObservation(
					message,
					definition.eventType,
					definition.action,
					event,
				)
			},
			func(
				ctx context.Context,
				event identitycontract.UserLifecycleChangedV1,
			) error {
				userID, err := canonicalMessageUUID(event.After.UserID)
				if err != nil {
					return permanentEventFailure(
						"invalid_user_lifecycle_target",
						err,
					)
				}
				count, err := revoker.RevokeSubject(ctx, userID)
				if err != nil {
					return retryableEventFailure(
						"user_lifecycle_realtime_revocation_failed",
						err,
					)
				}
				logger.FromContext(ctx).Info(
					"user lifecycle realtime authentication revoked",
					zap.String("action", event.Action),
					zap.String("user_id", userID.String()),
					zap.Int("connection_count", count),
				)
				return nil
			},
			func(err error) error {
				var failure *commonoutbox.PublishError
				if errors.As(err, &failure) {
					return err
				}
				return retryableEventFailure(
					"user_lifecycle_realtime_revocation_failed",
					err,
				)
			},
		))
	}
	for _, eventType := range []string{
		notificationcontract.EventCreated,
		notificationcontract.EventRead,
		notificationcontract.EventArchived,
	} {
		routes = append(
			routes,
			notificationRealtimeRoute(publisher, eventType),
		)
	}
	for _, eventType := range authorizationRealtimeEventTypes {
		routes = append(routes, authorizationRealtimeRoute(
			eventType,
			authorizationSource,
			authorizationPublisher,
			revoker,
		))
	}
	return routes, nil
}

// publishSubmissionOutboxProjection is deliberately called from the durable
// outbox route, after the notification consumer has committed its own inbox
// state. It never reads the mutable submissions table: the committed source
// event is the complete routing authority. A retry reuses the same projection
// identities, so replay insertion is idempotent before the Hub is offered a
// frame.
func publishSubmissionOutboxProjection(
	ctx context.Context,
	publisher realtime.RoutedPublisher,
	event submissioncontract.SubmissionJudgedV1,
) error {
	messages, err := realtime.MessagesForSubmissionOutbox(event)
	if err != nil {
		return permanentEventFailure("invalid_submission_projection", err)
	}
	for _, message := range messages {
		if err := publisher.PublishMessage(ctx, message.Topic, message.Message); err != nil {
			return retryableEventFailure("submission_realtime_failed", err)
		}
	}
	return nil
}

func notificationRealtimeRoute(
	publisher realtime.RoutedPublisher,
	eventType string,
) commonoutbox.Route {
	return commonoutbox.Route{
		Destination: eventType,
		HandlerName: notificationRealtimeHandler,
		Handle: func(ctx context.Context, message commonoutbox.Message) error {
			var event notificationdto.InboxEventPayload
			if err := commonjson.UnmarshalStrictLimit(
				message.Payload,
				&event,
				64<<10,
			); err != nil {
				return permanentEventFailure("invalid_event_payload", err)
			}
			if err := validateNotificationRealtimeEvent(
				message,
				eventType,
				event,
			); err != nil {
				return permanentEventFailure("invalid_event_envelope", err)
			}
			occurredAt, err := deliveredEventOccurredAt(message)
			if err != nil {
				return permanentEventFailure("invalid_event_envelope", err)
			}
			topic, err := realtime.NotificationTopic(event.UserID)
			if err != nil {
				return permanentEventFailure("invalid_notification_projection", err)
			}
			payload, err := json.Marshal(event)
			if err != nil {
				return permanentEventFailure("invalid_notification_projection", err)
			}
			browserMessage := commonwebsocket.Message{
				Envelope: commonwebsocket.Envelope{
					Version:          commonwebsocket.ProtocolVersion,
					Operation:        commonwebsocket.OperationEvent,
					ID:               message.ID,
					Topic:            topic,
					Type:             eventType,
					AggregateVersion: event.InboxVersion,
					OccurredAt:       occurredAt,
					Data:             payload,
				},
				Class: commonwebsocket.DeliveryCritical,
			}
			if err := publisher.PublishMessage(ctx, topic, browserMessage); err != nil {
				return retryableEventFailure("notification_realtime_failed", err)
			}
			logger.FromContext(ctx).Debug(
				"notification realtime projection delivered",
				zap.String("notification_event_type", eventType),
				zap.Int64("inbox_version", event.InboxVersion),
			)
			return nil
		},
	}
}

func validateNotificationRealtimeEvent(
	message commonoutbox.Message,
	eventType string,
	event notificationdto.InboxEventPayload,
) error {
	if event.UserID == uuid.Nil || event.InboxVersion < 1 ||
		event.UnreadCount < 0 || !validNotificationProjection(event.Notification) {
		return ErrInvalidEventDelivery
	}
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	expected := eventEnvelopeExpectation{
		EventID:     messageID,
		Destination: eventType,
	}
	switch eventType {
	case notificationcontract.EventCreated:
		if event.Notification == nil || event.Notification.ReadAt != nil ||
			event.Notification.ArchivedAt != nil {
			return ErrInvalidEventDelivery
		}
		expected.AggregateType = notificationcontract.NotificationAggregateType
		expected.AggregateID = event.Notification.ID
		expected.AggregateVersion = event.Notification.Version
	case notificationcontract.EventRead:
		if event.Notification == nil {
			expected.AggregateType = notificationcontract.NotificationInboxAggregateType
			expected.AggregateID = event.UserID
			expected.AggregateVersion = event.InboxVersion
		} else {
			if event.Notification.ReadAt == nil {
				return ErrInvalidEventDelivery
			}
			expected.AggregateType = notificationcontract.NotificationAggregateType
			expected.AggregateID = event.Notification.ID
			expected.AggregateVersion = event.Notification.Version
		}
	case notificationcontract.EventArchived:
		if event.Notification == nil || event.Notification.ArchivedAt == nil {
			return ErrInvalidEventDelivery
		}
		expected.AggregateType = notificationcontract.NotificationAggregateType
		expected.AggregateID = event.Notification.ID
		expected.AggregateVersion = event.Notification.Version
	default:
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, expected)
}

func validNotificationProjection(
	value *notificationdto.InboxNotificationPayload,
) bool {
	if value == nil {
		return true
	}
	if value.ID == uuid.Nil || value.Version < 1 || value.CreatedAt.IsZero() ||
		!notificationcontract.ValidType(value.Type) ||
		!notificationcontract.ValidCategory(value.Category) ||
		!notificationcontract.ValidContentKind(value.ContentKind) ||
		(value.ActionPath != nil &&
			notificationcontract.ValidateActionPath(*value.ActionPath) != nil) ||
		(value.ReadAt != nil && value.ReadAt.Before(value.CreatedAt)) ||
		(value.ArchivedAt != nil && value.ArchivedAt.Before(value.CreatedAt)) ||
		(value.ExpiresAt != nil && !value.ExpiresAt.After(value.CreatedAt)) {
		return false
	}
	switch value.ContentKind {
	case notificationcontract.ContentKindCatalog:
		return value.CampaignRevisionID == nil && value.CampaignTitle == nil &&
			value.TitleKey != nil &&
			notificationcontract.ValidMessageKey(*value.TitleKey) &&
			value.BodyKey != nil &&
			notificationcontract.ValidMessageKey(*value.BodyKey) &&
			len(value.Params) > 0 && len(value.Params) <= notificationcontract.MaximumParamsBytes &&
			json.Valid(value.Params)
	case notificationcontract.ContentKindCampaign:
		return value.CampaignRevisionID != nil &&
			*value.CampaignRevisionID != uuid.Nil && value.TitleKey == nil &&
			value.BodyKey == nil && len(value.Params) == 0 &&
			(value.CampaignTitle == nil ||
				notificationcontract.ValidCampaignTitle(*value.CampaignTitle))
	default:
		return false
	}
}

func deliveredEventOccurredAt(message commonoutbox.Message) (time.Time, error) {
	value, err := uniqueMessageAttribute(message, "occurred_at")
	if err != nil {
		return time.Time{}, ErrInvalidEventDelivery
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || occurredAt.IsZero() {
		return time.Time{}, ErrInvalidEventDelivery
	}
	return occurredAt.UTC(), nil
}

type authorizationMutationProjection struct {
	Action            string          `json:"action"`
	Resource          string          `json:"resource"`
	TargetID          string          `json:"target_id"`
	ExpectedRevision  int64           `json:"expected_revision"`
	ResultingRevision int64           `json:"resulting_revision"`
	AuditID           string          `json:"audit_id"`
	CommandID         string          `json:"command_id,omitempty"`
	RequestHash       string          `json:"request_hash,omitempty"`
	Result            json.RawMessage `json:"result"`
}

func authorizationRealtimeRoute(
	eventType string,
	source realtime.AuthorizationRevisionSource,
	publisher realtime.AuthorizationChangePublisher,
	revoker realtime.ConnectionRevoker,
) commonoutbox.Route {
	definition := authorizationRealtimeEvents[eventType]
	return strictJSONRoute(
		eventType,
		authorizationRealtimeHandler,
		64<<10,
		func(
			message commonoutbox.Message,
			event authorizationMutationProjection,
		) error {
			return validateAuthorizationRealtimeEvent(
				message,
				eventType,
				definition,
				event,
			)
		},
		func(ctx context.Context, event authorizationMutationProjection) error {
			change, err := source.Current(ctx)
			if err != nil {
				return retryableEventFailure("authorization_revision_unavailable", err)
			}
			if change.Event.Revision < event.ResultingRevision {
				return retryableEventFailure(
					"authorization_revision_stale",
					ErrInvalidEventDelivery,
				)
			}
			var revoked int
			if definition.Subject {
				userID, parseErr := canonicalMessageUUID(event.TargetID)
				if parseErr != nil {
					return permanentEventFailure("invalid_authorization_target", parseErr)
				}
				revoked, err = revoker.RevokeSubject(ctx, userID)
			} else {
				revoked, err = revoker.RevokeAllAuthenticated(ctx)
			}
			if err != nil {
				return retryableEventFailure("authorization_revocation_failed", err)
			}
			if err := publisher.PublishAuthorization(ctx, change); err != nil {
				return retryableEventFailure("authorization_realtime_failed", err)
			}
			logger.FromContext(ctx).Info(
				"authorization realtime revision invalidated",
				zap.Int64("authorization_revision", change.Event.Revision),
				zap.Int("connection_count", revoked),
			)
			return nil
		},
		func(err error) error {
			var failure *commonoutbox.PublishError
			if errors.As(err, &failure) {
				return err
			}
			return retryableEventFailure("authorization_realtime_failed", err)
		},
	)
}

func validateAuthorizationRealtimeEvent(
	message commonoutbox.Message,
	eventType string,
	definition struct {
		Action   string
		Resource string
		Subject  bool
	},
	event authorizationMutationProjection,
) error {
	if definition.Action == "" || definition.Resource == "" ||
		event.Action != definition.Action || event.Resource != definition.Resource ||
		event.TargetID == "" || event.ExpectedRevision < 1 ||
		event.ResultingRevision != event.ExpectedRevision+1 ||
		!canonicalReceiptIdentifier(event.AuditID) ||
		len(event.Result) < 2 || !json.Valid(event.Result) ||
		event.Result[0] != '{' || event.Result[len(event.Result)-1] != '}' {
		return ErrInvalidEventDelivery
	}
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	if definition.Subject {
		if _, err := canonicalMessageUUID(event.TargetID); err != nil {
			return ErrInvalidEventDelivery
		}
	}
	if (event.CommandID == "") != (event.RequestHash == "") {
		return ErrInvalidEventDelivery
	}
	if event.CommandID != "" {
		commandID, err := canonicalMessageUUID(event.CommandID)
		if err != nil || commandID != messageID || len(event.RequestHash) != 64 {
			return ErrInvalidEventDelivery
		}
		decoded, err := hex.DecodeString(event.RequestHash)
		if err != nil || len(decoded) != 32 {
			return ErrInvalidEventDelivery
		}
	}
	aggregateID, err := canonicalMessageUUID(authorizationAggregateID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          messageID,
		Destination:      eventType,
		AggregateType:    "authorization",
		AggregateID:      aggregateID,
		AggregateVersion: event.ResultingRevision,
	})
}

func canonicalReceiptIdentifier(value string) bool {
	_, err := canonicalMessageUUID(value)
	return err == nil
}

func validateSubmissionJudgedEvent(
	message commonoutbox.Message,
	event submissioncontract.SubmissionJudgedV1,
) error {
	if event.EventID == uuid.Nil || event.SubmissionID == uuid.Nil ||
		event.UserID == uuid.Nil || event.ProblemID == uuid.Nil ||
		event.JudgementID == uuid.Nil || event.ProblemRevisionID == uuid.Nil ||
		event.TestsetRevisionID == uuid.Nil || event.Generation <= 0 ||
		event.AggregateVersion <= 0 || event.LatestPhase == "" ||
		event.LatestOutcome == "" || event.Verdict == "" ||
		event.PassedCount < 0 || event.TotalCount < 0 ||
		event.PassedCount > event.TotalCount || event.SubmittedAt.IsZero() ||
		event.OccurredAt.IsZero() ||
		event.SubmittedAt.After(event.OccurredAt.Add(time.Microsecond)) {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          event.EventID,
		Destination:      submissioncontract.EventSubmissionJudgedV1,
		AggregateType:    submissioncontract.AggregateSubmission,
		AggregateID:      event.SubmissionID,
		AggregateVersion: event.AggregateVersion,
		CorrelationID:    event.CorrelationID,
		OccurredAt:       event.OccurredAt,
	})
}

func validateIdentitySecurityEvent(
	message commonoutbox.Message,
	event identitySecurityEvent,
	definition securityNotificationDefinition,
) error {
	mutationID, err := canonicalMessageUUID(event.MutationID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	if _, err := canonicalMessageUUID(event.UserID); err != nil ||
		event.Status != definition.ExpectedStatus ||
		definition.EventType == "" ||
		event.OccurredAt.IsZero() ||
		!validOptionalCanonicalUUID(event.SessionID) ||
		!validOptionalCanonicalUUID(event.TargetSessionID) ||
		!validOptionalCanonicalUUID(event.ContactID) ||
		!validOptionalCanonicalUUID(event.IntentID) ||
		!validOptionalCanonicalUUID(event.FederatedIdentityID) {
		return ErrInvalidEventDelivery
	}
	switch definition.Revocation {
	case securityConnectionRevocationNone:
	case securityConnectionRevocationSession:
		if event.TargetSessionID == "" {
			return ErrInvalidEventDelivery
		}
	case securityConnectionRevocationSubject:
		if event.SecurityVersion == 0 {
			return ErrInvalidEventDelivery
		}
	default:
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          mutationID,
		Destination:      definition.EventType,
		AggregateType:    "security_mutation",
		AggregateID:      mutationID,
		AggregateVersion: 1,
		OccurredAt:       event.OccurredAt,
	})
}

func revokeIdentitySecurityConnections(
	ctx context.Context,
	revoker realtime.ConnectionRevoker,
	definition securityNotificationDefinition,
	event identitySecurityEvent,
) error {
	if ctx == nil || revoker == nil {
		return ErrInvalidEventDelivery
	}
	var (
		count int
		err   error
	)
	switch definition.Revocation {
	case securityConnectionRevocationNone:
		return nil
	case securityConnectionRevocationSession:
		sessionID, parseErr := canonicalMessageUUID(event.TargetSessionID)
		if parseErr != nil {
			return permanentEventFailure("invalid_event_envelope", parseErr)
		}
		count, err = revoker.RevokeSession(ctx, sessionID)
	case securityConnectionRevocationSubject:
		userID, parseErr := canonicalMessageUUID(event.UserID)
		if parseErr != nil {
			return permanentEventFailure("invalid_event_envelope", parseErr)
		}
		count, err = revoker.RevokeSubject(ctx, userID)
	default:
		return permanentEventFailure(
			"invalid_event_envelope",
			ErrInvalidEventDelivery,
		)
	}
	if err != nil {
		return retryableEventFailure("realtime_revocation_failed", err)
	}
	logger.FromContext(ctx).Info(
		"stale realtime authentication revoked",
		zap.String("security_event_type", definition.EventType),
		zap.Int("connection_count", count),
	)
	return nil
}

func consumeIdentitySecurityNotification(
	ctx context.Context,
	consumer *notificationservice.Consumer,
	definition securityNotificationDefinition,
	event identitySecurityEvent,
) error {
	mutationID, err := canonicalMessageUUID(event.MutationID)
	if err != nil {
		return permanentEventFailure("invalid_event_envelope", err)
	}
	userID, err := canonicalMessageUUID(event.UserID)
	if err != nil {
		return permanentEventFailure("invalid_event_envelope", err)
	}
	params := map[string]any{"status": event.Status}
	if event.Provider != "" {
		params["provider"] = event.Provider
	}
	encodedParams, err := json.Marshal(params)
	if err != nil {
		return permanentEventFailure("notification_payload_failed", err)
	}
	titleKey := notificationcontract.MessageKeySecurityAccountTitle
	bodyKey := notificationcontract.MessageKeySecurityAccountBody
	if definition.SessionCopy {
		titleKey = notificationcontract.MessageKeySecuritySessionTitle
		bodyKey = notificationcontract.MessageKeySecuritySessionBody
	}
	actionPath := "/profile/security"
	_, created, err := consumer.Consume(ctx, notificationports.CreateCommand{
		UserID:        userID,
		SourceEventID: mutationID,
		Type:          definition.EventType,
		Category:      notificationcontract.CategorySecurity,
		TitleKey:      &titleKey,
		BodyKey:       &bodyKey,
		Params:        encodedParams,
		ActionPath:    &actionPath,
		OccurredAt:    event.OccurredAt.UTC(),
	})
	if err != nil {
		return err
	}
	logger.FromContext(ctx).Debug(
		"identity security notification consumed",
		zap.String("user_id", userID.String()),
		zap.String("security_event_type", definition.EventType),
		zap.Bool("notification_created", created),
	)
	return nil
}

func classifyNotificationFailure(err error) error {
	if err == nil {
		return nil
	}
	var failure *commonoutbox.PublishError
	if errors.As(err, &failure) {
		return err
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) &&
		applicationError.Code >= 40000 &&
		applicationError.Code < 50000 &&
		applicationError.Code != apperr.CodeTooManyRequests {
		return permanentEventFailure("notification_rejected", err)
	}
	if errors.Is(err, notificationports.ErrInvalidRequest) {
		return permanentEventFailure("notification_rejected", err)
	}
	return retryableEventFailure("notification_failed", err)
}

func validOptionalCanonicalUUID(value string) bool {
	if value == "" {
		return true
	}
	_, err := canonicalMessageUUID(value)
	return err == nil
}
