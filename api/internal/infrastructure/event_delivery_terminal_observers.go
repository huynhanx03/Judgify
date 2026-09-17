package infrastructure

import (
	"context"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"
	"github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	contestentity "github.com/huynhanx03/judgify/internal/contest/domain"
	cultivationcontract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	materialcontract "github.com/huynhanx03/judgify/internal/material/contract"
	materialdto "github.com/huynhanx03/judgify/internal/material/contract"
	materialentity "github.com/huynhanx03/judgify/internal/material/domain"
	contentcontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

const (
	submissionWorkObservationHandler            = "observation.submission-work-state"
	contestAdministrationObservationHandler     = "observation.contest-administration"
	problemAuthoringObservationHandler          = "observation.problem-authoring"
	materialAuthoringObservationHandler         = "observation.material-authoring"
	cultivationAdministrationObservationHandler = "observation.cultivation-administration"
)

func cultivationAdministrationObservationRoutes() []commonoutbox.Route {
	return []commonoutbox.Route{
		strictJSONRoute(
			cultivationcontract.EventRankingReconciliationStartedV1,
			cultivationAdministrationObservationHandler,
			4<<10,
			validateRankingReconciliationStartedObservation,
			func(
				ctx context.Context,
				event cultivationcontract.RankingReconciliationStartedV1,
			) error {
				logger.FromContext(ctx).Debug(
					"cultivation ranking reconciliation start observed",
					zap.String("operation_id", event.OperationID.String()),
					zap.Int64("source_count", event.SourceCount),
					zap.Int64("repair_count", event.RepairCount),
					zap.Int64("projection_revision", event.ProjectionRevision),
				)
				return nil
			},
			nil,
		),
	}
}

func validateRankingReconciliationStartedObservation(
	message commonoutbox.Message,
	event cultivationcontract.RankingReconciliationStartedV1,
) error {
	if event.OperationID == uuid.Nil || event.SourceCutoff.IsZero() ||
		event.SourceCutoff.Location() != time.UTC ||
		event.SourceCount < 0 || event.RepairCount < 0 ||
		event.RepairCount > event.SourceCount || event.ProjectionRevision < 0 {
		return ErrInvalidEventDelivery
	}
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil || messageID != cultivationcontract.RankingReconciliationStartedEventID(event.OperationID) {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		cultivationcontract.EventRankingReconciliationStartedV1,
		cultivationcontract.AggregateRankingReconciliation,
		event.OperationID,
		1,
		"",
		time.Time{},
	)
}

// These routes are intentional terminal sinks. Their mutations, audit records,
// and durable facts already commit atomically; no local projection remains to
// apply. Keeping them in the closed route catalog still gives malformed facts
// a permanent DLQ outcome and gives operators a CID-linked delivery trace.
func submissionWorkObservationRoutes() []commonoutbox.Route {
	return []commonoutbox.Route{
		strictJSONRoute(
			submissioncontract.EventSubmissionQueuedV1,
			submissionWorkObservationHandler,
			4<<10,
			validateSubmissionQueuedObservation,
			func(ctx context.Context, event submissioncontract.SubmissionQueuedV1) error {
				logger.FromContext(ctx).Debug(
					"submission work-state fact observed",
					zap.String("event_type", submissioncontract.EventSubmissionQueuedV1),
					zap.String("submission_id", event.SubmissionID.String()),
					zap.String("job_id", event.JobID.String()),
					zap.Int64("generation", event.Generation),
				)
				return nil
			},
			nil,
		),
		strictJSONRoute(
			submissioncontract.EventSubmissionCancelledV1,
			submissionWorkObservationHandler,
			4<<10,
			validateSubmissionCancelledObservation,
			func(ctx context.Context, event submissioncontract.SubmissionCancelledV1) error {
				logger.FromContext(ctx).Debug(
					"submission work-state fact observed",
					zap.String("event_type", submissioncontract.EventSubmissionCancelledV1),
					zap.String("submission_id", event.SubmissionID.String()),
					zap.String("job_id", event.JobID.String()),
					zap.Int64("generation", event.Generation),
				)
				return nil
			},
			nil,
		),
	}
}

func validateSubmissionQueuedObservation(
	message commonoutbox.Message,
	event submissioncontract.SubmissionQueuedV1,
) error {
	queuedAt, err := time.Parse(time.RFC3339Nano, event.QueuedAt)
	if err != nil || queuedAt.IsZero() ||
		event.SubmissionID == uuid.Nil || event.Generation < 1 ||
		event.JudgeSpecID == uuid.Nil || event.JobID == uuid.Nil ||
		!validSubmissionLane(event.Lane) {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		submissioncontract.EventSubmissionQueuedV1,
		submissioncontract.AggregateSubmission,
		event.SubmissionID,
		0,
		event.CID,
		queuedAt,
	)
}

func validateSubmissionCancelledObservation(
	message commonoutbox.Message,
	event submissioncontract.SubmissionCancelledV1,
) error {
	occurredAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil || occurredAt.IsZero() ||
		event.SubmissionID == uuid.Nil || event.Generation < 1 ||
		event.JobID == uuid.Nil || event.Phase != "cancelled" {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		submissioncontract.EventSubmissionCancelledV1,
		submissioncontract.AggregateSubmission,
		event.SubmissionID,
		0,
		event.CID,
		occurredAt,
	)
}

func validSubmissionLane(value string) bool {
	switch value {
	case "contest", "regular", "maintenance":
		return true
	default:
		return false
	}
}

func contestAdministrationObservationRoutes() []commonoutbox.Route {
	routes := make([]commonoutbox.Route, 0, 4)
	for _, eventType := range []string{
		contestcontract.EventDraftCreatedV1,
		contestcontract.EventDraftUpdatedV1,
		contestcontract.EventDraftDeletedV1,
	} {
		eventType := eventType
		routes = append(routes, strictJSONRoute(
			eventType,
			contestAdministrationObservationHandler,
			8<<10,
			func(message commonoutbox.Message, event contestcontract.DraftChangedV1) error {
				return validateContestDraftObservation(message, eventType, event)
			},
			func(ctx context.Context, event contestcontract.DraftChangedV1) error {
				logger.FromContext(ctx).Debug(
					"contest administration fact observed",
					zap.String("event_type", eventType),
					zap.String("contest_id", event.ID.String()),
					zap.String("state", event.State),
					zap.Int64("aggregate_version", event.Version),
				)
				return nil
			},
			nil,
		))
	}
	routes = append(routes, strictJSONRoute(
		contestcontract.EventRegistrationChangedV1,
		contestAdministrationObservationHandler,
		4<<10,
		validateContestRegistrationObservation,
		func(ctx context.Context, event contestcontract.RegistrationChangedV1) error {
			logger.FromContext(ctx).Debug(
				"contest registration fact observed",
				zap.String("contest_id", event.ContestID.String()),
				zap.String("registration_id", event.RegistrationID.String()),
				zap.String("user_id", event.UserID.String()),
				zap.String("status", event.Status),
				zap.Int64("aggregate_version", event.Version),
			)
			return nil
		},
		nil,
	))
	return routes
}

func validateContestDraftObservation(
	message commonoutbox.Message,
	eventType string,
	event contestcontract.DraftChangedV1,
) error {
	if event.ID == uuid.Nil || event.Version < 1 {
		return ErrInvalidEventDelivery
	}
	switch eventType {
	case contestcontract.EventDraftCreatedV1,
		contestcontract.EventDraftUpdatedV1:
		if event.State != string(contestentity.ContestDraft) ||
			event.AuthorID == uuid.Nil || event.StartTime.IsZero() ||
			event.EndTime.IsZero() || !event.EndTime.After(event.StartTime) ||
			!contestentity.RegistrationMode(event.RegistrationMode).Valid() ||
			event.ProblemCount == nil || *event.ProblemCount < 1 ||
			event.RatingAlgorithmVersion != "expected-rank-v1" {
			return ErrInvalidEventDelivery
		}
	case contestcontract.EventDraftDeletedV1:
		if event.State != "deleted" || event.AuthorID != uuid.Nil ||
			!event.StartTime.IsZero() || !event.EndTime.IsZero() ||
			event.RegistrationMode != "" || event.ProblemCount != nil ||
			event.Rated || event.RatingAlgorithmVersion != "" {
			return ErrInvalidEventDelivery
		}
	default:
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		contestcontract.AggregateContest,
		event.ID,
		event.Version,
		"",
		time.Time{},
	)
}

func validateContestRegistrationObservation(
	message commonoutbox.Message,
	event contestcontract.RegistrationChangedV1,
) error {
	if event.ContestID == uuid.Nil || event.RegistrationID == uuid.Nil ||
		event.UserID == uuid.Nil || event.Version < 1 ||
		event.OccurredAt.IsZero() ||
		!contestentity.RegistrationStatus(event.Status).Valid() ||
		!contestentity.RegistrationSource(event.Source).Valid() {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		contestcontract.EventRegistrationChangedV1,
		contestcontract.AggregateRegistration,
		event.RegistrationID,
		event.Version,
		"",
		event.OccurredAt,
	)
}

func problemAuthoringObservationRoutes() []commonoutbox.Route {
	return []commonoutbox.Route{
		strictJSONRoute(
			problemcontract.EventDraftSavedV1,
			problemAuthoringObservationHandler,
			8<<10,
			validateProblemDraftObservation,
			func(ctx context.Context, event problemcontract.DraftSavedV1) error {
				logProblemObservation(ctx, problemcontract.EventDraftSavedV1, event.ProblemID, event.Version)
				return nil
			},
			nil,
		),
		strictJSONRoute(
			problemcontract.EventPublishedV1,
			problemAuthoringObservationHandler,
			8<<10,
			validateProblemPublishedObservation,
			func(ctx context.Context, event problemcontract.PublishedV1) error {
				logProblemObservation(ctx, problemcontract.EventPublishedV1, event.ProblemID, event.Version)
				return nil
			},
			nil,
		),
		strictJSONRoute(
			problemcontract.EventArchivedV1,
			problemAuthoringObservationHandler,
			4<<10,
			validateProblemArchivedObservation,
			func(ctx context.Context, event problemcontract.ArchivedV1) error {
				logProblemObservation(ctx, problemcontract.EventArchivedV1, event.ProblemID, event.Version)
				return nil
			},
			nil,
		),
	}
}

func validateProblemDraftObservation(
	message commonoutbox.Message,
	event problemcontract.DraftSavedV1,
) error {
	if event.ProblemID == uuid.Nil || event.ProblemRevisionID == uuid.Nil ||
		event.TestsetRevisionID == uuid.Nil || event.RenderSourceID == uuid.Nil ||
		event.RenderArtifactID == uuid.Nil || event.TestCount < 1 ||
		event.Version < 1 ||
		(event.ArtifactStatus != contentcontract.StatusReady &&
			event.ArtifactStatus != contentcontract.StatusRejected) ||
		!validSHA256Hex(event.ContentChecksum) ||
		!validSHA256Hex(event.TestsetChecksum) ||
		!validSHA256Hex(event.RewardSnapshotChecksum) {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		problemcontract.EventDraftSavedV1,
		problemcontract.AggregateProblem,
		event.ProblemID,
		event.Version,
		"",
		time.Time{},
	)
}

func validateProblemPublishedObservation(
	message commonoutbox.Message,
	event problemcontract.PublishedV1,
) error {
	if event.ProblemID == uuid.Nil || event.ProblemRevisionID == uuid.Nil ||
		event.TestsetRevisionID == uuid.Nil || event.RenderSourceID == uuid.Nil ||
		event.RenderArtifactID == uuid.Nil || event.Version < 1 ||
		!validSHA256Hex(event.ContentChecksum) ||
		!validSHA256Hex(event.TestsetChecksum) ||
		!validSHA256Hex(event.RewardSnapshotChecksum) {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		problemcontract.EventPublishedV1,
		problemcontract.AggregateProblem,
		event.ProblemID,
		event.Version,
		"",
		time.Time{},
	)
}

func validateProblemArchivedObservation(
	message commonoutbox.Message,
	event problemcontract.ArchivedV1,
) error {
	if event.ProblemID == uuid.Nil || event.PublishedRevisionID == uuid.Nil ||
		event.Version < 1 {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		problemcontract.EventArchivedV1,
		problemcontract.AggregateProblem,
		event.ProblemID,
		event.Version,
		"",
		time.Time{},
	)
}

func logProblemObservation(
	ctx context.Context,
	eventType string,
	problemID uuid.UUID,
	version int64,
) {
	logger.FromContext(ctx).Debug(
		"problem authoring fact observed",
		zap.String("event_type", eventType),
		zap.String("problem_id", problemID.String()),
		zap.Int64("aggregate_version", version),
	)
}

func materialAuthoringObservationRoutes() []commonoutbox.Route {
	routes := make([]commonoutbox.Route, 0, 6)
	for _, eventType := range []string{
		materialcontract.EventDraftCreatedV1,
		materialcontract.EventDraftSavedV1,
		materialcontract.EventPublishedV1,
		materialcontract.EventRolledBackV1,
		materialcontract.EventArchivedV1,
		materialcontract.EventSlugChangedV1,
	} {
		eventType := eventType
		routes = append(routes, strictJSONRoute(
			eventType,
			materialAuthoringObservationHandler,
			96<<10,
			func(message commonoutbox.Message, event materialcontract.ChangedV1) error {
				return validateMaterialAuthoringObservation(message, eventType, event)
			},
			func(ctx context.Context, event materialcontract.ChangedV1) error {
				logger.FromContext(ctx).Debug(
					"material authoring fact observed",
					zap.String("event_type", eventType),
					zap.String("material_id", event.MaterialID.String()),
					zap.Int64("aggregate_version", event.Version),
				)
				return nil
			},
			nil,
		))
	}
	return routes
}

type materialRevisionEvidence struct {
	MaterialID       uuid.UUID `json:"material_id"`
	Version          int64     `json:"version"`
	Status           string    `json:"status"`
	Visibility       string    `json:"visibility"`
	RevisionID       uuid.UUID `json:"revision_id"`
	RevisionNumber   int64     `json:"revision_number"`
	RenderSourceID   uuid.UUID `json:"render_source_id"`
	RenderArtifactID uuid.UUID `json:"render_artifact_id"`
}

type materialArchiveEvidence struct {
	Version int64  `json:"version"`
	Status  string `json:"status"`
}

type materialSlugEvidence struct {
	Version int64  `json:"version"`
	Slug    string `json:"slug"`
}

func validateMaterialAuthoringObservation(
	message commonoutbox.Message,
	eventType string,
	event materialcontract.ChangedV1,
) error {
	if event.MaterialID == uuid.Nil || event.Version < 1 ||
		len(event.After) == 0 {
		return ErrInvalidEventDelivery
	}
	switch eventType {
	case materialcontract.EventDraftCreatedV1,
		materialcontract.EventDraftSavedV1,
		materialcontract.EventPublishedV1,
		materialcontract.EventRolledBackV1:
		var evidence materialRevisionEvidence
		if commonjson.UnmarshalStrictLimit(event.After, &evidence, 16<<10) != nil ||
			evidence.MaterialID != event.MaterialID ||
			evidence.Version != event.Version ||
			evidence.RevisionID == uuid.Nil || evidence.RevisionNumber < 1 ||
			evidence.RenderSourceID == uuid.Nil ||
			evidence.RenderArtifactID == uuid.Nil ||
			!materialentity.MaterialState(evidence.Status).Valid() ||
			!materialentity.MaterialVisibility(evidence.Visibility).Valid() {
			return ErrInvalidEventDelivery
		}
		if (eventType == materialcontract.EventPublishedV1 ||
			eventType == materialcontract.EventRolledBackV1) &&
			evidence.Status != string(materialentity.MaterialPublished) {
			return ErrInvalidEventDelivery
		}
	case materialcontract.EventArchivedV1:
		var evidence materialArchiveEvidence
		if commonjson.UnmarshalStrictLimit(event.After, &evidence, 2<<10) != nil ||
			evidence.Version != event.Version ||
			evidence.Status != string(materialentity.MaterialArchived) {
			return ErrInvalidEventDelivery
		}
	case materialcontract.EventSlugChangedV1:
		var evidence materialSlugEvidence
		if commonjson.UnmarshalStrictLimit(event.After, &evidence, 2<<10) != nil {
			return ErrInvalidEventDelivery
		}
		normalized, err := materialentity.NormalizeSlug(evidence.Slug)
		if err != nil || normalized != evidence.Slug ||
			evidence.Version != event.Version {
			return ErrInvalidEventDelivery
		}
	default:
		return ErrInvalidEventDelivery
	}
	if eventType == materialcontract.EventRolledBackV1 {
		if !validSHA256Hex(event.RequestHash) || len(event.Result) == 0 {
			return ErrInvalidEventDelivery
		}
		var result materialdto.MaterialResponse
		if commonjson.UnmarshalStrictLimit(event.Result, &result, 64<<10) != nil ||
			result.ID != event.MaterialID || result.Version != event.Version ||
			result.Content != "" || result.SanitizedHTML != "" ||
			!materialentity.MaterialState(result.Status).Valid() ||
			!materialentity.MaterialVisibility(result.Visibility).Valid() {
			return ErrInvalidEventDelivery
		}
	} else if event.RequestHash != "" || len(event.Result) != 0 {
		return ErrInvalidEventDelivery
	}
	return validateObservedEnvelope(
		message,
		eventType,
		materialcontract.AggregateMaterial,
		event.MaterialID,
		event.Version,
		"",
		time.Time{},
	)
}

func validateObservedEnvelope(
	message commonoutbox.Message,
	destination string,
	aggregateType string,
	aggregateID uuid.UUID,
	aggregateVersion int64,
	correlationID string,
	occurredAt time.Time,
) error {
	messageID, err := canonicalMessageUUID(message.ID)
	if err != nil {
		return ErrInvalidEventDelivery
	}
	versionText, err := uniqueMessageAttribute(message, "aggregate_version")
	if err != nil {
		return ErrInvalidEventDelivery
	}
	envelopeVersion, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || envelopeVersion < 1 ||
		aggregateVersion > 0 && envelopeVersion != aggregateVersion {
		return ErrInvalidEventDelivery
	}
	return validateEventEnvelope(message, eventEnvelopeExpectation{
		EventID:          messageID,
		Destination:      destination,
		AggregateType:    aggregateType,
		AggregateID:      aggregateID,
		AggregateVersion: envelopeVersion,
		CorrelationID:    correlationID,
		OccurredAt:       occurredAt,
	})
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 &&
		hex.EncodeToString(decoded) == value
}
