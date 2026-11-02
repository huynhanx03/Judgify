package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestJudgeOperationsRejudgeRejectsUnsafeCommandBeforePersistence(t *testing.T) {
	t.Parallel()
	repository := &operationsRepositoryStub{}
	service, err := NewJudgeOperations(repository)
	require.NoError(t, err)

	_, err = service.Rejudge(context.Background(), validOperationActorForTest(), ports.RejudgeCommand{
		SubmissionID: uuid.New(), ExpectedVersion: 2, IdempotencyKey: uuid.New(),
		Reason: "\t",
	})
	require.ErrorIs(t, err, ErrInvalidJudgeOperation)
	require.Zero(t, repository.rejudgeCalls)
}

func TestJudgeOperationsCancelPassesFencedActorAndCommand(t *testing.T) {
	t.Parallel()
	repository := &operationsRepositoryStub{cancelResult: ports.OperationResult{
		SubmissionID: uuid.New(), Generation: 3, AggregateVersion: 12,
		Phase: entity.PhaseCancelled, Outcome: "cancelled",
	}}
	service, err := NewJudgeOperations(repository)
	require.NoError(t, err)
	actor := validOperationActorForTest()
	command := ports.CancelCommand{
		SubmissionID: repository.cancelResult.SubmissionID, Generation: 3,
		ExpectedVersion: 11, Reason: "duplicate test data was discovered",
	}

	result, err := service.Cancel(context.Background(), actor, command)
	require.NoError(t, err)
	require.Equal(t, repository.cancelResult, result)
	require.Equal(t, 1, repository.cancelCalls)
	require.Equal(t, actor, repository.cancelActor)
	require.Equal(t, command, repository.cancelCommand)
}

func TestJudgeOperationsMapsStateConflict(t *testing.T) {
	t.Parallel()
	service, err := NewJudgeOperations(&operationsRepositoryStub{rejudgeErr: ports.ErrOperationConflict})
	require.NoError(t, err)

	_, err = service.Rejudge(context.Background(), validOperationActorForTest(), ports.RejudgeCommand{
		SubmissionID: uuid.New(), ExpectedVersion: 1, IdempotencyKey: uuid.New(),
		Reason: "re-evaluate after checker correction",
	})
	require.ErrorIs(t, err, ports.ErrOperationConflict)
}

func validOperationActorForTest() ports.OperationActor {
	sessionID := uuid.New()
	return ports.OperationActor{UserID: uuid.New(), SessionID: &sessionID}
}

type operationsRepositoryStub struct {
	rejudgeResult  ports.OperationResult
	rejudgeErr     error
	cancelResult   ports.OperationResult
	cancelErr      error
	rejudgeCalls   int
	cancelCalls    int
	rejudgeActor   ports.OperationActor
	rejudgeCommand ports.RejudgeCommand
	cancelActor    ports.OperationActor
	cancelCommand  ports.CancelCommand
}

func (stub *operationsRepositoryStub) Rejudge(
	_ context.Context,
	actor ports.OperationActor,
	command ports.RejudgeCommand,
) (ports.OperationResult, error) {
	stub.rejudgeCalls++
	stub.rejudgeActor = actor
	stub.rejudgeCommand = command
	if stub.rejudgeErr != nil {
		return ports.OperationResult{}, stub.rejudgeErr
	}
	return stub.rejudgeResult, nil
}

func (stub *operationsRepositoryStub) Cancel(
	_ context.Context,
	actor ports.OperationActor,
	command ports.CancelCommand,
) (ports.OperationResult, error) {
	stub.cancelCalls++
	stub.cancelActor = actor
	stub.cancelCommand = command
	if stub.cancelErr != nil {
		return ports.OperationResult{}, stub.cancelErr
	}
	return stub.cancelResult, nil
}

var _ ports.SubmissionOperationsRepository = (*operationsRepositoryStub)(nil)
