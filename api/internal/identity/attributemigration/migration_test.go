package attributemigration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/operation"
)

type migrationRepositoryStub struct {
	batch        Batch
	cleanupBatch CleanupBatch
	err          error
	cleanupErr   error
	prepared     *Request
	shadows      []ShadowValue
	verified     int
}

func (s *migrationRepositoryStub) Prepare(_ context.Context, r Request) (Request, error) {
	if s.prepared != nil {
		return *s.prepared, nil
	}
	return r, nil
}
func (s *migrationRepositoryStub) CleanupTargetBatch(context.Context, Request, Checkpoint, int) (CleanupBatch, error) {
	return s.cleanupBatch, s.cleanupErr
}
func (s *migrationRepositoryStub) LoadBatch(context.Context, Request, Checkpoint, int) (Batch, error) {
	return s.batch, s.err
}
func (s *migrationRepositoryStub) PutShadow(_ context.Context, v ShadowValue) error {
	s.shadows = append(s.shadows, v)
	return nil
}
func (s *migrationRepositoryStub) VerifyFinal(context.Context, Request) error {
	s.verified++
	return nil
}

type migrationActivatorStub struct{ calls int }

func (s *migrationActivatorStub) ActivateMigratedRevision(context.Context, Request) error {
	s.calls++
	return nil
}

func migrationID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	return id
}
func migrationRequest(t *testing.T) Request {
	id := migrationID(t)
	return Request{OperationID: id, CommandID: id, DefinitionID: migrationID(t), ExpectedVersion: 1, SourceRevisionID: migrationID(t), CandidateRevisionID: migrationID(t), ActorUserID: migrationID(t), ActorSessionID: migrationID(t), Reason: "approved migration", ConfirmationHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SourceCutoff: migrationID(t), SourceCount: 1}
}

func TestExecutorBatchesCheckpointAndActivatesExactlyOnce(t *testing.T) {
	r := migrationRequest(t)
	sourceRevision := entity.AttributeDefinitionRevision{ID: r.SourceRevisionID, DefinitionID: r.DefinitionID, DataType: entity.AttributeValueTypeString, Validation: json.RawMessage(`{}`)}
	candidateRevision := entity.AttributeDefinitionRevision{ID: r.CandidateRevisionID, DefinitionID: r.DefinitionID, DataType: entity.AttributeValueTypeString, Validation: json.RawMessage(`{}`)}
	source := SourceValue{ID: r.SourceCutoff, UserID: migrationID(t), DefinitionID: r.DefinitionID, RevisionID: r.SourceRevisionID, CanonicalValue: json.RawMessage(`"Ada"`), LegacyValue: "Ada", Version: 1}
	repo := &migrationRepositoryStub{batch: Batch{Source: sourceRevision, Candidate: candidateRevision, Values: []SourceValue{source}, Done: true}}
	activator := &migrationActivatorStub{}
	executor, err := NewExecutor(repo, activator)
	require.NoError(t, err)
	checkpoint, _ := json.Marshal(Checkpoint{})
	result, err := executor.executeBatch(context.Background(), r, operation.BatchInput{OperationID: r.OperationID, Sequence: 1, Limit: 1, Checkpoint: checkpoint})
	require.NoError(t, err)
	require.True(t, result.Done)
	require.Equal(t, int64(1), result.Processed)
	require.Len(t, repo.shadows, 1)
	require.Equal(t, 1, repo.verified)
	require.Equal(t, 1, activator.calls)
}

func TestExecutorQuarantinePreventsActivation(t *testing.T) {
	r := migrationRequest(t)
	sourceRevision := entity.AttributeDefinitionRevision{ID: r.SourceRevisionID, DefinitionID: r.DefinitionID, DataType: entity.AttributeValueTypeString, Validation: json.RawMessage(`{}`)}
	candidateRevision := entity.AttributeDefinitionRevision{ID: r.CandidateRevisionID, DefinitionID: r.DefinitionID, DataType: entity.AttributeValueTypeDate, Validation: json.RawMessage(`{}`)}
	source := SourceValue{ID: r.SourceCutoff, UserID: migrationID(t), DefinitionID: r.DefinitionID, RevisionID: r.SourceRevisionID, CanonicalValue: json.RawMessage(`"not-a-date"`), LegacyValue: "not-a-date", Version: 1}
	repo := &migrationRepositoryStub{batch: Batch{Source: sourceRevision, Candidate: candidateRevision, Values: []SourceValue{source}, Done: true}}
	activator := &migrationActivatorStub{}
	executor, _ := NewExecutor(repo, activator)
	checkpoint, _ := json.Marshal(Checkpoint{})
	result, err := executor.executeBatch(context.Background(), r, operation.BatchInput{OperationID: r.OperationID, Sequence: 1, Limit: 1, Checkpoint: checkpoint})
	require.NoError(t, err)
	require.Equal(t, "attribute_migration_quarantined", result.FailureCode)
	require.Zero(t, repo.verified)
	require.Zero(t, activator.calls)
}

func TestExecutorClassifiesDefinitionAndValueDriftAsTerminal(t *testing.T) {
	for _, cause := range []error{ErrDefinitionDrift, ErrValueDrift} {
		r := migrationRequest(t)
		repo := &migrationRepositoryStub{err: cause}
		executor, _ := NewExecutor(repo, &migrationActivatorStub{})
		checkpoint, _ := json.Marshal(Checkpoint{})
		_, err := executor.executeBatch(context.Background(), r, operation.BatchInput{OperationID: r.OperationID, Sequence: 1, Limit: 1, Checkpoint: checkpoint})
		require.Error(t, err)
	}
}

func TestExecutorCleansTargetBeforeCopyingSource(t *testing.T) {
	r := migrationRequest(t)
	targetID := migrationID(t)
	r.TargetCutoff = targetID
	r.TargetCount = 1
	repo := &migrationRepositoryStub{cleanupBatch: CleanupBatch{
		ValueIDs: []uuid.UUID{targetID},
		Done:     true,
	}}
	activator := &migrationActivatorStub{}
	executor, err := NewExecutor(repo, activator)
	require.NoError(t, err)
	checkpoint, err := json.Marshal(Checkpoint{})
	require.NoError(t, err)

	cleanupResult, err := executor.executeBatch(context.Background(), r, operation.BatchInput{
		OperationID: r.OperationID,
		Sequence:    1,
		Limit:       1,
		Checkpoint:  checkpoint,
	})
	require.NoError(t, err)
	require.False(t, cleanupResult.Done)
	require.Equal(t, int64(1), cleanupResult.Processed)
	var copyCheckpoint Checkpoint
	require.NoError(t, json.Unmarshal(cleanupResult.NextCheckpoint, &copyCheckpoint))
	require.Equal(t, PhaseCopySource, copyCheckpoint.Phase)
	require.Equal(t, int64(1), copyCheckpoint.TargetProcessed)

	source := SourceValue{
		ID:             r.SourceCutoff,
		UserID:         migrationID(t),
		DefinitionID:   r.DefinitionID,
		RevisionID:     r.SourceRevisionID,
		CanonicalValue: json.RawMessage(`"Ada"`),
		LegacyValue:    "Ada",
		Version:        1,
	}
	repo.batch = Batch{
		Source: entity.AttributeDefinitionRevision{
			ID:           r.SourceRevisionID,
			DefinitionID: r.DefinitionID,
			DataType:     entity.AttributeValueTypeString,
			Validation:   json.RawMessage(`{}`),
		},
		Candidate: entity.AttributeDefinitionRevision{
			ID:           r.CandidateRevisionID,
			DefinitionID: r.DefinitionID,
			DataType:     entity.AttributeValueTypeString,
			Validation:   json.RawMessage(`{}`),
		},
		Values: []SourceValue{source},
		Done:   true,
	}
	copyResult, err := executor.executeBatch(context.Background(), r, operation.BatchInput{
		OperationID: r.OperationID,
		Sequence:    2,
		Limit:       1,
		Checkpoint:  cleanupResult.NextCheckpoint,
	})
	require.NoError(t, err)
	require.True(t, copyResult.Done)
	require.Len(t, repo.shadows, 1)
	require.Equal(t, 1, repo.verified)
	require.Equal(t, 1, activator.calls)
}

func TestExecutorFinalizesAfterTargetOnlyCleanup(t *testing.T) {
	r := migrationRequest(t)
	r.SourceCutoff = uuid.Nil
	r.SourceCount = 0
	targetID := migrationID(t)
	r.TargetCutoff = targetID
	r.TargetCount = 1
	repo := &migrationRepositoryStub{cleanupBatch: CleanupBatch{
		ValueIDs: []uuid.UUID{targetID},
		Done:     true,
	}}
	activator := &migrationActivatorStub{}
	executor, err := NewExecutor(repo, activator)
	require.NoError(t, err)
	checkpoint, err := json.Marshal(Checkpoint{})
	require.NoError(t, err)

	result, err := executor.executeBatch(context.Background(), r, operation.BatchInput{
		OperationID: r.OperationID,
		Sequence:    1,
		Limit:       1,
		Checkpoint:  checkpoint,
	})
	require.NoError(t, err)
	require.True(t, result.Done)
	require.Empty(t, repo.shadows)
	require.Equal(t, 1, repo.verified)
	require.Equal(t, 1, activator.calls)
}

func TestPrepareRejectsRepositoryIntentRewrite(t *testing.T) {
	intent := migrationRequest(t)
	intent.SourceCutoff = uuid.Nil
	intent.SourceCount = 0
	prepared := intent
	prepared.SourceCutoff = migrationID(t)
	prepared.SourceCount = 1
	prepared.ActorUserID = migrationID(t)
	repo := &migrationRepositoryStub{prepared: &prepared}

	_, err := Prepare(context.Background(), repo, intent)
	require.ErrorIs(t, err, ErrInvalidMigration)
}
