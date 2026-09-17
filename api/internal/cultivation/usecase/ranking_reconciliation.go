package usecase

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	cultivationcontract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	RankingReconciliationOperationKind    = entity.RankingReconciliationOperationKind
	defaultRankingReconciliationBatchSize = 100
	rankingReconciliationRetryDelay       = 30 * time.Second
)

var rankingReconciliationNamespace = uuid.MustParse(
	"6f4ea98e-4608-50bf-9b86-0f2e7bb1a9f4",
)

type RankingReconciliationOptions struct {
	BatchSize int
}

type RankingReconciliationWorkflow struct {
	repository   ports.RankingReconciliationRepository
	operations   ports.RankingOperationReader
	starter      ports.RankingOperationStarter
	transactions operation.UnitOfWork
	outbox       eventing.OutboxAppender
	batchSize    int
}

type rankingReconciliationRequest struct {
	OperationID        uuid.UUID `json:"operation_id"`
	CommandID          uuid.UUID `json:"command_id"`
	ActorUserID        uuid.UUID `json:"actor_user_id"`
	ActorSessionID     uuid.UUID `json:"actor_session_id"`
	Reason             string    `json:"reason"`
	SourceCutoff       time.Time `json:"source_cutoff"`
	ThroughUserID      uuid.UUID `json:"through_user_id"`
	Total              int64     `json:"total"`
	RepairCount        int64     `json:"repair_count"`
	ProjectionRevision int64     `json:"projection_revision"`
	PreviewChecksum    string    `json:"preview_checksum"`
}

type rankingReconciliationCheckpoint struct {
	AfterUserID uuid.UUID `json:"after_user_id"`
	Repaired    int64     `json:"repaired"`
}

func NewRankingReconciliationWorkflow(
	repository ports.RankingReconciliationRepository,
	operations ports.RankingOperationReader,
	starter ports.RankingOperationStarter,
	transactions operation.UnitOfWork,
	outbox eventing.OutboxAppender,
	options RankingReconciliationOptions,
) (*RankingReconciliationWorkflow, error) {
	if repository == nil || operations == nil || starter == nil || transactions == nil || outbox == nil {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	if options.BatchSize == 0 {
		options.BatchSize = defaultRankingReconciliationBatchSize
	}
	if options.BatchSize < 1 || options.BatchSize > operation.MaxBatchSize {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	return &RankingReconciliationWorkflow{
		repository:   repository,
		operations:   operations,
		starter:      starter,
		transactions: transactions,
		outbox:       outbox,
		batchSize:    options.BatchSize,
	}, nil
}

func (workflow *RankingReconciliationWorkflow) Preview(
	ctx context.Context,
	command entity.RankingReconciliationPreviewCommand,
) (*contract.RankingReconciliationPreviewResponse, error) {
	if workflow == nil || workflow.repository == nil || ctx == nil || ctx.Err() != nil ||
		command.ActorUserID == uuid.Nil || command.ActorSessionID == uuid.Nil {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	plan, err := workflow.repository.Preview(ctx)
	if err != nil {
		return nil, err
	}
	if !validRankingReconciliationPlan(plan) {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	checksum := entity.RankingReconciliationPreviewChecksum(command, plan)
	if checksum == ([32]byte{}) {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	logger.FromContext(ctx).Debug(
		"cultivation ranking reconciliation previewed",
		zap.Int64("source_count", plan.Total),
		zap.Int64("repair_count", plan.RepairCount),
		zap.Int64("projection_revision", plan.ProjectionRevision),
		zap.Time("source_cutoff", plan.SourceCutoff.UTC()),
	)
	return &contract.RankingReconciliationPreviewResponse{
		SourceCutoff:       plan.SourceCutoff.UTC(),
		ProjectionRevision: plan.ProjectionRevision,
		SourceCount:        plan.Total,
		RepairCount:        plan.RepairCount,
		PreviewChecksum:    hex.EncodeToString(checksum[:]),
	}, nil
}

func (workflow *RankingReconciliationWorkflow) Register(
	registry *operation.Registry,
) error {
	if workflow == nil || registry == nil {
		return operation.ErrInvalidExecutor
	}
	return operation.RegisterJSON(
		registry,
		RankingReconciliationOperationKind,
		workflow.executeBatch,
	)
}

func (workflow *RankingReconciliationWorkflow) Start(
	ctx context.Context,
	command entity.RankingReconciliationCommand,
) (*contract.RankingReconciliationStartResponse, error) {
	command.Reason = strings.TrimSpace(command.Reason)
	if workflow == nil || workflow.repository == nil || workflow.operations == nil ||
		workflow.starter == nil || workflow.transactions == nil ||
		workflow.outbox == nil ||
		ctx == nil || ctx.Err() != nil || !validRankingReconciliationCommand(command) {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	ctx = correlation.EnsureContext(ctx)
	operationID := deriveRankingReconciliationID(command.ID)
	if existing, err := workflow.operations.Get(ctx, operationID); err == nil {
		return workflow.replayStart(existing, command)
	} else if !errors.Is(err, operation.ErrNotFound) {
		return nil, err
	}

	var (
		plan    entity.RankingReconciliationPlan
		started operation.CreateResult
	)
	err := workflow.transactions.Within(ctx, func(txCtx context.Context) error {
		var prepareErr error
		plan, prepareErr = workflow.repository.Prepare(txCtx, operationID, entity.RankingReconciliationPlan{
			SourceCutoff:       command.SourceCutoff.UTC(),
			Total:              command.SourceCount,
			RepairCount:        command.RepairCount,
			ProjectionRevision: command.ExpectedRevision,
		})
		if prepareErr != nil {
			return prepareErr
		}
		if !validRankingReconciliationPlan(plan) ||
			!entity.RankingReconciliationPreviewMatches(
				command.PreviewChecksum,
				entity.RankingReconciliationPreviewCommand{
					ActorUserID:    command.ActorUserID,
					ActorSessionID: command.ActorSessionID,
				},
				plan,
			) {
			return entity.ErrRankingReconciliationPreviewConflict
		}
		request := rankingReconciliationRequest{
			OperationID:        operationID,
			CommandID:          command.ID,
			ActorUserID:        command.ActorUserID,
			ActorSessionID:     command.ActorSessionID,
			Reason:             command.Reason,
			SourceCutoff:       plan.SourceCutoff.UTC(),
			ThroughUserID:      plan.ThroughUserID,
			Total:              plan.Total,
			RepairCount:        plan.RepairCount,
			ProjectionRevision: plan.ProjectionRevision,
			PreviewChecksum:    hex.EncodeToString(command.PreviewChecksum[:]),
		}
		encodedRequest, requestHash, encodeErr := operation.EncodeRequest(request)
		if encodeErr != nil {
			return encodeErr
		}
		checkpoint, encodeErr := json.Marshal(rankingReconciliationCheckpoint{})
		if encodeErr != nil {
			return encodeErr
		}
		sessionID := command.ActorSessionID
		started, prepareErr = workflow.starter.StartInTransaction(txCtx, operation.StartRequest{
			Operation: operation.CreateCommand{
				ID:             operationID,
				Kind:           RankingReconciliationOperationKind,
				IdempotencyKey: command.ID,
				RequestHash:    requestHash,
				Request:        encodedRequest,
				Total:          plan.Total,
				BatchSize:      workflow.batchSize,
				Checkpoint:     checkpoint,
				ActorUserID:    command.ActorUserID,
				ActorSessionID: &sessionID,
				Reason:         command.Reason,
				CorrelationID:  correlation.FromContext(txCtx),
			},
			Audit: audit.Entry{
				Action:   "cultivation.ranking.reconciliation.start",
				Resource: "cultivation_ranking_projection",
				Before:   map[string]any{},
				After: map[string]any{
					"source_cutoff":       plan.SourceCutoff.UTC(),
					"source_count":        plan.Total,
					"repair_count":        plan.RepairCount,
					"projection_revision": plan.ProjectionRevision,
					"batch_size":          workflow.batchSize,
				},
			},
		})
		if prepareErr != nil || started.Replay {
			return prepareErr
		}
		payload, encodeErr := json.Marshal(cultivationcontract.RankingReconciliationStartedV1{
			OperationID:        operationID,
			SourceCutoff:       plan.SourceCutoff.UTC(),
			SourceCount:        plan.Total,
			RepairCount:        plan.RepairCount,
			ProjectionRevision: plan.ProjectionRevision,
		})
		if encodeErr != nil {
			return encodeErr
		}
		return workflow.outbox.Append(txCtx, eventing.Event{
			ID:               cultivationcontract.RankingReconciliationStartedEventID(operationID),
			Type:             cultivationcontract.EventRankingReconciliationStartedV1,
			AggregateType:    cultivationcontract.AggregateRankingReconciliation,
			AggregateID:      operationID,
			AggregateVersion: 1,
			CorrelationID:    correlation.FromContext(txCtx),
			CausationID:      &command.ID,
			Payload:          payload,
			OccurredAt:       time.Now().UTC(),
		})
	})
	if err != nil {
		if errors.Is(err, entity.ErrRankingReconciliationActive) {
			if existing, getErr := workflow.operations.Get(ctx, operationID); getErr == nil {
				return workflow.replayStart(existing, command)
			}
		}
		if errors.Is(err, operation.ErrConflict) {
			if existing, getErr := workflow.operations.Get(ctx, operationID); getErr == nil {
				return workflow.replayStart(existing, command)
			}
		}
		return nil, err
	}
	response, err := rankingReconciliationStartResponse(started.Operation, plan, started.Replay)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"cultivation ranking reconciliation scheduled",
		zap.String("operation_id", response.Operation.ID.String()),
		zap.Int64("source_count", plan.Total),
		zap.Int64("repair_count", plan.RepairCount),
		zap.Time("source_cutoff", plan.SourceCutoff.UTC()),
		zap.Bool("replayed", started.Replay),
	)
	return response, nil
}

func (workflow *RankingReconciliationWorkflow) Status(
	ctx context.Context,
) (*contract.RankingReconciliationStatusResponse, error) {
	if workflow == nil || workflow.repository == nil || workflow.operations == nil ||
		ctx == nil || ctx.Err() != nil {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	state, err := workflow.repository.State(ctx)
	if err != nil {
		return nil, err
	}
	if state.ProjectionRevision < 0 ||
		(state.LastReconciledAt != nil && state.LastReconciledAt.IsZero()) {
		return nil, entity.ErrInvalidRankingReconciliation
	}
	page, err := workflow.operations.List(ctx, operation.ListFilter{
		Kind:  RankingReconciliationOperationKind,
		Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	response := &contract.RankingReconciliationStatusResponse{
		ProjectionRevision: state.ProjectionRevision,
		LastReconciledAt:   cloneRankingTime(state.LastReconciledAt),
	}
	if len(page.Items) > 0 {
		projected, projectErr := operation.Project(page.Items[0])
		if projectErr != nil {
			return nil, projectErr
		}
		response.LatestOperation = &projected
	}
	return response, nil
}

func (workflow *RankingReconciliationWorkflow) executeBatch(
	ctx context.Context,
	request rankingReconciliationRequest,
	input operation.BatchInput,
) (operation.BatchResult, error) {
	if workflow == nil || workflow.repository == nil ||
		ctx == nil || ctx.Err() != nil || input.OperationID == uuid.Nil ||
		input.OperationID != request.OperationID || input.Sequence < 1 || input.Limit < 1 ||
		!validRankingReconciliationRequest(request) {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_ranking_reconciliation",
			entity.ErrInvalidRankingReconciliation,
		)
	}
	checkpoint, err := decodeRankingReconciliationCheckpoint(input.Checkpoint)
	if err != nil || !validRankingReconciliationCheckpoint(checkpoint, request) {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_ranking_reconciliation_checkpoint",
			entity.ErrInvalidRankingReconciliation,
		)
	}
	batch, err := workflow.repository.ReconcileBatch(ctx, entity.RankingReconciliationBatchQuery{
		SourceCutoff:  request.SourceCutoff.UTC(),
		ThroughUserID: request.ThroughUserID,
		AfterUserID:   checkpoint.AfterUserID,
		ExpectedTotal: request.Total,
		Limit:         input.Limit,
	})
	if err != nil {
		if errors.Is(err, entity.ErrRankingReconciliationPreviewConflict) {
			return operation.BatchResult{}, operation.Terminal(
				"ranking_reconciliation_fence_changed",
				err,
			)
		}
		return operation.BatchResult{}, operation.Retryable(
			"ranking_reconciliation_unavailable",
			rankingReconciliationRetryDelay,
			err,
		)
	}
	if !validRankingReconciliationBatch(batch, checkpoint, request, input.Limit) ||
		checkpoint.Repaired > math.MaxInt64-batch.Repaired {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_ranking_reconciliation_batch",
			entity.ErrInvalidRankingReconciliation,
		)
	}
	checkpoint.AfterUserID = batch.LastUserID
	checkpoint.Repaired += batch.Repaired
	nextCheckpoint, err := json.Marshal(checkpoint)
	if err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_ranking_reconciliation_checkpoint",
			err,
		)
	}
	logger.FromContext(ctx).Debug(
		"cultivation ranking reconciliation batch applied",
		zap.String("operation_id", input.OperationID.String()),
		zap.Int64("batch_sequence", input.Sequence),
		zap.Int64("processed_count", batch.Processed),
		zap.Int64("repaired_count", batch.Repaired),
		zap.Int64("cumulative_repaired_count", checkpoint.Repaired),
		zap.Int64("projection_revision", batch.ProjectionRevision),
		zap.Bool("done", batch.Done),
	)
	if batch.Done {
		logger.FromContext(ctx).Info(
			"cultivation ranking reconciliation completed",
			zap.String("operation_id", input.OperationID.String()),
			zap.Int64("repaired_count", checkpoint.Repaired),
			zap.Int64("projection_revision", batch.ProjectionRevision),
			zap.Time("reconciled_through", request.SourceCutoff.UTC()),
		)
	}
	return operation.BatchResult{
		Processed:      batch.Processed,
		Succeeded:      batch.Processed,
		NextCheckpoint: nextCheckpoint,
		Done:           batch.Done,
	}, nil
}

func (workflow *RankingReconciliationWorkflow) replayStart(
	existing operation.Operation,
	command entity.RankingReconciliationCommand,
) (*contract.RankingReconciliationStartResponse, error) {
	request, err := decodeRankingReconciliationRequest(existing.Request)
	if err != nil || existing.ID != deriveRankingReconciliationID(command.ID) ||
		existing.Kind != RankingReconciliationOperationKind ||
		existing.IdempotencyKey != command.ID || request.CommandID != command.ID ||
		request.ActorUserID != command.ActorUserID ||
		request.ActorSessionID != command.ActorSessionID ||
		request.Reason != command.Reason || existing.ActorUserID != command.ActorUserID ||
		existing.ActorSessionID == nil || *existing.ActorSessionID != command.ActorSessionID ||
		existing.Reason != command.Reason || existing.Total != request.Total ||
		!request.SourceCutoff.Equal(command.SourceCutoff) ||
		request.ProjectionRevision != command.ExpectedRevision ||
		request.Total != command.SourceCount || request.RepairCount != command.RepairCount ||
		request.PreviewChecksum != hex.EncodeToString(command.PreviewChecksum[:]) {
		return nil, operation.ErrConflict
	}
	return rankingReconciliationStartResponse(existing, entity.RankingReconciliationPlan{
		SourceCutoff:       request.SourceCutoff,
		ThroughUserID:      request.ThroughUserID,
		Total:              request.Total,
		RepairCount:        request.RepairCount,
		ProjectionRevision: request.ProjectionRevision,
	}, true)
}

func rankingReconciliationStartResponse(
	value operation.Operation,
	plan entity.RankingReconciliationPlan,
	replayed bool,
) (*contract.RankingReconciliationStartResponse, error) {
	projected, err := operation.Project(value)
	if err != nil {
		return nil, err
	}
	response := &contract.RankingReconciliationStartResponse{
		Operation:          projected,
		SourceCutoff:       plan.SourceCutoff.UTC(),
		SourceCount:        plan.Total,
		RepairCount:        plan.RepairCount,
		ProjectionRevision: plan.ProjectionRevision,
		IdempotentReplay:   replayed,
	}
	return response, nil
}

func deriveRankingReconciliationID(commandID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(rankingReconciliationNamespace, commandID[:])
}

func validRankingReconciliationCommand(command entity.RankingReconciliationCommand) bool {
	if command.ID == uuid.Nil || command.ActorUserID == uuid.Nil ||
		command.ActorSessionID == uuid.Nil || !utf8.ValidString(command.Reason) ||
		len(command.Reason) < 3 || len(command.Reason) > operation.MaxReasonBytes ||
		command.SourceCutoff.IsZero() || command.SourceCutoff.Location() != time.UTC ||
		command.ExpectedRevision < 0 || command.SourceCount < 0 ||
		command.RepairCount < 0 || command.RepairCount > command.SourceCount ||
		command.PreviewChecksum == ([32]byte{}) {
		return false
	}
	for _, current := range command.Reason {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}

func validRankingReconciliationPlan(plan entity.RankingReconciliationPlan) bool {
	return !plan.SourceCutoff.IsZero() && plan.SourceCutoff.Location() == time.UTC &&
		plan.Total >= 0 && plan.RepairCount >= 0 && plan.RepairCount <= plan.Total &&
		plan.ProjectionRevision >= 0 && ((plan.Total == 0) == (plan.ThroughUserID == uuid.Nil))
}

func validRankingReconciliationRequest(request rankingReconciliationRequest) bool {
	return request.OperationID != uuid.Nil && request.CommandID != uuid.Nil &&
		request.OperationID == deriveRankingReconciliationID(request.CommandID) &&
		request.ActorUserID != uuid.Nil && request.ActorSessionID != uuid.Nil &&
		validRankingReconciliationCommand(entity.RankingReconciliationCommand{
			ID:               request.CommandID,
			ActorUserID:      request.ActorUserID,
			ActorSessionID:   request.ActorSessionID,
			Reason:           request.Reason,
			SourceCutoff:     request.SourceCutoff,
			ExpectedRevision: request.ProjectionRevision,
			SourceCount:      request.Total,
			RepairCount:      request.RepairCount,
			PreviewChecksum:  decodeRankingPreviewChecksum(request.PreviewChecksum),
		}) && validRankingReconciliationPlan(entity.RankingReconciliationPlan{
		SourceCutoff:       request.SourceCutoff,
		ThroughUserID:      request.ThroughUserID,
		Total:              request.Total,
		RepairCount:        request.RepairCount,
		ProjectionRevision: request.ProjectionRevision,
	})
}

func decodeRankingPreviewChecksum(value string) [32]byte {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != value {
		return [32]byte{}
	}
	var checksum [32]byte
	copy(checksum[:], decoded)
	return checksum
}

func validRankingReconciliationCheckpoint(
	checkpoint rankingReconciliationCheckpoint,
	request rankingReconciliationRequest,
) bool {
	return checkpoint.Repaired >= 0 &&
		(checkpoint.AfterUserID == uuid.Nil || request.ThroughUserID != uuid.Nil) &&
		(checkpoint.AfterUserID == uuid.Nil ||
			bytes.Compare(checkpoint.AfterUserID[:], request.ThroughUserID[:]) <= 0)
}

func validRankingReconciliationBatch(
	batch entity.RankingReconciliationBatch,
	checkpoint rankingReconciliationCheckpoint,
	request rankingReconciliationRequest,
	limit int,
) bool {
	if batch.Processed < 0 || batch.Processed > int64(limit) ||
		batch.Repaired < 0 || batch.Repaired > batch.Processed ||
		batch.ProjectionRevision < 0 || (!batch.Done && batch.Processed == 0) {
		return false
	}
	if batch.Processed == 0 {
		return batch.Done && batch.LastUserID == checkpoint.AfterUserID
	}
	return batch.LastUserID != uuid.Nil &&
		(checkpoint.AfterUserID == uuid.Nil ||
			bytes.Compare(batch.LastUserID[:], checkpoint.AfterUserID[:]) > 0) &&
		bytes.Compare(batch.LastUserID[:], request.ThroughUserID[:]) <= 0
}

func decodeRankingReconciliationRequest(
	value json.RawMessage,
) (rankingReconciliationRequest, error) {
	var request rankingReconciliationRequest
	if err := decodeStrictRankingDocument(value, &request); err != nil ||
		!validRankingReconciliationRequest(request) {
		return rankingReconciliationRequest{}, entity.ErrInvalidRankingReconciliation
	}
	return request, nil
}

func decodeRankingReconciliationCheckpoint(
	value json.RawMessage,
) (rankingReconciliationCheckpoint, error) {
	var checkpoint rankingReconciliationCheckpoint
	if err := decodeStrictRankingDocument(value, &checkpoint); err != nil ||
		checkpoint.Repaired < 0 {
		return rankingReconciliationCheckpoint{}, entity.ErrInvalidRankingReconciliation
	}
	return checkpoint, nil
}

func decodeStrictRankingDocument(value json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return entity.ErrInvalidRankingReconciliation
	}
	return nil
}

func cloneRankingTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

var _ ports.RankingReconciliationService = (*RankingReconciliationWorkflow)(nil)
