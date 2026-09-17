package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	notificationcontract "github.com/huynhanx03/judgify/internal/notification/contract"
	notificationservice "github.com/huynhanx03/judgify/internal/notification/usecase"
	contestports "github.com/huynhanx03/judgify/internal/ports"
	notificationports "github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/realtime"
)

const (
	ratingProjectionHandler        = "contest.rating-projections"
	maximumRatingEventParticipants = 10000
)

func eventRatingRoute(
	ratingService contestports.RatingService,
	notifications *notificationservice.Consumer,
	publisher *realtime.HubPublisher,
) (commonoutbox.Route, error) {
	if ratingService == nil || notifications == nil || publisher == nil {
		return commonoutbox.Route{}, ErrInvalidEventDelivery
	}
	return strictJSONRoute(
		contestcontract.EventRatingCalculatedV1,
		ratingProjectionHandler,
		8<<10,
		validateRatingCalculatedEvent,
		func(
			ctx context.Context,
			event contestcontract.RatingCalculatedV1,
		) error {
			changes, err := ratingService.GetContestRatingChanges(
				ctx,
				event.ContestID,
			)
			if err != nil {
				return err
			}
			if len(changes) != event.ParticipantCount {
				return permanentEventFailure(
					"rating_result_cardinality_mismatch",
					ErrInvalidEventDelivery,
				)
			}
			titleKey := notificationcontract.MessageKeyContestRatingTitle
			bodyKey := notificationcontract.MessageKeyContestRatingBody
			actionPath := "/contest/" + event.ContestID.String()
			for _, change := range changes {
				if change == nil || change.UserID == uuid.Nil ||
					change.NewRating < 0 ||
					change.NewRating-change.OldRating != change.Delta {
					return permanentEventFailure(
						"invalid_rating_result",
						ErrInvalidEventDelivery,
					)
				}
				params, marshalErr := json.Marshal(map[string]any{
					"contest_id":   event.ContestID.String(),
					"rank":         change.Rank,
					"old_rating":   change.OldRating,
					"new_rating":   change.NewRating,
					"rating_delta": fmt.Sprintf("%+d", change.Delta),
				})
				if marshalErr != nil {
					return permanentEventFailure(
						"notification_payload_failed",
						marshalErr,
					)
				}
				if _, _, consumeErr := notifications.Consume(
					ctx,
					notificationports.CreateCommand{
						UserID:        change.UserID,
						SourceEventID: event.EventID,
						Type:          contestcontract.EventRatingCalculatedV1,
						Category:      notificationcontract.CategoryContest,
						TitleKey:      &titleKey,
						BodyKey:       &bodyKey,
						Params:        params,
						ActionPath:    &actionPath,
						OccurredAt:    event.OccurredAt.UTC(),
					},
				); consumeErr != nil {
					return consumeErr
				}
			}
			if err := publishContestInvalidation(
				ctx,
				publisher,
				realtime.ContestChange{
					EventID:      event.EventID,
					EventType:    contestcontract.EventRatingCalculatedV1,
					Audience:     realtime.ContestAudiencePublic,
					IncludeStaff: true,
					Event: realtime.ContestInvalidationV1{
						ContestID:  event.ContestID,
						Resource:   realtime.ContestResourceRating,
						ResourceID: event.RatingRunID,
						Status:     "applied",
						Version:    event.RunVersion,
						ChangedAt:  event.OccurredAt.UTC(),
					},
				},
			); err != nil {
				return err
			}
			logger.FromContext(ctx).Debug(
				"rating projections consumed",
				zap.String("contest_id", event.ContestID.String()),
				zap.String(
					"rating_run_id",
					event.RatingRunID.String(),
				),
				zap.Int("participants", event.ParticipantCount),
			)
			return nil
		},
		classifyNotificationFailure,
	), nil
}

func validateRatingCalculatedEvent(
	message commonoutbox.Message,
	event contestcontract.RatingCalculatedV1,
) error {
	if event.EventID == uuid.Nil || event.RatingRunID == uuid.Nil ||
		event.ContestID == uuid.Nil || event.RatingSequence < 1 ||
		event.RunVersion < 1 ||
		event.StandingsProjectionVersion < 1 ||
		event.AlgorithmVersion != "expected-rank-v1" ||
		event.ParticipantCount < 1 ||
		event.ParticipantCount > maximumRatingEventParticipants ||
		event.OccurredAt.IsZero() {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          event.EventID,
		Destination:      contestcontract.EventRatingCalculatedV1,
		AggregateType:    contestcontract.AggregateRatingRun,
		AggregateID:      event.RatingRunID,
		AggregateVersion: event.RunVersion,
		OccurredAt:       event.OccurredAt,
	})
}

func eventRatingProjectionActivatedRoute(
	database *internalent.EntClient,
	publisher *realtime.HubPublisher,
) (commonoutbox.Route, error) {
	if database == nil || publisher == nil {
		return commonoutbox.Route{}, ErrInvalidEventDelivery
	}
	return strictJSONRoute(
		contestcontract.EventRatingProjectionActivatedV1,
		ratingProjectionHandler,
		8<<10,
		validateRatingProjectionActivatedEvent,
		func(
			ctx context.Context,
			event contestcontract.RatingProjectionActivatedV1,
		) error {
			rows, err := database.DB(ctx).Contest.QueryContext(ctx, `
				SELECT rating_run.contest_id
				FROM rating_run_chain_entries AS chain_entry
				JOIN contest_rating_runs AS rating_run
				  ON rating_run.id = chain_entry.rating_run_id
				WHERE chain_entry.chain_id = $1
				  AND chain_entry.rating_sequence >= $2
				  AND chain_entry.rating_sequence <= $3
				ORDER BY chain_entry.rating_sequence ASC`,
				event.RunChainID,
				event.FromSequence,
				event.ThroughSequence,
			)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()
			contestIDs := make(
				[]uuid.UUID,
				0,
				int(event.ThroughSequence-event.FromSequence+1),
			)
			for rows.Next() {
				var contestID uuid.UUID
				if err := rows.Scan(&contestID); err != nil {
					return err
				}
				if contestID == uuid.Nil {
					return permanentEventFailure(
						"invalid_rerated_contest",
						ErrInvalidEventDelivery,
					)
				}
				contestIDs = append(contestIDs, contestID)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			expected := event.ThroughSequence -
				event.FromSequence + 1
			if int64(len(contestIDs)) != expected {
				return permanentEventFailure(
					"rating_rerating_cardinality_mismatch",
					sql.ErrNoRows,
				)
			}
			for _, contestID := range contestIDs {
				if err := publishContestInvalidation(
					ctx,
					publisher,
					realtime.ContestChange{
						EventID:      event.EventID,
						EventType:    contestcontract.EventRatingProjectionActivatedV1,
						Audience:     realtime.ContestAudiencePublic,
						IncludeStaff: true,
						Event: realtime.ContestInvalidationV1{
							ContestID:  contestID,
							Resource:   realtime.ContestResourceRating,
							ResourceID: event.ProjectionGenerationID,
							Status:     "applied",
							Version:    event.ProjectionRevision,
							ChangedAt:  event.OccurredAt.UTC(),
						},
					},
				); err != nil {
					return err
				}
			}
			logger.FromContext(ctx).Info(
				"historical rating activation projected",
				zap.String(
					"operation_id",
					event.OperationID.String(),
				),
				zap.Int64(
					"projection_revision",
					event.ProjectionRevision,
				),
				zap.Int("affected_contests", len(contestIDs)),
			)
			return nil
		},
		classifyContestRealtimeFailure,
	), nil
}

func validateRatingProjectionActivatedEvent(
	message commonoutbox.Message,
	event contestcontract.RatingProjectionActivatedV1,
) error {
	if event.EventID == uuid.Nil ||
		event.OperationID == uuid.Nil ||
		event.TriggerContestID == uuid.Nil ||
		event.FromSequence < 1 ||
		event.ThroughSequence < event.FromSequence ||
		event.ThroughSequence-event.FromSequence+1 >
			contestcontract.MaximumRatingReratingSequences ||
		event.ProjectionRevision < 2 ||
		event.ProjectionGenerationID == uuid.Nil ||
		event.RunChainID == uuid.Nil ||
		event.AlgorithmVersion != "expected-rank-v1" ||
		event.OccurredAt.IsZero() {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(
		message,
		eventEnvelopeExpectation{
			EventID:          event.EventID,
			Destination:      contestcontract.EventRatingProjectionActivatedV1,
			AggregateType:    contestcontract.AggregateRatingProjection,
			AggregateID:      event.ProjectionGenerationID,
			AggregateVersion: event.ProjectionRevision,
			OccurredAt:       event.OccurredAt,
		},
	)
}
