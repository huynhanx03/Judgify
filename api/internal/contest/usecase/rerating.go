package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	defaultRatingReratingBatchSize = 10
	ratingReratingRetryDelay       = 30 * time.Second
)

var (
	ErrInvalidRatingRerating = errors.New("rating rerating: invalid request or dependency")
	ratingReratingNamespace  = uuid.MustParse(
		"fb5708ac-8641-5c1d-94bb-987141ace06b",
	)
)

type RatingReratingDependencies struct {
	Repository   ports.RatingReratingRepository
	Operations   operation.Repository
	Starter      *operation.Starter
	Transactions operation.UnitOfWork
	Outbox       eventing.OutboxAppender
}

type RatingReratingConfig struct {
	BatchSize int
}

// RatingReratingWorkflow coordinates the contest-owned rating ledger with the
// domain-neutral operation control plane. The generic runner never needs to
// understand contests, rating algorithms, or projection activation.
type RatingReratingWorkflow struct {
	repository   ports.RatingReratingRepository
	operations   operation.Repository
	starter      *operation.Starter
	transactions operation.UnitOfWork
	outbox       eventing.OutboxAppender
	batchSize    int
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

type ratingReratingRequest struct {
	OperationID                  uuid.UUID `json:"operation_id"`
	TriggerContestID             uuid.UUID `json:"trigger_contest_id"`
	ActorUserID                  uuid.UUID `json:"actor_user_id"`
	Reason                       string    `json:"reason"`
	FromSequence                 int64     `json:"from_sequence"`
	ThroughSequence              int64     `json:"through_sequence"`
	SourceProjectionRevision     int64     `json:"source_projection_revision"`
	SourceProjectionGenerationID uuid.UUID `json:"source_projection_generation_id"`
	SourceRunChainID             uuid.UUID `json:"source_run_chain_id"`
	SourceProjectionChecksum     []byte    `json:"source_projection_checksum"`
	SourceRunChainChecksum       []byte    `json:"source_run_chain_checksum"`
	TargetProjectionGenerationID uuid.UUID `json:"target_projection_generation_id"`
	TargetRunChainID             uuid.UUID `json:"target_run_chain_id"`
	AlgorithmVersion             string    `json:"algorithm_version"`
	InitialProjectionChecksum    []byte    `json:"initial_projection_checksum"`
	InitialRunChainChecksum      []byte    `json:"initial_run_chain_checksum"`
}

type ratingReratingCheckpoint struct {
	NextSequence       int64  `json:"next_sequence"`
	ProjectionChecksum []byte `json:"projection_checksum"`
	RunChainChecksum   []byte `json:"run_chain_checksum"`
}

func NewRatingReratingWorkflow(
	dependencies RatingReratingDependencies,
	config RatingReratingConfig,
) (*RatingReratingWorkflow, error) {
	if dependencies.Repository == nil ||
		dependencies.Operations == nil ||
		dependencies.Starter == nil ||
		dependencies.Transactions == nil ||
		dependencies.Outbox == nil {
		return nil, ErrInvalidRatingRerating
	}
	if config.BatchSize == 0 {
		config.BatchSize = defaultRatingReratingBatchSize
	}
	if config.BatchSize < 1 ||
		config.BatchSize > operation.MaxBatchSize {
		return nil, ErrInvalidRatingRerating
	}
	return &RatingReratingWorkflow{
		repository:   dependencies.Repository,
		operations:   dependencies.Operations,
		starter:      dependencies.Starter,
		transactions: dependencies.Transactions,
		outbox:       dependencies.Outbox,
		batchSize:    config.BatchSize,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewV7,
	}, nil
}

func (workflow *RatingReratingWorkflow) Register(
	registry *operation.Registry,
) error {
	if workflow == nil || registry == nil {
		return operation.ErrInvalidExecutor
	}
	return operation.RegisterJSON(
		registry,
		contract.RatingReratingOperationKind,
		workflow.executeBatch,
	)
}

func (workflow *RatingReratingWorkflow) Start(
	ctx context.Context,
	actorUserID uuid.UUID,
	actorSessionID uuid.UUID,
	idempotencyKey uuid.UUID,
	triggerContestID uuid.UUID,
	reason string,
) (*contract.RatingReratingStartResponse, error) {
	reason = strings.TrimSpace(reason)
	if workflow == nil || workflow.repository == nil ||
		workflow.operations == nil || workflow.starter == nil ||
		workflow.transactions == nil || workflow.outbox == nil ||
		workflow.now == nil || ctx == nil || ctx.Err() != nil ||
		actorUserID == uuid.Nil || actorSessionID == uuid.Nil ||
		idempotencyKey == uuid.Nil || triggerContestID == uuid.Nil ||
		!utf8.ValidString(reason) || len(reason) < 3 ||
		len(reason) > operation.MaxReasonBytes {
		return nil, ErrInvalidRatingRerating
	}
	ctx = correlation.EnsureContext(ctx)
	operationID := deriveRatingReratingID(
		"operation",
		idempotencyKey,
	)
	if existing, err := workflow.operations.Get(ctx, operationID); err == nil {
		return workflow.replayStart(
			existing,
			actorUserID,
			actorSessionID,
			idempotencyKey,
			triggerContestID,
			reason,
		)
	} else if !errors.Is(err, operation.ErrNotFound) {
		return nil, err
	}

	now := workflow.now().UTC()
	targetGenerationID := deriveRatingReratingID(
		"projection",
		operationID,
	)
	targetRunChainID := deriveRatingReratingID(
		"chain",
		operationID,
	)
	var (
		started operation.CreateResult
		plan    entity.RatingReratingPlan
	)
	err := workflow.transactions.Within(
		ctx,
		func(txCtx context.Context) error {
			existing, existingErr := workflow.operations.Get(
				txCtx,
				operationID,
			)
			if existingErr == nil {
				replayed, replayErr := workflow.replayStart(
					existing,
					actorUserID,
					actorSessionID,
					idempotencyKey,
					triggerContestID,
					reason,
				)
				if replayErr != nil {
					return replayErr
				}
				decoded, decodeErr := decodeRatingReratingRequest(
					existing.Request,
				)
				if decodeErr != nil {
					return decodeErr
				}
				plan = decoded.plan()
				started = operation.CreateResult{
					Operation: existing,
					Replay:    replayed.IdempotentReplay,
				}
				return nil
			}
			if !errors.Is(existingErr, operation.ErrNotFound) {
				return existingErr
			}
			prepared, prepareErr := workflow.repository.PrepareRerating(
				txCtx,
				operationID,
				triggerContestID,
				targetGenerationID,
				targetRunChainID,
				now,
			)
			if prepareErr != nil {
				return prepareErr
			}
			plan = *prepared
			plan.ActorUserID = actorUserID
			plan.Reason = reason
			request := ratingReratingRequestFromPlan(plan)
			encodedRequest, requestHash, encodeErr :=
				operation.EncodeRequest(request)
			if encodeErr != nil {
				return encodeErr
			}
			checkpoint, encodeErr := json.Marshal(
				ratingReratingCheckpoint{
					NextSequence: 1,
					ProjectionChecksum: bytes.Clone(
						plan.InitialProjectionChecksum,
					),
					RunChainChecksum: bytes.Clone(
						plan.InitialRunChainChecksum,
					),
				},
			)
			if encodeErr != nil {
				return encodeErr
			}
			sessionID := actorSessionID
			started, prepareErr = workflow.starter.StartInTransaction(
				txCtx,
				operation.StartRequest{
					Operation: operation.CreateCommand{
						ID:             operationID,
						Kind:           contract.RatingReratingOperationKind,
						IdempotencyKey: idempotencyKey,
						RequestHash:    requestHash,
						Request:        encodedRequest,
						Total:          plan.ThroughSequence,
						BatchSize:      workflow.batchSize,
						Checkpoint:     checkpoint,
						ActorUserID:    actorUserID,
						ActorSessionID: &sessionID,
						Reason:         reason,
						CorrelationID:  correlation.FromContext(txCtx),
					},
					Audit: audit.Entry{
						Action:   "contest.rating.rerating.start",
						Resource: contract.AggregateRatingProjection,
						Before: map[string]any{
							"projection_revision":      plan.SourceProjectionRevision,
							"projection_generation_id": plan.SourceProjectionGenerationID.String(),
							"run_chain_id":             plan.SourceRunChainID.String(),
						},
						After: map[string]any{
							"trigger_contest_id":              plan.TriggerContestID.String(),
							"from_sequence":                   plan.FromSequence,
							"through_sequence":                plan.ThroughSequence,
							"target_projection_generation_id": plan.TargetProjectionGenerationID.String(),
							"target_run_chain_id":             plan.TargetRunChainID.String(),
						},
					},
				},
			)
			return prepareErr
		},
	)
	if err != nil {
		if errors.Is(err, operation.ErrConflict) {
			existing, replayErr := workflow.operations.Get(
				ctx,
				operationID,
			)
			if replayErr == nil {
				return workflow.replayStart(
					existing,
					actorUserID,
					actorSessionID,
					idempotencyKey,
					triggerContestID,
					reason,
				)
			}
		}
		return nil, err
	}
	projected, err := operation.Project(started.Operation)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"historical rating rebuild scheduled",
		zap.String("operation_id", projected.ID.String()),
		zap.String("trigger_contest_id", triggerContestID.String()),
		zap.Int64("from_sequence", plan.FromSequence),
		zap.Int64("through_sequence", plan.ThroughSequence),
		zap.Bool("replayed", started.Replay),
	)
	return ratingReratingStartResponse(
		projected,
		plan,
		started.Replay,
	), nil
}

func (workflow *RatingReratingWorkflow) executeBatch(
	ctx context.Context,
	request ratingReratingRequest,
	input operation.BatchInput,
) (operation.BatchResult, error) {
	if workflow == nil || workflow.repository == nil ||
		workflow.outbox == nil || workflow.now == nil ||
		workflow.newID == nil || ctx == nil ||
		input.OperationID == uuid.Nil ||
		input.OperationID != request.OperationID ||
		input.Sequence < 1 || input.Limit < 1 {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_rating_rerating",
			ErrInvalidRatingRerating,
		)
	}
	plan := request.plan()
	if err := validateRatingReratingRequest(request); err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_rating_rerating",
			err,
		)
	}
	checkpoint, err := decodeRatingReratingCheckpoint(
		input.Checkpoint,
		plan,
	)
	if err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_rating_rerating_checkpoint",
			err,
		)
	}
	now := workflow.now().UTC()
	if err := workflow.repository.EnsureReratingTargets(
		ctx,
		plan,
		now,
	); err != nil {
		return operation.BatchResult{}, classifyRatingReratingError(err)
	}
	var processed int64
	for processed < int64(input.Limit) &&
		checkpoint.NextSequence <= plan.ThroughSequence {
		sequence := checkpoint.NextSequence
		var step *entity.RatingReratingStepResult
		if sequence < plan.FromSequence {
			step, err = workflow.repository.ReplayReratingPrefix(
				ctx,
				plan,
				sequence,
				checkpoint.ProjectionChecksum,
				checkpoint.RunChainChecksum,
				now,
			)
		} else {
			step, err = workflow.recalculateSequence(
				ctx,
				plan,
				sequence,
				checkpoint,
				now,
			)
		}
		if err != nil {
			return operation.BatchResult{},
				classifyRatingReratingError(err)
		}
		if step == nil || step.RatingRunID == uuid.Nil {
			return operation.BatchResult{}, operation.Terminal(
				"invalid_rating_rerating_step",
				ErrInvalidRatingRerating,
			)
		}
		checkpoint.ProjectionChecksum = bytes.Clone(
			step.ProjectionChecksum,
		)
		checkpoint.RunChainChecksum = bytes.Clone(
			step.RunChainChecksum,
		)
		checkpoint.NextSequence++
		processed++
	}
	done := checkpoint.NextSequence > plan.ThroughSequence
	if done {
		if err := workflow.repository.ActivateRerating(
			ctx,
			entity.ActivateRatingRerating{
				Plan:               plan,
				ProjectionChecksum: checkpoint.ProjectionChecksum,
				RunChainChecksum:   checkpoint.RunChainChecksum,
				ActivatedAt:        now,
			},
		); err != nil {
			return operation.BatchResult{},
				classifyRatingReratingError(err)
		}
		eventID, idErr := workflow.newID()
		if idErr != nil || eventID == uuid.Nil {
			return operation.BatchResult{}, operation.Retryable(
				"rating_rerating_identity_unavailable",
				ratingReratingRetryDelay,
				idErr,
			)
		}
		payload, encodeErr := json.Marshal(
			contract.RatingProjectionActivatedV1{
				EventID:                eventID,
				OperationID:            plan.OperationID,
				TriggerContestID:       plan.TriggerContestID,
				FromSequence:           plan.FromSequence,
				ThroughSequence:        plan.ThroughSequence,
				ProjectionRevision:     plan.SourceProjectionRevision + 1,
				ProjectionGenerationID: plan.TargetProjectionGenerationID,
				RunChainID:             plan.TargetRunChainID,
				AlgorithmVersion:       plan.AlgorithmVersion,
				OccurredAt:             now,
			},
		)
		if encodeErr != nil {
			return operation.BatchResult{}, operation.Terminal(
				"invalid_rating_rerating_event",
				encodeErr,
			)
		}
		if appendErr := workflow.outbox.Append(
			ctx,
			eventing.Event{
				ID:               eventID,
				Type:             contract.EventRatingProjectionActivatedV1,
				AggregateType:    contract.AggregateRatingProjection,
				AggregateID:      plan.TargetProjectionGenerationID,
				AggregateVersion: plan.SourceProjectionRevision + 1,
				Payload:          payload,
				OccurredAt:       now,
			},
		); appendErr != nil {
			return operation.BatchResult{}, operation.Retryable(
				"rating_rerating_event_unavailable",
				ratingReratingRetryDelay,
				appendErr,
			)
		}
	}
	nextCheckpoint, err := json.Marshal(checkpoint)
	if err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_rating_rerating_checkpoint",
			err,
		)
	}
	logger.FromContext(ctx).Debug(
		"historical rating rebuild batch applied",
		zap.String("operation_id", input.OperationID.String()),
		zap.Int64("batch_sequence", input.Sequence),
		zap.Int64("processed_count", processed),
		zap.Int64("next_rating_sequence", checkpoint.NextSequence),
		zap.Bool("activated", done),
	)
	if done {
		logger.FromContext(ctx).Info(
			"historical rating projection activated",
			zap.String("operation_id", input.OperationID.String()),
			zap.String(
				"projection_generation_id",
				plan.TargetProjectionGenerationID.String(),
			),
			zap.String(
				"run_chain_id",
				plan.TargetRunChainID.String(),
			),
			zap.Int64(
				"projection_revision",
				plan.SourceProjectionRevision+1,
			),
		)
	}
	return operation.BatchResult{
		Processed:      processed,
		Succeeded:      processed,
		NextCheckpoint: nextCheckpoint,
		Done:           done,
	}, nil
}

func (workflow *RatingReratingWorkflow) recalculateSequence(
	ctx context.Context,
	plan entity.RatingReratingPlan,
	sequence int64,
	checkpoint ratingReratingCheckpoint,
	now time.Time,
) (*entity.RatingReratingStepResult, error) {
	snapshot, err := workflow.repository.PrepareReratingSequence(
		ctx,
		plan,
		sequence,
		now,
	)
	if err != nil {
		return nil, err
	}
	inputs := make([]RatingInput, len(snapshot.Participants))
	usernames := make(
		map[uuid.UUID]string,
		len(snapshot.Participants),
	)
	for index, participant := range snapshot.Participants {
		inputs[index] = RatingInput{
			UserID:                participant.UserID,
			SolvedCount:           participant.SolvedCount,
			PenaltySeconds:        participant.PenaltySeconds,
			PreviousRating:        participant.PreviousRating,
			PreviousRatedContests: participant.PreviousRatedContests,
		}
		usernames[participant.UserID] = participant.Username
	}
	calculated, err := CalculateRatingDeltas(inputs)
	if err != nil {
		return nil, err
	}
	results := make([]entity.RatingRunResult, len(calculated))
	for index, result := range calculated {
		results[index] = entity.RatingRunResult{
			UserID:                result.UserID,
			Username:              usernames[result.UserID],
			PreviousRating:        result.PreviousRating,
			PreviousRatedContests: result.PreviousRatedContests,
			Rank:                  result.Rank,
			ActualRank:            result.ActualRank,
			ExpectedRank:          result.ExpectedRank,
			Delta:                 result.Delta,
			NewRating:             result.NewRating,
		}
	}
	runID, err := workflow.newID()
	if err != nil || runID == uuid.Nil {
		return nil, fmt.Errorf("generate rerated run identity: %w", err)
	}
	settlement := entity.RatingSettlementSnapshot{
		ContestID:                  snapshot.ContestID,
		RatingSequence:             snapshot.RatingSequence,
		RatingSequenceSlotID:       snapshot.RatingSequenceSlotID,
		RunVersion:                 snapshot.RunVersion,
		StandingsProjectionVersion: snapshot.StandingsProjectionVersion,
		AlgorithmVersion:           snapshot.AlgorithmVersion,
		Participants:               snapshot.Participants,
	}
	return workflow.repository.ApplyReratingSequence(
		ctx,
		entity.ApplyRatingReratingSequence{
			Plan:                       plan,
			Snapshot:                   *snapshot,
			RunID:                      runID,
			InputChecksum:              ratingSettlementInputChecksum(settlement),
			Results:                    results,
			PreviousProjectionChecksum: checkpoint.ProjectionChecksum,
			PreviousRunChainChecksum:   checkpoint.RunChainChecksum,
			AppliedAt:                  now,
		},
	)
}

func (workflow *RatingReratingWorkflow) replayStart(
	existing operation.Operation,
	actorUserID uuid.UUID,
	actorSessionID uuid.UUID,
	idempotencyKey uuid.UUID,
	triggerContestID uuid.UUID,
	reason string,
) (*contract.RatingReratingStartResponse, error) {
	request, err := decodeRatingReratingRequest(existing.Request)
	if err != nil ||
		existing.Kind != contract.RatingReratingOperationKind ||
		existing.IdempotencyKey != idempotencyKey ||
		existing.ActorUserID != actorUserID ||
		existing.ActorSessionID == nil ||
		*existing.ActorSessionID != actorSessionID ||
		existing.Reason != reason ||
		request.TriggerContestID != triggerContestID ||
		request.ActorUserID != actorUserID ||
		request.Reason != reason {
		return nil, operation.ErrConflict
	}
	projected, err := operation.Project(existing)
	if err != nil {
		return nil, err
	}
	return ratingReratingStartResponse(
		projected,
		request.plan(),
		true,
	), nil
}

func ratingReratingStartResponse(
	projected operation.Projection,
	plan entity.RatingReratingPlan,
	replayed bool,
) *contract.RatingReratingStartResponse {
	return &contract.RatingReratingStartResponse{
		Operation:          projected,
		TriggerContestID:   plan.TriggerContestID,
		FromSequence:       plan.FromSequence,
		ThroughSequence:    plan.ThroughSequence,
		SourceRevision:     plan.SourceProjectionRevision,
		TargetGenerationID: plan.TargetProjectionGenerationID,
		TargetRunChainID:   plan.TargetRunChainID,
		AlgorithmVersion:   plan.AlgorithmVersion,
		IdempotentReplay:   replayed,
	}
}

func ratingReratingRequestFromPlan(
	plan entity.RatingReratingPlan,
) ratingReratingRequest {
	return ratingReratingRequest{
		OperationID:                  plan.OperationID,
		TriggerContestID:             plan.TriggerContestID,
		ActorUserID:                  plan.ActorUserID,
		Reason:                       plan.Reason,
		FromSequence:                 plan.FromSequence,
		ThroughSequence:              plan.ThroughSequence,
		SourceProjectionRevision:     plan.SourceProjectionRevision,
		SourceProjectionGenerationID: plan.SourceProjectionGenerationID,
		SourceRunChainID:             plan.SourceRunChainID,
		SourceProjectionChecksum: bytes.Clone(
			plan.SourceProjectionChecksum,
		),
		SourceRunChainChecksum: bytes.Clone(
			plan.SourceRunChainChecksum,
		),
		TargetProjectionGenerationID: plan.TargetProjectionGenerationID,
		TargetRunChainID:             plan.TargetRunChainID,
		AlgorithmVersion:             plan.AlgorithmVersion,
		InitialProjectionChecksum: bytes.Clone(
			plan.InitialProjectionChecksum,
		),
		InitialRunChainChecksum: bytes.Clone(
			plan.InitialRunChainChecksum,
		),
	}
}

func (request ratingReratingRequest) plan() entity.RatingReratingPlan {
	return entity.RatingReratingPlan{
		OperationID:                  request.OperationID,
		TriggerContestID:             request.TriggerContestID,
		ActorUserID:                  request.ActorUserID,
		Reason:                       request.Reason,
		FromSequence:                 request.FromSequence,
		ThroughSequence:              request.ThroughSequence,
		SourceProjectionRevision:     request.SourceProjectionRevision,
		SourceProjectionGenerationID: request.SourceProjectionGenerationID,
		SourceRunChainID:             request.SourceRunChainID,
		SourceProjectionChecksum: bytes.Clone(
			request.SourceProjectionChecksum,
		),
		SourceRunChainChecksum: bytes.Clone(
			request.SourceRunChainChecksum,
		),
		TargetProjectionGenerationID: request.TargetProjectionGenerationID,
		TargetRunChainID:             request.TargetRunChainID,
		AlgorithmVersion:             request.AlgorithmVersion,
		InitialProjectionChecksum: bytes.Clone(
			request.InitialProjectionChecksum,
		),
		InitialRunChainChecksum: bytes.Clone(
			request.InitialRunChainChecksum,
		),
	}
}

func validateRatingReratingRequest(
	request ratingReratingRequest,
) error {
	plan := request.plan()
	if plan.OperationID == uuid.Nil ||
		plan.TriggerContestID == uuid.Nil ||
		plan.ActorUserID == uuid.Nil ||
		!utf8.ValidString(plan.Reason) ||
		len(strings.TrimSpace(plan.Reason)) < 3 ||
		len(plan.Reason) > operation.MaxReasonBytes ||
		plan.FromSequence < 1 ||
		plan.ThroughSequence < plan.FromSequence ||
		plan.ThroughSequence >
			contract.MaximumRatingReratingSequences ||
		plan.SourceProjectionRevision < 1 ||
		plan.SourceProjectionGenerationID == uuid.Nil ||
		plan.SourceRunChainID == uuid.Nil ||
		plan.TargetProjectionGenerationID == uuid.Nil ||
		plan.TargetRunChainID == uuid.Nil ||
		plan.AlgorithmVersion != RatingAlgorithmVersion ||
		len(plan.SourceProjectionChecksum) != 32 ||
		len(plan.SourceRunChainChecksum) != 32 ||
		len(plan.InitialProjectionChecksum) != 32 ||
		len(plan.InitialRunChainChecksum) != 32 {
		return ErrInvalidRatingRerating
	}
	return nil
}

func decodeRatingReratingRequest(
	raw json.RawMessage,
) (ratingReratingRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request ratingReratingRequest
	if err := decoder.Decode(&request); err != nil {
		return ratingReratingRequest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ratingReratingRequest{}, ErrInvalidRatingRerating
	}
	if err := validateRatingReratingRequest(request); err != nil {
		return ratingReratingRequest{}, err
	}
	return request, nil
}

func decodeRatingReratingCheckpoint(
	raw json.RawMessage,
	plan entity.RatingReratingPlan,
) (ratingReratingCheckpoint, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var checkpoint ratingReratingCheckpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return ratingReratingCheckpoint{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ratingReratingCheckpoint{},
			operation.ErrInvalidCheckpoint
	}
	if checkpoint.NextSequence < 1 ||
		checkpoint.NextSequence > plan.ThroughSequence+1 ||
		len(checkpoint.ProjectionChecksum) != 32 ||
		len(checkpoint.RunChainChecksum) != 32 {
		return ratingReratingCheckpoint{},
			operation.ErrInvalidCheckpoint
	}
	if checkpoint.NextSequence == 1 &&
		(!bytes.Equal(
			checkpoint.ProjectionChecksum,
			plan.InitialProjectionChecksum,
		) ||
			!bytes.Equal(
				checkpoint.RunChainChecksum,
				plan.InitialRunChainChecksum,
			)) {
		return ratingReratingCheckpoint{},
			operation.ErrInvalidCheckpoint
	}
	return checkpoint, nil
}

func classifyRatingReratingError(err error) error {
	switch {
	case errors.Is(err, ports.ErrRatingReratingConflict),
		errors.Is(err, ports.ErrRatingNotAvailable):
		return operation.Terminal(
			"rating_rerating_fence_changed",
			err,
		)
	case errors.Is(err, ports.ErrRatingReratingInvariant):
		return operation.Terminal(
			"rating_rerating_invariant_failed",
			err,
		)
	case errors.Is(err, ErrInvalidRatingRerating),
		errors.Is(err, ErrInvalidRatingInput):
		return operation.Terminal(
			"invalid_rating_rerating",
			err,
		)
	case errors.Is(err, ports.ErrRatingSettlementNotReady),
		errors.Is(err, ports.ErrRatingSequenceBlocked):
		return operation.Retryable(
			"rating_rerating_not_ready",
			ratingReratingRetryDelay,
			err,
		)
	default:
		return operation.Retryable(
			"rating_rerating_unavailable",
			ratingReratingRetryDelay,
			err,
		)
	}
}

func deriveRatingReratingID(
	purpose string,
	source uuid.UUID,
) uuid.UUID {
	return uuid.NewSHA1(
		ratingReratingNamespace,
		[]byte(
			contract.RatingReratingOperationKind+
				"\x00"+purpose+"\x00"+source.String(),
		),
	)
}

var _ ports.RatingReratingStarter = (*RatingReratingWorkflow)(nil)
