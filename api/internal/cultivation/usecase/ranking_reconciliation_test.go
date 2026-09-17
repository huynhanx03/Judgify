package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/correlation"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
)

type rankingReconciliationRepositoryStub struct {
	plan             entity.RankingReconciliationPlan
	state            entity.RankingReconciliationState
	batch            entity.RankingReconciliationBatch
	batchQuery       entity.RankingReconciliationBatchQuery
	preparedExpected entity.RankingReconciliationPlan
}

func (repository *rankingReconciliationRepositoryStub) Preview(
	context.Context,
) (entity.RankingReconciliationPlan, error) {
	return repository.plan, nil
}

func (repository *rankingReconciliationRepositoryStub) Prepare(
	_ context.Context,
	_ uuid.UUID,
	expected entity.RankingReconciliationPlan,
) (entity.RankingReconciliationPlan, error) {
	repository.preparedExpected = expected
	return repository.plan, nil
}

func (repository *rankingReconciliationRepositoryStub) ReconcileBatch(
	_ context.Context,
	query entity.RankingReconciliationBatchQuery,
) (entity.RankingReconciliationBatch, error) {
	repository.batchQuery = query
	return repository.batch, nil
}

func (repository *rankingReconciliationRepositoryStub) State(
	context.Context,
) (entity.RankingReconciliationState, error) {
	return repository.state, nil
}

type rankingOperationStarterStub struct {
	request operation.StartRequest
	result  operation.CreateResult
}

func (starter *rankingOperationStarterStub) StartInTransaction(
	_ context.Context,
	request operation.StartRequest,
) (operation.CreateResult, error) {
	starter.request = request
	if request.Operation.AuditID == uuid.Nil {
		request.Operation.AuditID = uuid.MustParse("00000000-0000-0000-0000-000000000116")
	}
	starter.result.Operation = operationFromCreateCommand(request.Operation)
	return starter.result, nil
}

type rankingOperationReaderStub struct {
	operation.Operation
	found bool
	page  operation.Page
}

func (repository *rankingOperationReaderStub) Get(
	context.Context,
	uuid.UUID,
) (operation.Operation, error) {
	if !repository.found {
		return operation.Operation{}, operation.ErrNotFound
	}
	return repository.Operation, nil
}

func (repository *rankingOperationReaderStub) List(
	context.Context,
	operation.ListFilter,
) (operation.Page, error) {
	return repository.page, nil
}

type rankingOperationUnitOfWorkStub struct{}

func (rankingOperationUnitOfWorkStub) Within(
	ctx context.Context,
	work func(context.Context) error,
) error {
	return work(ctx)
}

type rankingReconciliationOutboxStub struct {
	events []eventing.Event
}

func (outbox *rankingReconciliationOutboxStub) Append(
	_ context.Context,
	events ...eventing.Event,
) error {
	outbox.events = append(outbox.events, events...)
	return nil
}

func TestRankingReconciliationStartFreezesDatabasePlanInDurableOperation(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	throughUserID := uuid.MustParse("00000000-0000-0000-0000-000000000111")
	repository := &rankingReconciliationRepositoryStub{plan: entity.RankingReconciliationPlan{
		SourceCutoff:       cutoff,
		ThroughUserID:      throughUserID,
		Total:              42,
		RepairCount:        3,
		ProjectionRevision: 7,
	}}
	starter := &rankingOperationStarterStub{}
	outbox := &rankingReconciliationOutboxStub{}
	workflow, err := NewRankingReconciliationWorkflow(
		repository,
		&rankingOperationReaderStub{},
		starter,
		rankingOperationUnitOfWorkStub{},
		outbox,
		RankingReconciliationOptions{BatchSize: 25},
	)
	require.NoError(t, err)

	ctx := correlation.WithContext(context.Background(), "cid-ranking-reconcile")
	previewCommand := entity.RankingReconciliationPreviewCommand{
		ActorUserID:    uuid.MustParse("00000000-0000-0000-0000-000000000114"),
		ActorSessionID: uuid.MustParse("00000000-0000-0000-0000-000000000115"),
	}
	response, err := workflow.Start(ctx, entity.RankingReconciliationCommand{
		ID:               uuid.MustParse("00000000-0000-0000-0000-000000000113"),
		ActorUserID:      previewCommand.ActorUserID,
		ActorSessionID:   previewCommand.ActorSessionID,
		Reason:           "repair missed projection writes",
		SourceCutoff:     cutoff,
		ExpectedRevision: 7,
		SourceCount:      42,
		RepairCount:      3,
		PreviewChecksum:  entity.RankingReconciliationPreviewChecksum(previewCommand, repository.plan),
	})
	require.NoError(t, err)
	require.Equal(t, int64(42), starter.request.Operation.Total)
	require.Equal(t, cutoff, repository.preparedExpected.SourceCutoff)
	require.Equal(t, uuid.Nil, repository.preparedExpected.ThroughUserID)
	require.Equal(t, int64(3), repository.preparedExpected.RepairCount)
	require.Equal(t, 25, starter.request.Operation.BatchSize)
	require.Equal(t, "cid-ranking-reconcile", starter.request.Operation.CorrelationID)
	require.Equal(t, "cultivation.ranking.reconciliation.start", starter.request.Audit.Action)
	require.Equal(t, cutoff, response.SourceCutoff)
	require.Equal(t, int64(3), response.RepairCount)
	require.Equal(t, int64(7), response.ProjectionRevision)
	require.Equal(t, int64(42), response.Operation.Total)
	require.Len(t, outbox.events, 1)
	require.Equal(t, "cultivation.ranking_reconciliation.started.v1", outbox.events[0].Type)

	request, err := decodeRankingReconciliationRequest(starter.request.Operation.Request)
	require.NoError(t, err)
	require.Equal(t, cutoff, request.SourceCutoff)
	require.Equal(t, throughUserID, request.ThroughUserID)
}

func TestRankingReconciliationPreviewBindsActorAndHidesUpperUserFence(t *testing.T) {
	t.Parallel()

	plan := entity.RankingReconciliationPlan{
		SourceCutoff:       time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC),
		ThroughUserID:      uuid.MustParse("00000000-0000-0000-0000-000000000121"),
		Total:              12,
		RepairCount:        2,
		ProjectionRevision: 4,
	}
	workflow, err := NewRankingReconciliationWorkflow(
		&rankingReconciliationRepositoryStub{plan: plan},
		&rankingOperationReaderStub{},
		&rankingOperationStarterStub{},
		rankingOperationUnitOfWorkStub{},
		&rankingReconciliationOutboxStub{},
		RankingReconciliationOptions{},
	)
	require.NoError(t, err)
	command := entity.RankingReconciliationPreviewCommand{
		ActorUserID:    uuid.MustParse("00000000-0000-0000-0000-000000000122"),
		ActorSessionID: uuid.MustParse("00000000-0000-0000-0000-000000000123"),
	}
	preview, err := workflow.Preview(context.Background(), command)
	require.NoError(t, err)
	require.Equal(t, int64(12), preview.SourceCount)
	require.Equal(t, int64(2), preview.RepairCount)
	encoded, err := json.Marshal(preview)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), plan.ThroughUserID.String())
	checksum := decodeRankingPreviewChecksum(preview.PreviewChecksum)
	require.True(t, entity.RankingReconciliationPreviewMatches(checksum, command, plan))
	command.ActorSessionID = uuid.MustParse("00000000-0000-0000-0000-000000000124")
	require.False(t, entity.RankingReconciliationPreviewMatches(checksum, command, plan))
}

func TestRankingReconciliationBatchAdvancesOpaqueCheckpointExactly(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	throughUserID := uuid.MustParse("00000000-0000-0000-0000-000000000211")
	afterUserID := uuid.MustParse("00000000-0000-0000-0000-000000000210")
	lastUserID := uuid.MustParse("00000000-0000-0000-0000-000000000211")
	repository := &rankingReconciliationRepositoryStub{batch: entity.RankingReconciliationBatch{
		Processed:          2,
		Repaired:           1,
		LastUserID:         lastUserID,
		Done:               false,
		ProjectionRevision: 8,
	}}
	workflow, err := NewRankingReconciliationWorkflow(
		repository,
		&rankingOperationReaderStub{},
		&rankingOperationStarterStub{},
		rankingOperationUnitOfWorkStub{},
		&rankingReconciliationOutboxStub{},
		RankingReconciliationOptions{BatchSize: 2},
	)
	require.NoError(t, err)
	commandID := uuid.MustParse("00000000-0000-0000-0000-000000000212")
	request := rankingReconciliationRequest{
		OperationID:        deriveRankingReconciliationID(commandID),
		CommandID:          commandID,
		ActorUserID:        uuid.MustParse("00000000-0000-0000-0000-000000000213"),
		ActorSessionID:     uuid.MustParse("00000000-0000-0000-0000-000000000214"),
		Reason:             "repair ranking drift",
		SourceCutoff:       cutoff,
		ThroughUserID:      throughUserID,
		Total:              10,
		RepairCount:        4,
		ProjectionRevision: 8,
		PreviewChecksum:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	checkpoint, err := json.Marshal(rankingReconciliationCheckpoint{
		AfterUserID: afterUserID,
		Repaired:    3,
	})
	require.NoError(t, err)

	result, err := workflow.executeBatch(context.Background(), request, operation.BatchInput{
		OperationID: request.OperationID,
		Sequence:    3,
		Checkpoint:  checkpoint,
		Limit:       2,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Processed)
	require.Equal(t, int64(2), result.Succeeded)
	require.False(t, result.Done)
	require.Equal(t, afterUserID, repository.batchQuery.AfterUserID)
	require.Equal(t, throughUserID, repository.batchQuery.ThroughUserID)
	require.Equal(t, cutoff, repository.batchQuery.SourceCutoff)

	next, err := decodeRankingReconciliationCheckpoint(result.NextCheckpoint)
	require.NoError(t, err)
	require.Equal(t, lastUserID, next.AfterUserID)
	require.Equal(t, int64(4), next.Repaired)
}

func TestRankingReconciliationStatusExposesOnlySafeOperationProjection(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	privateRequest, privateHash, err := operation.EncodeRequest(map[string]any{
		"private": "request",
	})
	require.NoError(t, err)
	current := operationFromCreateCommand(operation.CreateCommand{
		ID:             uuid.MustParse("00000000-0000-0000-0000-000000000311"),
		Kind:           RankingReconciliationOperationKind,
		IdempotencyKey: uuid.MustParse("00000000-0000-0000-0000-000000000312"),
		RequestHash:    privateHash,
		Request:        privateRequest,
		Total:          7,
		BatchSize:      5,
		Checkpoint:     json.RawMessage(`{"private":"checkpoint"}`),
		ActorUserID:    uuid.MustParse("00000000-0000-0000-0000-000000000313"),
		AuditID:        uuid.MustParse("00000000-0000-0000-0000-000000000314"),
		Reason:         "private operator reason",
		CorrelationID:  "cid-safe-status",
	})
	workflow, err := NewRankingReconciliationWorkflow(
		&rankingReconciliationRepositoryStub{state: entity.RankingReconciliationState{
			ProjectionRevision: 9,
			LastReconciledAt:   &cutoff,
		}},
		&rankingOperationReaderStub{page: operation.Page{Items: []operation.Operation{current}}},
		&rankingOperationStarterStub{},
		rankingOperationUnitOfWorkStub{},
		&rankingReconciliationOutboxStub{},
		RankingReconciliationOptions{},
	)
	require.NoError(t, err)

	status, err := workflow.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(9), status.ProjectionRevision)
	require.Equal(t, &cutoff, status.LastReconciledAt)
	require.NotNil(t, status.LatestOperation)
	encoded, err := json.Marshal(status)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private operator reason")
	require.NotContains(t, string(encoded), "private checkpoint")
	require.NotContains(t, string(encoded), "private request")
}

func operationFromCreateCommand(command operation.CreateCommand) operation.Operation {
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	return operation.Operation{
		ID:                command.ID,
		Kind:              command.Kind,
		IdempotencyKey:    command.IdempotencyKey,
		RequestHash:       command.RequestHash,
		Request:           command.Request,
		Status:            operation.StatusPending,
		Total:             command.Total,
		BatchSize:         command.BatchSize,
		Checkpoint:        command.Checkpoint,
		NextBatchSequence: 1,
		Version:           1,
		RequestedAction:   "none",
		ActorUserID:       command.ActorUserID,
		ActorSessionID:    command.ActorSessionID,
		AuditID:           command.AuditID,
		Reason:            command.Reason,
		CorrelationID:     command.CorrelationID,
		AvailableAt:       now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}
