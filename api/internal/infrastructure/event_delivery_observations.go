package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	notificationcontract "github.com/huynhanx03/judgify/internal/notification/contract"
	notificationdto "github.com/huynhanx03/judgify/internal/notification/contract"
)

const (
	notificationObservationHandler = "observation.notification-inbox"
	campaignObservationHandler     = "observation.notification-campaign"
)

// eventObservationRoutes terminates integration facts that already completed
// their only in-process side effect in the committing transaction. The route
// still strictly validates the durable envelope and records a CID-linked
// trace; it deliberately does not duplicate best-effort WebSocket delivery.
func eventObservationRoutes() []commonoutbox.Route {
	routes := make([]commonoutbox.Route, 0, 20)
	for _, eventType := range []string{
		notificationcontract.EventCreated,
		notificationcontract.EventRead,
		notificationcontract.EventArchived,
	} {
		eventType := eventType
		routes = append(routes, strictJSONRoute(
			eventType,
			notificationObservationHandler,
			32<<10,
			func(
				message commonoutbox.Message,
				event notificationdto.InboxEventPayload,
			) error {
				return validateNotificationObservation(
					message,
					eventType,
					event,
				)
			},
			func(
				ctx context.Context,
				event notificationdto.InboxEventPayload,
			) error {
				logger.FromContext(ctx).Debug(
					"notification integration fact observed",
					zap.String("event_type", eventType),
					zap.Int64("inbox_version", event.InboxVersion),
				)
				return nil
			},
			nil,
		))
	}
	for _, eventType := range []string{
		notificationcontract.EventCampaignDraft,
		notificationcontract.EventCampaignStatus,
	} {
		eventType := eventType
		routes = append(routes, strictJSONRoute(
			eventType,
			campaignObservationHandler,
			4<<10,
			func(
				message commonoutbox.Message,
				event notificationcontract.CampaignLifecycleEvent,
			) error {
				return validateCampaignObservation(
					message,
					eventType,
					event,
				)
			},
			func(
				ctx context.Context,
				event notificationcontract.CampaignLifecycleEvent,
			) error {
				logger.FromContext(ctx).Debug(
					"notification campaign integration fact observed",
					zap.String("event_type", eventType),
					zap.String("campaign_id", event.CampaignID.String()),
					zap.String("status", event.Status),
					zap.Int64("aggregate_version", event.Version),
				)
				return nil
			},
			nil,
		))
	}
	routes = append(routes, submissionWorkObservationRoutes()...)
	routes = append(routes, cultivationAdministrationObservationRoutes()...)
	routes = append(routes, contestAdministrationObservationRoutes()...)
	routes = append(routes, problemAuthoringObservationRoutes()...)
	routes = append(routes, materialAuthoringObservationRoutes()...)
	routes = append(routes, identityObservationRoutes()...)
	return routes
}

func validateNotificationObservation(
	message commonoutbox.Message,
	eventType string,
	event notificationdto.InboxEventPayload,
) error {
	if event.UnreadCount < 0 || event.InboxVersion < 1 {
		return ErrInvalidEventDelivery
	}
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	expectation := eventEnvelopeExpectation{
		EventID:     messageID,
		Destination: eventType,
	}
	if event.Notification == nil {
		if eventType != notificationcontract.EventRead {
			return ErrInvalidEventDelivery
		}
		aggregateID, err := canonicalMessageUUID(string(message.Key))
		if err != nil {
			return ErrInvalidEventDelivery
		}
		expectation.AggregateType =
			notificationcontract.NotificationInboxAggregateType
		expectation.AggregateID = aggregateID
		expectation.AggregateVersion = event.InboxVersion
	} else {
		if err := validateNotificationProjection(*event.Notification); err != nil {
			return err
		}
		expectation.AggregateType =
			notificationcontract.NotificationAggregateType
		expectation.AggregateID = event.Notification.ID
		expectation.AggregateVersion = event.Notification.Version
		if eventType == notificationcontract.EventCreated &&
			(event.Notification.ReadAt != nil ||
				event.Notification.ArchivedAt != nil) {
			return ErrInvalidEventDelivery
		}
		if eventType == notificationcontract.EventArchived &&
			event.Notification.ArchivedAt == nil {
			return ErrInvalidEventDelivery
		}
		if eventType == notificationcontract.EventRead &&
			event.Notification.ReadAt == nil {
			return ErrInvalidEventDelivery
		}
	}
	return validateEventEnvelope(message, expectation)
}

func validateNotificationProjection(
	event notificationdto.InboxNotificationPayload,
) error {
	if event.ID == uuid.Nil || event.Version < 1 ||
		event.CreatedAt.IsZero() ||
		!notificationcontract.ValidType(event.Type) ||
		!notificationcontract.ValidCategory(event.Category) ||
		!notificationcontract.ValidContentKind(event.ContentKind) ||
		event.ReadAt != nil && event.ReadAt.IsZero() ||
		event.ArchivedAt != nil && event.ArchivedAt.IsZero() ||
		event.ExpiresAt != nil && !event.ExpiresAt.After(event.CreatedAt) {
		return ErrInvalidEventDelivery
	}
	switch event.ContentKind {
	case notificationcontract.ContentKindCatalog:
		if event.CampaignRevisionID != nil || event.TitleKey == nil ||
			event.BodyKey == nil ||
			!notificationcontract.ValidMessageKey(*event.TitleKey) ||
			!notificationcontract.ValidMessageKey(*event.BodyKey) ||
			len(event.Params) == 0 ||
			len(event.Params) > notificationcontract.MaximumParamsBytes ||
			!json.Valid(event.Params) {
			return ErrInvalidEventDelivery
		}
	case notificationcontract.ContentKindCampaign:
		if event.CampaignRevisionID == nil ||
			*event.CampaignRevisionID == uuid.Nil ||
			event.TitleKey != nil || event.BodyKey != nil ||
			len(event.Params) != 0 {
			return ErrInvalidEventDelivery
		}
	default:
		return ErrInvalidEventDelivery
	}
	if event.ActionPath != nil &&
		notificationcontract.ValidateActionPath(*event.ActionPath) != nil {
		return ErrInvalidEventDelivery
	}
	return nil
}

func validateCampaignObservation(
	message commonoutbox.Message,
	eventType string,
	event notificationcontract.CampaignLifecycleEvent,
) error {
	if err := event.Validate(); err != nil {
		return ErrInvalidEventDelivery
	}
	switch eventType {
	case notificationcontract.EventCampaignDraft:
		if event.Status != notificationcontract.CampaignStatusDraft ||
			event.OperationID != nil {
			return ErrInvalidEventDelivery
		}
	case notificationcontract.EventCampaignStatus:
		if event.Status == notificationcontract.CampaignStatusDraft {
			return ErrInvalidEventDelivery
		}
	default:
		return ErrInvalidEventDelivery
	}
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          messageID,
		Destination:      eventType,
		AggregateType:    notificationcontract.CampaignAggregateType,
		AggregateID:      event.CampaignID,
		AggregateVersion: event.Version,
	})
}
