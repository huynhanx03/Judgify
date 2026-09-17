package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const (
	problemAggregateType = problemcontract.AggregateProblem

	problemPublishedEventV1 = problemcontract.EventPublishedV1
	problemArchivedEventV1  = problemcontract.EventArchivedV1
)

type PublicationDependencies struct {
	Transactions platformtx.UnitOfWork
	Repository   ports.PublicationRepository
	Authorizer   ports.PublicationAuthorizer
	Catalog      ports.AuthoringCatalogFence
	Runtimes     ports.ExecutionCatalog
	Checkers     ports.CheckerCatalog
	Taxonomy     ports.TaxonomyCatalog
	Assets       entity.AssetVerifier
	Samples      entity.AssetDecoder
	Audit        audit.Appender
	Outbox       eventing.OutboxAppender
}

type PublicationOption func(*publicationService) error

func WithPublicationClock(clock func() time.Time) PublicationOption {
	return func(service *publicationService) error {
		if clock == nil {
			return ports.ErrInvalidPublication
		}
		service.clock = clock
		return nil
	}
}

func WithPublicationIDGenerator(generator func() uuid.UUID) PublicationOption {
	return func(service *publicationService) error {
		if generator == nil {
			return ports.ErrInvalidPublication
		}
		service.newID = generator
		return nil
	}
}

type publicationService struct {
	dependencies PublicationDependencies
	clock        func() time.Time
	newID        func() uuid.UUID
}

func NewPublicationService(
	dependencies PublicationDependencies,
	options ...PublicationOption,
) (ports.PublicationService, error) {
	if invalidPublicationDependencies(dependencies) {
		return nil, ports.ErrInvalidPublication
	}
	service := &publicationService{
		dependencies: dependencies,
		clock:        func() time.Time { return time.Now().UTC() },
		newID:        uuid.New,
	}
	for _, option := range options {
		if option == nil {
			return nil, ports.ErrInvalidPublication
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
}

var _ ports.PublicationService = (*publicationService)(nil)

type publishedEventPayload struct {
	ProblemID              string `json:"problem_id"`
	ProblemRevisionID      string `json:"problem_revision_id"`
	TestsetRevisionID      string `json:"testset_revision_id"`
	RenderSourceID         string `json:"render_source_id"`
	RenderArtifactID       string `json:"render_artifact_id"`
	ContentChecksum        string `json:"content_checksum"`
	TestsetChecksum        string `json:"testset_checksum"`
	RewardSnapshotChecksum string `json:"reward_snapshot_checksum"`
	Version                int64  `json:"version"`
}

type archivedEventPayload struct {
	ProblemID           string `json:"problem_id"`
	PublishedRevisionID string `json:"published_revision_id"`
	Version             int64  `json:"version"`
}

func (service *publicationService) Publish(
	ctx context.Context,
	command ports.PublishCommand,
) (ports.PublicationResult, error) {
	if service == nil || invalidPublicationDependencies(service.dependencies) ||
		service.clock == nil || service.newID == nil || ctx == nil {
		return ports.PublicationResult{}, ports.ErrInvalidPublication
	}
	if err := ctx.Err(); err != nil {
		return ports.PublicationResult{}, err
	}
	reason, err := validatePublicationCommand(command)
	if err != nil {
		return ports.PublicationResult{}, err
	}
	actorUserID, actorSessionID, err := publicationActor(ctx)
	if err != nil {
		return ports.PublicationResult{}, err
	}
	ctx = correlation.EnsureContext(ctx)

	logger.FromContext(ctx).Debug(
		"problem publication started",
		zap.String("problem_id", command.ProblemID.String()),
		zap.String("catalog_version", command.CatalogVersion),
		zap.String("problem_revision_id", command.DraftRevisionID.String()),
		zap.String("testset_revision_id", command.TestsetRevisionID.String()),
		zap.Int64("expected_version", command.ExpectedVersion),
	)

	var result ports.PublicationResult
	transactionErr := service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
		aggregate, err := service.dependencies.Repository.LockAggregate(txCtx, command.ProblemID)
		if err != nil {
			return fmt.Errorf("lock problem aggregate: %w", err)
		}
		if err := aggregate.Validate(); err != nil {
			return err
		}
		if aggregate.Version != command.ExpectedVersion {
			return ports.ErrPublicationConflict
		}
		if aggregate.Lifecycle == entity.LifecycleArchived {
			return entity.ErrProblemArchived
		}
		if err := service.dependencies.Authorizer.AuthorizePublish(txCtx, aggregate); err != nil {
			return errors.Join(ports.ErrPublicationDenied, err)
		}
		if err := service.dependencies.Catalog.ValidateAuthoringCatalogVersion(
			txCtx,
			command.CatalogVersion,
		); err != nil {
			return fmt.Errorf("validate publication authoring catalog version: %w", err)
		}

		candidate, err := service.dependencies.Repository.LoadPublicationCandidate(
			txCtx,
			aggregate,
			command.DraftRevisionID,
			command.TestsetRevisionID,
		)
		if err != nil {
			return fmt.Errorf("load publication candidate: %w", err)
		}
		if !sameProblemAggregate(candidate.Aggregate, aggregate) ||
			candidate.Revision.ID != command.DraftRevisionID ||
			candidate.Testset.ID != command.TestsetRevisionID {
			return entity.ErrInvalidPublicationCandidate
		}
		if err := candidate.Validate(); err != nil {
			return err
		}
		if candidate.Artifact.RendererVersion != contentrender.CurrentRendererVersion ||
			candidate.Artifact.SanitizerPolicyVersion != contentrender.CurrentSanitizerPolicyVersion {
			return entity.ErrInvalidPublicationCandidate
		}
		if err := candidate.Testset.VerifyAssets(txCtx, service.dependencies.Assets); err != nil {
			return fmt.Errorf("verify publication test assets: %w", err)
		}
		if _, err := entity.BuildPublicSamples(
			txCtx,
			candidate.Testset,
			service.dependencies.Samples,
		); err != nil {
			return fmt.Errorf("validate publication samples: %w", err)
		}
		if err := service.dependencies.Taxonomy.VerifyRewardSnapshot(txCtx, candidate.Revision.Reward); err != nil {
			return fmt.Errorf("validate publication taxonomy snapshot: %w", err)
		}
		if err := service.dependencies.Runtimes.ValidateRuntimeKeys(
			txCtx,
			append([]string(nil), candidate.Revision.AllowedRuntimeKeys...),
		); err != nil {
			return fmt.Errorf("validate publication runtimes: %w", err)
		}
		if err := service.dependencies.Checkers.ValidateChecker(
			txCtx,
			candidate.Revision.CheckerKey,
			candidate.Revision.CheckerVersion,
			append(json.RawMessage(nil), candidate.Revision.CheckerConfig...),
		); err != nil {
			return fmt.Errorf("validate publication checker: %w", err)
		}
		if err := service.dependencies.Runtimes.ValidateExecutionPairings(
			txCtx,
			candidate.Revision.AllowedRuntimeKeys,
			candidate.Revision.CheckerKey,
			candidate.Revision.CheckerVersion,
		); err != nil {
			return fmt.Errorf("validate publication execution capabilities: %w", err)
		}

		selection, err := candidate.Selection()
		if err != nil {
			return err
		}
		publishedAt, err := service.timestamp()
		if err != nil {
			return err
		}
		updated, err := service.dependencies.Repository.AdvancePublished(
			txCtx,
			command.ProblemID,
			command.ExpectedVersion,
			selection,
			publishedProblemProjection(candidate),
			publishedAt,
		)
		if err != nil {
			return fmt.Errorf("advance published problem selection: %w", err)
		}
		if err := validatePublishedResult(
			updated,
			aggregate,
			command,
			selection,
			publishedAt,
		); err != nil {
			return err
		}

		eventID, err := service.identifier()
		if err != nil {
			return err
		}
		payload := publishedEventPayload{
			ProblemID:              updated.ID.String(),
			ProblemRevisionID:      candidate.Revision.ID.String(),
			TestsetRevisionID:      candidate.Testset.ID.String(),
			RenderSourceID:         candidate.Source.ID.String(),
			RenderArtifactID:       candidate.Artifact.ID.String(),
			ContentChecksum:        hex.EncodeToString(candidate.Revision.ContentChecksum[:]),
			TestsetChecksum:        hex.EncodeToString(candidate.Testset.Checksum[:]),
			RewardSnapshotChecksum: hex.EncodeToString(candidate.Revision.Reward.Checksum[:]),
			Version:                updated.Version,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode problem publication event: %w", err)
		}
		if _, err := service.dependencies.Audit.Append(txCtx, audit.Entry{
			ID:             eventID,
			ActorUserID:    &actorUserID,
			ActorSessionID: &actorSessionID,
			Action:         problemPublishedEventV1,
			Resource:       problemAggregateType,
			ResourceID:     updated.ID.String(),
			Reason:         reason,
			Before: map[string]any{
				"lifecycle": string(aggregate.Lifecycle),
				"version":   aggregate.Version,
			},
			After: map[string]any{
				"lifecycle":           string(updated.Lifecycle),
				"version":             updated.Version,
				"problem_revision_id": candidate.Revision.ID.String(),
				"testset_revision_id": candidate.Testset.ID.String(),
				"render_source_id":    candidate.Source.ID.String(),
				"render_artifact_id":  candidate.Artifact.ID.String(),
			},
			CorrelationID: correlation.FromContext(txCtx),
			OccurredAt:    publishedAt,
		}); err != nil {
			return fmt.Errorf("append problem publication audit: %w", err)
		}
		if err := service.dependencies.Outbox.Append(txCtx, eventing.Event{
			ID:               eventID,
			Type:             problemPublishedEventV1,
			AggregateType:    problemAggregateType,
			AggregateID:      updated.ID,
			AggregateVersion: updated.Version,
			CorrelationID:    correlation.FromContext(txCtx),
			Payload:          payloadJSON,
			OccurredAt:       publishedAt,
		}); err != nil {
			return fmt.Errorf("append problem publication event: %w", err)
		}
		result = ports.PublicationResult{
			ProblemID:           updated.ID,
			PublishedRevisionID: candidate.Revision.ID,
			TestsetRevisionID:   candidate.Testset.ID,
			Version:             updated.Version,
			PublishedAt:         publishedAt,
		}
		return nil
	})
	if transactionErr != nil {
		logger.FromContext(ctx).Debug(
			"problem publication rejected",
			zap.String("problem_id", command.ProblemID.String()),
			zap.Int64("expected_version", command.ExpectedVersion),
			zap.Error(transactionErr),
		)
		return ports.PublicationResult{}, transactionErr
	}
	logger.FromContext(ctx).Info(
		"problem published",
		zap.String("problem_id", result.ProblemID.String()),
		zap.String("problem_revision_id", result.PublishedRevisionID.String()),
		zap.String("testset_revision_id", result.TestsetRevisionID.String()),
		zap.Int64("version", result.Version),
	)
	return result, nil
}

func publishedProblemProjection(
	candidate entity.PublicationCandidate,
) ports.PublishedProblemProjection {
	tagIDs := make([]uuid.UUID, len(candidate.Revision.Reward.Tags.Items))
	for index, item := range candidate.Revision.Reward.Tags.Items {
		tagIDs[index] = item.TagID
	}
	return ports.PublishedProblemProjection{
		RevisionID:        candidate.Revision.ID,
		Title:             candidate.Revision.Title,
		StatementMarkdown: string(candidate.Revision.StatementMarkdown),
		DifficultyID:      candidate.Revision.Reward.DifficultyID,
		TagIDs:            tagIDs,
		CPUTimeMs:         candidate.Revision.Limits.CPUTimeMilliseconds,
		MemoryLimitKb:     candidate.Revision.Limits.MemoryKilobytes,
	}
}

func (service *publicationService) Archive(
	ctx context.Context,
	command ports.ArchiveCommand,
) (entity.ProblemAggregate, error) {
	if service == nil || invalidPublicationDependencies(service.dependencies) ||
		service.clock == nil || service.newID == nil || ctx == nil {
		return entity.ProblemAggregate{}, ports.ErrInvalidPublication
	}
	if err := ctx.Err(); err != nil {
		return entity.ProblemAggregate{}, err
	}
	reason, err := validateArchiveCommand(command)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	actorUserID, actorSessionID, err := publicationActor(ctx)
	if err != nil {
		return entity.ProblemAggregate{}, err
	}
	ctx = correlation.EnsureContext(ctx)

	var archived entity.ProblemAggregate
	transactionErr := service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
		aggregate, err := service.dependencies.Repository.LockAggregate(txCtx, command.ProblemID)
		if err != nil {
			return fmt.Errorf("lock problem aggregate: %w", err)
		}
		if err := aggregate.Validate(); err != nil {
			return err
		}
		if aggregate.Version != command.ExpectedVersion {
			return ports.ErrPublicationConflict
		}
		if aggregate.Lifecycle == entity.LifecycleArchived {
			return entity.ErrProblemArchived
		}
		if aggregate.Published == nil {
			return ports.ErrInvalidPublication
		}
		if err := service.dependencies.Authorizer.AuthorizeArchive(txCtx, aggregate); err != nil {
			return errors.Join(ports.ErrPublicationDenied, err)
		}
		archivedAt, err := service.timestamp()
		if err != nil {
			return err
		}
		updated, err := service.dependencies.Repository.Archive(
			txCtx,
			command.ProblemID,
			command.ExpectedVersion,
			archivedAt,
		)
		if err != nil {
			return fmt.Errorf("archive problem: %w", err)
		}
		if !sameStableProblemAggregate(updated, aggregate) ||
			updated.Version != aggregate.Version+1 ||
			updated.Lifecycle != entity.LifecycleArchived ||
			!sameOptionalSelection(updated.Draft, aggregate.Draft) ||
			updated.Published == nil || *updated.Published != *aggregate.Published ||
			!sameOptionalTime(updated.PublishedAt, aggregate.PublishedAt) ||
			updated.ArchivedAt == nil || !updated.ArchivedAt.Equal(archivedAt) {
			return entity.ErrInvalidProblemAggregate
		}
		if err := updated.Validate(); err != nil {
			return err
		}
		eventID, err := service.identifier()
		if err != nil {
			return err
		}
		payloadJSON, err := json.Marshal(archivedEventPayload{
			ProblemID:           updated.ID.String(),
			PublishedRevisionID: updated.Published.RevisionID.String(),
			Version:             updated.Version,
		})
		if err != nil {
			return fmt.Errorf("encode problem archive event: %w", err)
		}
		if _, err := service.dependencies.Audit.Append(txCtx, audit.Entry{
			ID:             eventID,
			ActorUserID:    &actorUserID,
			ActorSessionID: &actorSessionID,
			Action:         problemArchivedEventV1,
			Resource:       problemAggregateType,
			ResourceID:     updated.ID.String(),
			Reason:         reason,
			Before: map[string]any{
				"lifecycle": string(aggregate.Lifecycle),
				"version":   aggregate.Version,
			},
			After: map[string]any{
				"lifecycle": string(updated.Lifecycle),
				"version":   updated.Version,
			},
			CorrelationID: correlation.FromContext(txCtx),
			OccurredAt:    archivedAt,
		}); err != nil {
			return fmt.Errorf("append problem archive audit: %w", err)
		}
		if err := service.dependencies.Outbox.Append(txCtx, eventing.Event{
			ID:               eventID,
			Type:             problemArchivedEventV1,
			AggregateType:    problemAggregateType,
			AggregateID:      updated.ID,
			AggregateVersion: updated.Version,
			CorrelationID:    correlation.FromContext(txCtx),
			Payload:          payloadJSON,
			OccurredAt:       archivedAt,
		}); err != nil {
			return fmt.Errorf("append problem archive event: %w", err)
		}
		archived = updated
		return nil
	})
	if transactionErr != nil {
		return entity.ProblemAggregate{}, transactionErr
	}
	logger.FromContext(ctx).Info(
		"problem archived",
		zap.String("problem_id", archived.ID.String()),
		zap.Int64("version", archived.Version),
	)
	return archived, nil
}

func validatePublicationCommand(command ports.PublishCommand) (string, error) {
	if command.ProblemID == uuid.Nil || command.DraftRevisionID == uuid.Nil ||
		command.TestsetRevisionID == uuid.Nil || command.ExpectedVersion < 1 ||
		!ports.IsAuthoringCatalogVersion(command.CatalogVersion) {
		return "", ports.ErrInvalidPublication
	}
	return normalizePublicationReason(command.Reason)
}

func validateArchiveCommand(command ports.ArchiveCommand) (string, error) {
	if command.ProblemID == uuid.Nil || command.ExpectedVersion < 1 {
		return "", ports.ErrInvalidPublication
	}
	return normalizePublicationReason(command.Reason)
}

func normalizePublicationReason(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || !utf8.ValidString(normalized) ||
		strings.IndexByte(normalized, 0) >= 0 ||
		len(normalized) > problemcontract.MaximumPublicationReasonBytes {
		return "", ports.ErrInvalidPublication
	}
	return normalized, nil
}

func publicationActor(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !userOK || !sessionOK || userID == uuid.Nil || sessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, ports.ErrPublicationDenied
	}
	return userID, sessionID, nil
}

func validatePublishedResult(
	aggregate entity.ProblemAggregate,
	previous entity.ProblemAggregate,
	command ports.PublishCommand,
	selection entity.RenderSelection,
	publishedAt time.Time,
) error {
	if aggregate.ID != command.ProblemID ||
		!sameStableProblemAggregate(aggregate, previous) ||
		aggregate.Version != command.ExpectedVersion+1 ||
		aggregate.Lifecycle != entity.LifecyclePublished ||
		!sameOptionalSelection(aggregate.Draft, previous.Draft) ||
		aggregate.Published == nil ||
		*aggregate.Published != selection ||
		aggregate.PublishedAt == nil ||
		!aggregate.PublishedAt.Equal(publishedAt) ||
		aggregate.ArchivedAt != nil {
		return entity.ErrInvalidProblemAggregate
	}
	return aggregate.Validate()
}

// sameStableProblemAggregate compares fields that lifecycle transitions are
// never allowed to mutate. UpdatedAt is intentionally excluded because Ent's
// time mixin advances it with the successful CAS update.
func sameStableProblemAggregate(
	current entity.ProblemAggregate,
	previous entity.ProblemAggregate,
) bool {
	return current.ID == previous.ID &&
		current.Slug == previous.Slug &&
		current.AuthorID == previous.AuthorID &&
		current.SubmissionCount == previous.SubmissionCount &&
		current.AcceptedCount == previous.AcceptedCount &&
		current.CreatedAt.Equal(previous.CreatedAt) &&
		!current.UpdatedAt.Before(previous.UpdatedAt)
}

func sameProblemAggregate(
	first entity.ProblemAggregate,
	second entity.ProblemAggregate,
) bool {
	return sameStableProblemAggregate(first, second) &&
		first.Lifecycle == second.Lifecycle &&
		first.Version == second.Version &&
		sameOptionalSelection(first.Draft, second.Draft) &&
		sameOptionalSelection(first.Published, second.Published) &&
		sameOptionalTime(first.PublishedAt, second.PublishedAt) &&
		sameOptionalTime(first.ArchivedAt, second.ArchivedAt) &&
		first.UpdatedAt.Equal(second.UpdatedAt)
}

func sameOptionalSelection(
	first *entity.RenderSelection,
	second *entity.RenderSelection,
) bool {
	switch {
	case first == nil || second == nil:
		return first == nil && second == nil
	default:
		return *first == *second
	}
}

func sameOptionalTime(first *time.Time, second *time.Time) bool {
	switch {
	case first == nil || second == nil:
		return first == nil && second == nil
	default:
		return first.Equal(*second)
	}
}

func (service *publicationService) timestamp() (time.Time, error) {
	value := service.clock().UTC()
	if value.IsZero() {
		return time.Time{}, ports.ErrInvalidPublication
	}
	return value, nil
}

func (service *publicationService) identifier() (uuid.UUID, error) {
	value := service.newID()
	if value == uuid.Nil {
		return uuid.Nil, ports.ErrInvalidPublication
	}
	return value, nil
}

func invalidPublicationDependencies(dependencies PublicationDependencies) bool {
	return nilDependency(dependencies.Transactions) ||
		nilDependency(dependencies.Repository) ||
		nilDependency(dependencies.Authorizer) ||
		nilDependency(dependencies.Catalog) ||
		nilDependency(dependencies.Runtimes) ||
		nilDependency(dependencies.Checkers) ||
		nilDependency(dependencies.Taxonomy) ||
		nilDependency(dependencies.Assets) ||
		nilDependency(dependencies.Samples) ||
		nilDependency(dependencies.Audit) ||
		nilDependency(dependencies.Outbox)
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
