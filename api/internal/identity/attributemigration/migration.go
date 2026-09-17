// Package attributemigration implements the bounded, resumable shadow-value
// half of an attribute-definition activation. The lifecycle workflow owns the
// final audited pointer change; this package never advances a definition by
// itself.
package attributemigration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/operation"
)

const OperationKind = "identity.attribute_value_migration.v1"

// DefaultBatchSize bounds both lock duration and memory usage for the
// single-host operation runner. It is domain-owned because a "batch" here is
// a set of typed profile values, not a generic operation concern.
const DefaultBatchSize = 100

var requestHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var (
	ErrInvalidMigration = errors.New("identity attribute migration: invalid migration")
	ErrMigrationActive  = errors.New("identity attribute migration: another migration is active")
	ErrDefinitionDrift  = errors.New("identity attribute migration: definition drift")
	ErrValueDrift       = errors.New("identity attribute migration: value drift")
)

// Request is the exact replay-bound durable operation payload. TargetCutoff
// and SourceCutoff are UUIDv7 high-water marks, never wall-clock timestamps,
// so rows created at the same instant cannot be skipped or duplicated.
type Request struct {
	OperationID         uuid.UUID `json:"operation_id"`
	CommandID           uuid.UUID `json:"command_id"`
	DefinitionID        uuid.UUID `json:"definition_id"`
	ExpectedVersion     int64     `json:"expected_version"`
	SourceRevisionID    uuid.UUID `json:"source_revision_id"`
	CandidateRevisionID uuid.UUID `json:"candidate_revision_id"`
	ActorUserID         uuid.UUID `json:"actor_user_id"`
	ActorSessionID      uuid.UUID `json:"actor_session_id"`
	Reason              string    `json:"reason"`
	ConfirmationHash    string    `json:"confirmation_hash"`
	TargetCutoff        uuid.UUID `json:"target_cutoff"`
	TargetCount         int64     `json:"target_count"`
	SourceCutoff        uuid.UUID `json:"source_cutoff"`
	SourceCount         int64     `json:"source_count"`
}

type Phase string

const (
	PhaseCleanupTarget Phase = "cleanup_target"
	PhaseCopySource    Phase = "copy_source"
)

type Checkpoint struct {
	Phase              Phase     `json:"phase,omitempty"`
	AfterTargetValueID uuid.UUID `json:"after_target_value_id,omitempty"`
	TargetProcessed    int64     `json:"target_processed"`
	AfterSourceValueID uuid.UUID `json:"after_source_value_id,omitempty"`
	SourceProcessed    int64     `json:"source_processed"`
	Quarantined        int64     `json:"quarantined"`
}

type SourceValue struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DefinitionID   uuid.UUID
	RevisionID     uuid.UUID
	CanonicalValue json.RawMessage
	LegacyValue    string
	Version        int64
}

type ShadowValue struct {
	UserID              uuid.UUID
	DefinitionID        uuid.UUID
	SourceRevisionID    uuid.UUID
	CandidateRevisionID uuid.UUID
	CanonicalValue      json.RawMessage
	LegacyValue         string
	SourceValueVersion  int64
}

type Batch struct {
	Source    entity.AttributeDefinitionRevision
	Candidate entity.AttributeDefinitionRevision
	Values    []SourceValue
	Done      bool
}

type CleanupBatch struct {
	ValueIDs []uuid.UUID
	Done     bool
}

// Repository methods run only inside the operation runner's UnitOfWork.
// CleanupTargetBatch must atomically inspect every returned active target row
// and retain it only when it exactly matches a current source row by user,
// definition, source revision, source version, and present canonical value.
// Every other returned row must be soft-deleted. Returned IDs are ordered,
// greater than the checkpoint cursor, at or below TargetCutoff, and bounded by
// limit. Implementations must also lock the definition before final
// verification and preserve candidate rows with a source version newer than a
// stale backfill.
type Repository interface {
	Prepare(context.Context, Request) (Request, error)
	CleanupTargetBatch(context.Context, Request, Checkpoint, int) (CleanupBatch, error)
	LoadBatch(context.Context, Request, Checkpoint, int) (Batch, error)
	PutShadow(context.Context, ShadowValue) error
	VerifyFinal(context.Context, Request) error
}

// CoordinatorRepository adds the lifecycle-facing lookup used to prevent a
// new draft from invalidating an in-flight candidate. The executor itself
// intentionally depends only on Repository.
type CoordinatorRepository interface {
	Repository
	ActiveMigrationTarget(context.Context, *entity.AttributeDefinition) (*Request, error)
}

// Activator is supplied by the attribute lifecycle workflow. It performs the
// short, audited active-revision pointer switch in the same transaction after
// VerifyFinal holds the definition write lock. Keeping it separate prevents a
// worker from bypassing the workflow's audit/outbox invariant.
type Activator interface {
	ActivateMigratedRevision(context.Context, Request) error
}

type Executor struct {
	repository Repository
	activator  Activator
}

func NewExecutor(repository Repository, activator Activator) (*Executor, error) {
	if repository == nil || activator == nil {
		return nil, ErrInvalidMigration
	}
	return &Executor{repository: repository, activator: activator}, nil
}

func (executor *Executor) Register(registry *operation.Registry) error {
	if executor == nil || registry == nil {
		return operation.ErrInvalidExecutor
	}
	return operation.RegisterJSON(registry, OperationKind, executor.executeBatch)
}

func (executor *Executor) executeBatch(
	ctx context.Context,
	request Request,
	input operation.BatchInput,
) (operation.BatchResult, error) {
	if executor == nil || executor.repository == nil || executor.activator == nil ||
		ctx == nil || ctx.Err() != nil || input.OperationID != request.OperationID ||
		input.Sequence < 1 || input.Limit < 1 || !validRequest(request) {
		return operation.BatchResult{}, operation.Terminal("invalid_attribute_migration", ErrInvalidMigration)
	}
	checkpoint, err := decodeCheckpoint(input.Checkpoint)
	if err != nil {
		return operation.BatchResult{}, operation.Terminal("invalid_attribute_migration_checkpoint", ErrInvalidMigration)
	}
	checkpoint, valid := normalizeCheckpoint(request, checkpoint)
	if !valid {
		return operation.BatchResult{}, operation.Terminal("invalid_attribute_migration_checkpoint", ErrInvalidMigration)
	}
	if checkpoint.Phase == PhaseCleanupTarget {
		return executor.executeCleanupBatch(ctx, request, checkpoint, input.Limit)
	}
	return executor.executeSourceBatch(ctx, request, checkpoint, input.Limit)
}

func (executor *Executor) executeCleanupBatch(
	ctx context.Context,
	request Request,
	checkpoint Checkpoint,
	limit int,
) (operation.BatchResult, error) {
	batch, err := executor.repository.CleanupTargetBatch(ctx, request, checkpoint, limit)
	if err != nil {
		return operation.BatchResult{}, classifyRepositoryError(err)
	}
	if !validCleanupBatch(request, checkpoint, batch, limit) {
		return operation.BatchResult{}, operation.Terminal("invalid_attribute_migration_cleanup_batch", ErrInvalidMigration)
	}
	processed := int64(len(batch.ValueIDs))
	checkpoint.TargetProcessed += processed
	checkpoint.AfterTargetValueID = nextValueID(checkpoint.AfterTargetValueID, batch.ValueIDs)
	done := false
	if batch.Done {
		if checkpoint.TargetProcessed != request.TargetCount {
			return operation.BatchResult{}, operation.Terminal("attribute_migration_target_count_drift", ErrValueDrift)
		}
		checkpoint.Phase = PhaseCopySource
		if request.SourceCount == 0 {
			if err := executor.verifyAndActivate(ctx, request); err != nil {
				return operation.BatchResult{}, err
			}
			done = true
		}
	}
	return migrationBatchResult(checkpoint, processed, processed, nil, done)
}

func (executor *Executor) executeSourceBatch(
	ctx context.Context,
	request Request,
	checkpoint Checkpoint,
	limit int,
) (operation.BatchResult, error) {
	if request.SourceCount == 0 {
		if err := executor.verifyAndActivate(ctx, request); err != nil {
			return operation.BatchResult{}, err
		}
		return migrationBatchResult(checkpoint, 0, 0, nil, true)
	}
	batch, err := executor.repository.LoadBatch(ctx, request, checkpoint, limit)
	if err != nil {
		return operation.BatchResult{}, classifyRepositoryError(err)
	}
	if !validBatch(request, checkpoint, batch, limit) {
		return operation.BatchResult{}, operation.Terminal("invalid_attribute_migration_batch", ErrInvalidMigration)
	}

	failures := make([]operation.ItemFailure, 0)
	var succeeded int64
	for _, source := range batch.Values {
		canonical, convertErr := sourceCanonical(source, batch.Source, batch.Candidate)
		candidateLegacy := ""
		if convertErr == nil {
			candidateLegacy, convertErr = attributevalue.LegacyProjection(
				canonical,
				batch.Candidate.DataType,
			)
		}
		if convertErr != nil {
			failures = append(failures, operation.ItemFailure{Reference: source.ID.String(), Code: "invalid_candidate_value"})
			continue
		}
		if writeErr := executor.repository.PutShadow(ctx, ShadowValue{
			UserID: source.UserID, DefinitionID: source.DefinitionID,
			SourceRevisionID:    request.SourceRevisionID,
			CandidateRevisionID: request.CandidateRevisionID,
			CanonicalValue:      canonical, LegacyValue: candidateLegacy,
			SourceValueVersion: source.Version,
		}); writeErr != nil {
			return operation.BatchResult{}, classifyRepositoryError(writeErr)
		}
		succeeded++
	}
	processed := int64(len(batch.Values))
	checkpoint.SourceProcessed += processed
	checkpoint.Quarantined += int64(len(failures))
	checkpoint.AfterSourceValueID = nextSourceID(checkpoint.AfterSourceValueID, batch.Values)
	if batch.Done && checkpoint.SourceProcessed != request.SourceCount {
		return operation.BatchResult{}, operation.Terminal("attribute_migration_count_drift", ErrValueDrift)
	}
	if batch.Done && checkpoint.Quarantined == 0 {
		if err := executor.verifyAndActivate(ctx, request); err != nil {
			return operation.BatchResult{}, err
		}
	}
	return migrationBatchResult(checkpoint, processed, succeeded, failures, batch.Done)
}

func (executor *Executor) verifyAndActivate(ctx context.Context, request Request) error {
	if verifyErr := executor.repository.VerifyFinal(ctx, request); verifyErr != nil {
		return classifyRepositoryError(verifyErr)
	}
	if activateErr := executor.activator.ActivateMigratedRevision(ctx, request); activateErr != nil {
		return classifyRepositoryError(activateErr)
	}
	return nil
}

func migrationBatchResult(
	checkpoint Checkpoint,
	processed int64,
	succeeded int64,
	failures []operation.ItemFailure,
	done bool,
) (operation.BatchResult, error) {
	nextCheckpoint, marshalErr := json.Marshal(checkpoint)
	if marshalErr != nil {
		return operation.BatchResult{}, operation.Terminal("invalid_attribute_migration_checkpoint", marshalErr)
	}
	result := operation.BatchResult{
		Processed: processed, Succeeded: succeeded,
		Failed: int64(len(failures)), Failures: failures,
		NextCheckpoint: nextCheckpoint, Done: done,
	}
	if done && checkpoint.Quarantined > 0 {
		// Operation batches retain only opaque value IDs and failure codes. Raw
		// profile content never reaches the operation, audit, or worker log.
		result.FailureCode = "attribute_migration_quarantined"
	}
	return result, nil
}

func Prepare(ctx context.Context, repository Repository, request Request) (Request, error) {
	if repository == nil || ValidateIntent(request) != nil ||
		request.TargetCutoff != uuid.Nil || request.TargetCount != 0 ||
		request.SourceCutoff != uuid.Nil || request.SourceCount != 0 {
		return Request{}, ErrInvalidMigration
	}
	prepared, err := repository.Prepare(ctx, request)
	if err != nil || ValidateRequest(prepared) != nil ||
		!sameMigrationIntent(request, prepared) {
		if err != nil {
			return Request{}, err
		}
		return Request{}, ErrInvalidMigration
	}
	return prepared, nil
}

func sameMigrationIntent(intent Request, prepared Request) bool {
	prepared.TargetCutoff = uuid.Nil
	prepared.TargetCount = 0
	prepared.SourceCutoff = uuid.Nil
	prepared.SourceCount = 0
	return prepared == intent
}

// ValidateIntent checks the immutable administrative command before a
// repository captures its value-set high-water mark.
func ValidateIntent(request Request) error {
	if request.OperationID == uuid.Nil || request.CommandID == uuid.Nil ||
		request.OperationID != request.CommandID || request.DefinitionID == uuid.Nil ||
		request.ExpectedVersion < 1 || request.SourceRevisionID == uuid.Nil ||
		request.CandidateRevisionID == uuid.Nil ||
		request.SourceRevisionID == request.CandidateRevisionID ||
		request.ActorUserID == uuid.Nil || request.ActorSessionID == uuid.Nil ||
		strings.TrimSpace(request.Reason) != request.Reason || request.Reason == "" ||
		len(request.Reason) > 1024 || !requestHashPattern.MatchString(request.ConfirmationHash) {
		return ErrInvalidMigration
	}
	return nil
}

// ValidateRequest checks the complete persisted operation request.
func ValidateRequest(request Request) error {
	if ValidateIntent(request) != nil ||
		!validFrozenValueSet(request.TargetCutoff, request.TargetCount) ||
		!validFrozenValueSet(request.SourceCutoff, request.SourceCount) ||
		(request.TargetCount == 0 && request.SourceCount == 0) ||
		request.TargetCount > (1<<63-1)-request.SourceCount {
		return ErrInvalidMigration
	}
	return nil
}

func validFrozenValueSet(cutoff uuid.UUID, count int64) bool {
	if count < 0 {
		return false
	}
	if count == 0 {
		return cutoff == uuid.Nil
	}
	return cutoff != uuid.Nil && cutoff.Version() == 7
}

func validRequest(request Request) bool {
	return ValidateRequest(request) == nil
}

func decodeCheckpoint(raw json.RawMessage) (Checkpoint, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return Checkpoint{}, ErrInvalidMigration
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var checkpoint Checkpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return Checkpoint{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Checkpoint{}, ErrInvalidMigration
	}
	return checkpoint, nil
}

func normalizeCheckpoint(request Request, checkpoint Checkpoint) (Checkpoint, bool) {
	if checkpoint.Phase == "" {
		if checkpoint.AfterTargetValueID != uuid.Nil || checkpoint.TargetProcessed != 0 ||
			checkpoint.AfterSourceValueID != uuid.Nil || checkpoint.SourceProcessed != 0 ||
			checkpoint.Quarantined != 0 {
			return Checkpoint{}, false
		}
		if request.TargetCount > 0 {
			checkpoint.Phase = PhaseCleanupTarget
		} else {
			checkpoint.Phase = PhaseCopySource
		}
	}
	if checkpoint.Quarantined < 0 || checkpoint.Quarantined > checkpoint.SourceProcessed ||
		!validProgress(
			checkpoint.TargetProcessed,
			request.TargetCount,
			checkpoint.AfterTargetValueID,
			request.TargetCutoff,
		) ||
		!validProgress(
			checkpoint.SourceProcessed,
			request.SourceCount,
			checkpoint.AfterSourceValueID,
			request.SourceCutoff,
		) {
		return Checkpoint{}, false
	}
	switch checkpoint.Phase {
	case PhaseCleanupTarget:
		return checkpoint, request.TargetCount > 0 &&
			checkpoint.SourceProcessed == 0 &&
			checkpoint.AfterSourceValueID == uuid.Nil &&
			checkpoint.Quarantined == 0
	case PhaseCopySource:
		return checkpoint, checkpoint.TargetProcessed == request.TargetCount
	default:
		return Checkpoint{}, false
	}
}

func validProgress(
	processed int64,
	count int64,
	afterValueID uuid.UUID,
	cutoff uuid.UUID,
) bool {
	if processed < 0 || processed > count {
		return false
	}
	if processed == 0 {
		return afterValueID == uuid.Nil
	}
	return afterValueID != uuid.Nil && afterValueID.Version() == 7 &&
		bytes.Compare(afterValueID[:], cutoff[:]) <= 0
}

func validCleanupBatch(
	request Request,
	checkpoint Checkpoint,
	batch CleanupBatch,
	limit int,
) bool {
	if len(batch.ValueIDs) > limit ||
		int64(len(batch.ValueIDs)) > request.TargetCount-checkpoint.TargetProcessed {
		return false
	}
	previous := checkpoint.AfterTargetValueID
	for _, valueID := range batch.ValueIDs {
		if valueID == uuid.Nil || valueID.Version() != 7 ||
			bytes.Compare(valueID[:], request.TargetCutoff[:]) > 0 ||
			(previous != uuid.Nil && bytes.Compare(valueID[:], previous[:]) <= 0) {
			return false
		}
		previous = valueID
	}
	return batch.Done || len(batch.ValueIDs) > 0
}

func validBatch(request Request, checkpoint Checkpoint, batch Batch, limit int) bool {
	if attributevalue.ValidateRevision(batch.Source) != nil ||
		batch.Source.ID != request.SourceRevisionID ||
		batch.Source.DefinitionID != request.DefinitionID ||
		attributevalue.ValidateRevision(batch.Candidate) != nil ||
		batch.Candidate.ID != request.CandidateRevisionID ||
		batch.Candidate.DefinitionID != request.DefinitionID || len(batch.Values) > limit ||
		int64(len(batch.Values)) > request.SourceCount-checkpoint.SourceProcessed {
		return false
	}
	previous := checkpoint.AfterSourceValueID
	for _, value := range batch.Values {
		if value.ID == uuid.Nil || value.UserID == uuid.Nil || value.DefinitionID != request.DefinitionID ||
			value.RevisionID != request.SourceRevisionID || value.Version < 1 ||
			len(value.CanonicalValue) == 0 ||
			value.ID.Version() != 7 || bytes.Compare(value.ID[:], request.SourceCutoff[:]) > 0 ||
			(previous != uuid.Nil && bytes.Compare(value.ID[:], previous[:]) <= 0) {
			return false
		}
		previous = value.ID
	}
	return batch.Done || len(batch.Values) > 0
}

func sourceCanonical(
	source SourceValue,
	sourceRevision entity.AttributeDefinitionRevision,
	candidate entity.AttributeDefinitionRevision,
) (json.RawMessage, error) {
	if len(source.CanonicalValue) == 0 ||
		source.RevisionID != sourceRevision.ID ||
		source.DefinitionID != sourceRevision.DefinitionID {
		return nil, ErrValueDrift
	}
	canonical, err := attributevalue.Canonicalize(source.CanonicalValue, sourceRevision)
	if err != nil || !bytes.Equal(canonical, source.CanonicalValue) {
		return nil, ErrValueDrift
	}
	legacy, err := attributevalue.LegacyProjection(canonical, sourceRevision.DataType)
	if err != nil || legacy != source.LegacyValue {
		return nil, ErrValueDrift
	}
	return attributevalue.CanonicalizeLegacy(legacy, candidate)
}

func nextSourceID(current uuid.UUID, values []SourceValue) uuid.UUID {
	if len(values) == 0 {
		return current
	}
	return values[len(values)-1].ID
}

func nextValueID(current uuid.UUID, values []uuid.UUID) uuid.UUID {
	if len(values) == 0 {
		return current
	}
	return values[len(values)-1]
}

func classifyRepositoryError(err error) error {
	switch {
	case errors.Is(err, ErrDefinitionDrift):
		return operation.Terminal("attribute_definition_drift", err)
	case errors.Is(err, ErrValueDrift):
		return operation.Terminal("attribute_value_drift", err)
	case errors.Is(err, ErrInvalidMigration):
		return operation.Terminal("invalid_attribute_migration", err)
	default:
		return operation.Retryable("attribute_migration_unavailable", 0, fmt.Errorf("attribute migration repository: %w", err))
	}
}
