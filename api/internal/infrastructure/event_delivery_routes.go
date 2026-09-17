package infrastructure

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"
	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	"github.com/huynhanx03/judgify/internal/di"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	materialentity "github.com/huynhanx03/judgify/internal/material/domain"
	platformdelivery "github.com/huynhanx03/judgify/internal/platform/eventdelivery"
)

type eventEnvelopeExpectation struct {
	EventID          uuid.UUID
	Destination      string
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	CorrelationID    string
	OccurredAt       time.Time
}

type eventPayloadValidator[Event any] func(
	commonoutbox.Message,
	Event,
) error

type eventPayloadConsumer[Event any] func(
	context.Context,
	Event,
) error

type eventErrorClassifier func(error) error

func eventDeliveryRoutes(
	container *di.Container,
	database *internalent.EntClient,
) ([]commonoutbox.Route, []platformdelivery.Reconciler, error) {
	if container == nil || container.Notification == nil ||
		container.Notification.Consumer == nil || container.Material == nil ||
		container.Material.ViewProjection == nil || container.Realtime == nil ||
		container.Realtime.Publisher == nil || container.Contest == nil ||
		container.Contest.RatingService == nil || database == nil {
		return nil, nil, ErrInvalidEventDelivery
	}
	routes := []commonoutbox.Route{
		strictJSONRoute(
			materialentity.MaterialViewRecordedV1,
			"material.daily-view-projection",
			4<<10,
			validateMaterialViewEvent,
			func(
				ctx context.Context,
				event materialentity.DailyViewRecorded,
			) error {
				applied, err := container.Material.ViewProjection.ApplyDailyViewProjection(
					ctx,
					event.ReceiptID,
					event.MaterialID,
				)
				if err != nil {
					return err
				}
				logger.FromContext(ctx).Debug(
					"material daily view projection processed",
					zap.String("receipt_id", event.ReceiptID.String()),
					zap.String("material_id", event.MaterialID.String()),
					zap.Bool("projection_applied", applied),
				)
				return nil
			},
			func(err error) error {
				return retryableEventFailure("material_projection_failed", err)
			},
		),
	}
	notificationRoutes, err := eventNotificationRoutes(
		container.Notification.Consumer,
		container.Realtime.Publisher,
		container.Realtime.Publisher,
		container.Realtime.AuthorizationSource,
	)
	if err != nil {
		return nil, nil, err
	}
	routes = append(routes, notificationRoutes...)
	contestRoutes, err := eventContestRealtimeRoutes(
		container.Realtime.Publisher,
	)
	if err != nil {
		return nil, nil, err
	}
	routes = append(routes, contestRoutes...)
	ratingRoute, err := eventRatingRoute(
		container.Contest.RatingService,
		container.Notification.Consumer,
		container.Realtime.Publisher,
	)
	if err != nil {
		return nil, nil, err
	}
	routes = append(routes, ratingRoute)
	ratingActivationRoute, err :=
		eventRatingProjectionActivatedRoute(
			database,
			container.Realtime.Publisher,
		)
	if err != nil {
		return nil, nil, err
	}
	routes = append(routes, ratingActivationRoute)
	routes = append(routes, eventObservationRoutes()...)
	reconcilers := []platformdelivery.Reconciler{
		{
			Name:      "material.daily-view",
			Reconcile: container.Material.ViewProjection.ReconcileDailyViewProjection,
		},
	}
	return routes, reconcilers, nil
}

func strictJSONRoute[Event any](
	destination string,
	handlerName string,
	maximumPayloadBytes int,
	validate eventPayloadValidator[Event],
	consume eventPayloadConsumer[Event],
	classify eventErrorClassifier,
) commonoutbox.Route {
	return commonoutbox.Route{
		Destination: destination,
		HandlerName: handlerName,
		Handle: func(ctx context.Context, message commonoutbox.Message) error {
			if maximumPayloadBytes <= 0 || validate == nil || consume == nil {
				return permanentEventFailure(
					"invalid_event_handler",
					ErrInvalidEventDelivery,
				)
			}
			var event Event
			if err := commonjson.UnmarshalStrictLimit(
				message.Payload,
				&event,
				maximumPayloadBytes,
			); err != nil {
				return permanentEventFailure("invalid_event_payload", err)
			}
			if err := validate(message, event); err != nil {
				return permanentEventFailure("invalid_event_envelope", err)
			}
			eventContext := logger.WithFields(
				ctx,
				logger.String("event_id", message.ID),
			)
			if err := consume(eventContext, event); err != nil {
				if classify != nil {
					return classify(err)
				}
				return err
			}
			return nil
		},
	}
}

func validateMaterialViewEvent(
	message commonoutbox.Message,
	event materialentity.DailyViewRecorded,
) error {
	if event.ReceiptID == uuid.Nil || event.MaterialID == uuid.Nil ||
		event.ViewDay.IsZero() ||
		!event.ViewDay.Equal(event.ViewDay.UTC().Truncate(24*time.Hour)) {
		return ErrInvalidEventDelivery
	}
	eventID := uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte(materialentity.MaterialViewRecordedV1+"\x00"+event.ReceiptID.String()),
	)
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          eventID,
		Destination:      materialentity.MaterialViewRecordedV1,
		AggregateType:    "material_view",
		AggregateID:      event.ReceiptID,
		AggregateVersion: 1,
	})
}

func validateEventEnvelope(
	message commonoutbox.Message,
	expected eventEnvelopeExpectation,
) error {
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil || messageID != expected.EventID ||
		message.Destination != expected.Destination {
		return ErrInvalidEventDelivery
	}
	aggregateID, err := canonicalMessageUUID(string(message.Key))
	if err != nil || aggregateID != expected.AggregateID {
		return ErrInvalidEventDelivery
	}
	aggregateType, err := uniqueMessageAttribute(message, "aggregate_type")
	if err != nil || aggregateType != expected.AggregateType {
		return ErrInvalidEventDelivery
	}
	versionText, err := uniqueMessageAttribute(message, "aggregate_version")
	if err != nil {
		return ErrInvalidEventDelivery
	}
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version != expected.AggregateVersion {
		return ErrInvalidEventDelivery
	}
	if correlation.Validate(message.Metadata.CorrelationID) != nil ||
		expected.CorrelationID != "" &&
			message.Metadata.CorrelationID != expected.CorrelationID {
		return ErrInvalidEventDelivery
	}
	occurredText, err := uniqueMessageAttribute(message, "occurred_at")
	if err != nil {
		return ErrInvalidEventDelivery
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, occurredText)
	if err != nil || occurredAt.IsZero() {
		return ErrInvalidEventDelivery
	}
	if !expected.OccurredAt.IsZero() &&
		absoluteDuration(occurredAt.Sub(expected.OccurredAt.UTC())) > time.Microsecond {
		return ErrInvalidEventDelivery
	}
	return nil
}

func uniqueMessageAttribute(
	message commonoutbox.Message,
	key string,
) (string, error) {
	value := ""
	found := false
	for _, attribute := range message.Metadata.Attributes {
		if attribute.Key != key {
			continue
		}
		if found {
			return "", ErrInvalidEventDelivery
		}
		value = attribute.Value
		found = true
	}
	if !found || value == "" {
		return "", ErrInvalidEventDelivery
	}
	return value, nil
}

func canonicalMessageUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, ErrInvalidEventDelivery
	}
	return parsed, nil
}

func permanentEventFailure(code string, cause error) error {
	return &commonoutbox.PublishError{
		Code:      code,
		Retryable: false,
		Ambiguous: false,
		Cause:     cause,
	}
}

func retryableEventFailure(code string, cause error) error {
	if cause == nil {
		cause = ErrInvalidEventDelivery
	}
	var classified *commonoutbox.PublishError
	if errors.As(cause, &classified) {
		return cause
	}
	return &commonoutbox.PublishError{
		Code:      code,
		Retryable: true,
		Ambiguous: true,
		Cause:     cause,
	}
}

func absoluteDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
