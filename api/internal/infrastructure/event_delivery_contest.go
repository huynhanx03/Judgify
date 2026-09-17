package infrastructure

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	contestentity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/realtime"
)

const contestRealtimeHandler = "realtime.contest-invalidation"

func eventContestRealtimeRoutes(
	publisher *realtime.HubPublisher,
) ([]commonoutbox.Route, error) {
	if publisher == nil {
		return nil, ErrInvalidEventDelivery
	}
	routes := []commonoutbox.Route{
		strictJSONRoute(
			contestcontract.EventLifecycleChangedV1,
			contestRealtimeHandler,
			4<<10,
			validateContestLifecycleEvent,
			func(
				ctx context.Context,
				event contestcontract.LifecycleChangedV1,
			) error {
				audience := realtime.ContestAudienceStaff
				includeStaff := false
				if lifecycleStateIsPublic(event.From) ||
					lifecycleStateIsPublic(event.To) {
					audience = realtime.ContestAudiencePublic
					includeStaff = true
				}
				return publishContestInvalidation(
					ctx,
					publisher,
					realtime.ContestChange{
						EventID:      event.EventID,
						EventType:    contestcontract.EventLifecycleChangedV1,
						Audience:     audience,
						IncludeStaff: includeStaff,
						Event: realtime.ContestInvalidationV1{
							ContestID:  event.ContestID,
							Resource:   realtime.ContestResourceLifecycle,
							ResourceID: event.ContestID,
							Status:     event.To,
							Version:    event.Version,
							ChangedAt:  event.OccurredAt.UTC(),
						},
					},
				)
			},
			classifyContestRealtimeFailure,
		),
		announcementRealtimeRoute(
			publisher,
			contestcontract.EventAnnouncementPublishedV1,
			"published",
		),
		announcementRealtimeRoute(
			publisher,
			contestcontract.EventAnnouncementWithdrawnV1,
			"withdrawn",
		),
		strictJSONRoute(
			contestcontract.EventClarificationSubmittedV1,
			contestRealtimeHandler,
			4<<10,
			validateClarificationSubmittedEvent,
			func(
				ctx context.Context,
				event contestcontract.ClarificationSubmittedV1,
			) error {
				return publishContestInvalidation(
					ctx,
					publisher,
					realtime.ContestChange{
						EventID:   event.EventID,
						EventType: contestcontract.EventClarificationSubmittedV1,
						Audience:  realtime.ContestAudienceStaff,
						Event: realtime.ContestInvalidationV1{
							ContestID:  event.ContestID,
							Resource:   realtime.ContestResourceClarification,
							ResourceID: event.ClarificationID,
							Status:     "open",
							Version:    event.Version,
							ChangedAt:  event.OccurredAt.UTC(),
						},
					},
				)
			},
			classifyContestRealtimeFailure,
		),
		clarificationRealtimeRoute(
			publisher,
			contestcontract.EventClarificationAnsweredV1,
			"answered",
		),
		clarificationRealtimeRoute(
			publisher,
			contestcontract.EventClarificationClosedV1,
			"closed",
		),
		clarificationRealtimeRoute(
			publisher,
			contestcontract.EventClarificationWithdrawnV1,
			"withdrawn",
		),
	}
	return routes, nil
}

func announcementRealtimeRoute(
	publisher *realtime.HubPublisher,
	eventType string,
	status string,
) commonoutbox.Route {
	return strictJSONRoute(
		eventType,
		contestRealtimeHandler,
		8<<10,
		func(
			message commonoutbox.Message,
			event contestcontract.AnnouncementChangedV1,
		) error {
			return validateAnnouncementEvent(message, event, eventType, status)
		},
		func(
			ctx context.Context,
			event contestcontract.AnnouncementChangedV1,
		) error {
			audience, includeStaff, err := announcementRealtimeAudience(
				event.Audience,
			)
			if err != nil {
				return err
			}
			return publishContestInvalidation(
				ctx,
				publisher,
				realtime.ContestChange{
					EventID:      event.EventID,
					EventType:    eventType,
					Audience:     audience,
					IncludeStaff: includeStaff,
					Event: realtime.ContestInvalidationV1{
						ContestID:  event.ContestID,
						Resource:   realtime.ContestResourceAnnouncement,
						ResourceID: event.AnnouncementID,
						Status:     status,
						Version:    event.Version,
						ChangedAt:  event.OccurredAt.UTC(),
					},
				},
			)
		},
		classifyContestRealtimeFailure,
	)
}

func clarificationRealtimeRoute(
	publisher *realtime.HubPublisher,
	eventType string,
	status string,
) commonoutbox.Route {
	return strictJSONRoute(
		eventType,
		contestRealtimeHandler,
		8<<10,
		func(
			message commonoutbox.Message,
			event contestcontract.ClarificationChangedV1,
		) error {
			return validateClarificationEvent(message, event, eventType)
		},
		func(
			ctx context.Context,
			event contestcontract.ClarificationChangedV1,
		) error {
			audience, recipientID, err := clarificationRealtimeAudience(event)
			if err != nil {
				return err
			}
			return publishContestInvalidation(
				ctx,
				publisher,
				realtime.ContestChange{
					EventID:      event.EventID,
					EventType:    eventType,
					Audience:     audience,
					RecipientID:  recipientID,
					IncludeStaff: true,
					Event: realtime.ContestInvalidationV1{
						ContestID:  event.ContestID,
						Resource:   realtime.ContestResourceClarification,
						ResourceID: event.ClarificationID,
						Status:     status,
						Version:    event.Version,
						ChangedAt:  event.OccurredAt.UTC(),
					},
				},
			)
		},
		classifyContestRealtimeFailure,
	)
}

func validateContestLifecycleEvent(
	message commonoutbox.Message,
	event contestcontract.LifecycleChangedV1,
) error {
	from := contestentity.ContestState(event.From)
	to := contestentity.ContestState(event.To)
	if event.EventID == uuid.Nil || event.ContestID == uuid.Nil ||
		event.Version < 2 || event.OccurredAt.IsZero() ||
		contestentity.ValidateContestTransition(from, to) != nil {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          event.EventID,
		Destination:      contestcontract.EventLifecycleChangedV1,
		AggregateType:    contestcontract.AggregateContest,
		AggregateID:      event.ContestID,
		AggregateVersion: event.Version,
		OccurredAt:       event.OccurredAt,
	})
}

func validateAnnouncementEvent(
	message commonoutbox.Message,
	event contestcontract.AnnouncementChangedV1,
	eventType string,
	status string,
) error {
	if event.EventID == uuid.Nil || event.ContestID == uuid.Nil ||
		event.AnnouncementID == uuid.Nil || event.Version < 1 ||
		event.OccurredAt.IsZero() ||
		!validAnnouncementAudience(event.Audience) {
		return ErrInvalidEventDelivery
	}
	switch status {
	case "published":
		if eventType != contestcontract.EventAnnouncementPublishedV1 ||
			event.Revision < 1 || event.Title == "" ||
			len(event.Title) > contestentity.MaximumAnnouncementTitleBytes ||
			event.Title != strings.TrimSpace(event.Title) ||
			!utf8.ValidString(event.Title) {
			return ErrInvalidEventDelivery
		}
	case "withdrawn":
		if eventType != contestcontract.EventAnnouncementWithdrawnV1 ||
			event.Revision != 0 || event.Title != "" {
			return ErrInvalidEventDelivery
		}
	default:
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          event.EventID,
		Destination:      eventType,
		AggregateType:    contestcontract.AggregateAnnouncement,
		AggregateID:      event.AnnouncementID,
		AggregateVersion: event.Version,
		OccurredAt:       event.OccurredAt,
	})
}

func validateClarificationSubmittedEvent(
	message commonoutbox.Message,
	event contestcontract.ClarificationSubmittedV1,
) error {
	if event.EventID == uuid.Nil || event.ContestID == uuid.Nil ||
		event.ClarificationID == uuid.Nil || event.Version < 1 ||
		event.OccurredAt.IsZero() ||
		event.ContestProblemID != nil &&
			*event.ContestProblemID == uuid.Nil {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          event.EventID,
		Destination:      contestcontract.EventClarificationSubmittedV1,
		AggregateType:    contestcontract.AggregateClarification,
		AggregateID:      event.ClarificationID,
		AggregateVersion: event.Version,
		OccurredAt:       event.OccurredAt,
	})
}

func validateClarificationEvent(
	message commonoutbox.Message,
	event contestcontract.ClarificationChangedV1,
	eventType string,
) error {
	if event.EventID == uuid.Nil || event.ContestID == uuid.Nil ||
		event.ClarificationID == uuid.Nil || event.RequesterID == uuid.Nil ||
		event.Version < 1 || event.OccurredAt.IsZero() ||
		event.ContestProblemID != nil &&
			*event.ContestProblemID == uuid.Nil ||
		event.Audience != "requester" &&
			event.Audience != "participants" ||
		event.Shared != (event.Audience == "participants") {
		return ErrInvalidEventDelivery
	}
	switch eventType {
	case contestcontract.EventClarificationAnsweredV1,
		contestcontract.EventClarificationClosedV1,
		contestcontract.EventClarificationWithdrawnV1:
	default:
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          event.EventID,
		Destination:      eventType,
		AggregateType:    contestcontract.AggregateClarification,
		AggregateID:      event.ClarificationID,
		AggregateVersion: event.Version,
		OccurredAt:       event.OccurredAt,
	})
}

func announcementRealtimeAudience(
	value string,
) (realtime.ContestAudience, bool, error) {
	switch value {
	case "public":
		return realtime.ContestAudiencePublic, true, nil
	case "participants":
		return realtime.ContestAudienceParticipants, true, nil
	case "jury":
		return realtime.ContestAudienceStaff, false, nil
	default:
		return 0, false, ErrInvalidEventDelivery
	}
}

func clarificationRealtimeAudience(
	event contestcontract.ClarificationChangedV1,
) (realtime.ContestAudience, uuid.UUID, error) {
	switch event.Audience {
	case "participants":
		return realtime.ContestAudienceParticipants, uuid.Nil, nil
	case "requester":
		return realtime.ContestAudiencePrivateUser, event.RequesterID, nil
	default:
		return 0, uuid.Nil, ErrInvalidEventDelivery
	}
}

func publishContestInvalidation(
	ctx context.Context,
	publisher *realtime.HubPublisher,
	change realtime.ContestChange,
) error {
	if publisher == nil {
		return realtime.ErrInvalidConfig
	}
	ctx = logger.WithFields(
		ctx,
		logger.String("event_type", change.EventType),
		logger.String("contest_id", change.Event.ContestID.String()),
		logger.String("resource", string(change.Event.Resource)),
		logger.String("resource_id", change.Event.ResourceID.String()),
	)
	if err := publisher.PublishContest(ctx, change); err != nil {
		return err
	}
	logger.FromContext(ctx).Debug(
		"contest realtime invalidation published",
		zap.Int64("aggregate_version", change.Event.Version),
	)
	return nil
}

func classifyContestRealtimeFailure(err error) error {
	if errors.Is(err, realtime.ErrInvalidChange) ||
		errors.Is(err, realtime.ErrInvalidConfig) {
		return permanentEventFailure("contest_realtime_rejected", err)
	}
	return retryableEventFailure("contest_realtime_failed", err)
}

func validAnnouncementAudience(value string) bool {
	return value == "public" || value == "participants" || value == "jury"
}

func lifecycleStateIsPublic(value string) bool {
	return value == "upcoming" || value == "running" || value == "ended"
}
