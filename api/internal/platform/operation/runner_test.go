package operation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type runnerTxKey struct{}

type runnerTestUnitOfWork struct{}

func (runnerTestUnitOfWork) Within(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	return fn(context.WithValue(ctx, runnerTxKey{}, true))
}

type runnerTestRepository struct {
	operation  Operation
	found      bool
	committed  BatchResult
	retry      RetryDirective
	failedCode string
	sawTx      bool
}

func (repository *runnerTestRepository) LockStart(
	context.Context,
	CreateCommand,
) (Operation, bool, error) {
	return Operation{}, false, nil
}

func (repository *runnerTestRepository) Create(
	context.Context,
	CreateCommand,
) (CreateResult, error) {
	return CreateResult{}, nil
}

func (repository *runnerTestRepository) Get(
	context.Context,
	uuid.UUID,
) (Operation, error) {
	return repository.operation, nil
}

func (repository *runnerTestRepository) List(
	context.Context,
	ListFilter,
) (Page, error) {
	return Page{}, nil
}

func (repository *runnerTestRepository) Claim(
	context.Context,
	ClaimCommand,
) (Operation, bool, error) {
	return repository.operation, repository.found, nil
}

func (repository *runnerTestRepository) LockClaim(
	ctx context.Context,
	_ Lease,
) (Operation, error) {
	repository.sawTx, _ = ctx.Value(runnerTxKey{}).(bool)
	return repository.operation, nil
}

func (repository *runnerTestRepository) SettleRequestedControl(
	context.Context,
	Lease,
) (Operation, bool, error) {
	return Operation{}, false, nil
}

func (repository *runnerTestRepository) CommitBatch(
	ctx context.Context,
	_ Lease,
	result BatchResult,
	_ time.Time,
) (Operation, error) {
	repository.sawTx = repository.sawTx &&
		ctx.Value(runnerTxKey{}) == true
	repository.committed = result
	next := repository.operation
	next.Processed += result.Processed
	next.Succeeded += result.Succeeded
	next.Failed += result.Failed
	next.Checkpoint = append([]byte(nil), result.NextCheckpoint...)
	next.NextBatchSequence++
	next.Version++
	next.Lease = nil
	next.UpdatedAt = next.UpdatedAt.Add(time.Second)
	if result.Done {
		next.Status = StatusCompleted
		finished := next.UpdatedAt
		next.FinishedAt = &finished
	}
	repository.operation = next
	return next, nil
}

func (repository *runnerTestRepository) ReleaseRetry(
	_ context.Context,
	_ Lease,
	directive RetryDirective,
) error {
	repository.retry = directive
	return nil
}

func (repository *runnerTestRepository) MarkFailed(
	_ context.Context,
	_ Lease,
	code string,
) error {
	repository.failedCode = code
	return nil
}

func (repository *runnerTestRepository) LockControl(
	context.Context,
	ControlCommand,
) (ControlResult, bool, error) {
	return ControlResult{}, false, nil
}

func (repository *runnerTestRepository) ApplyControl(
	context.Context,
	ControlCommand,
	uuid.UUID,
) (Operation, error) {
	return Operation{}, nil
}

func TestRunnerCommitsDomainBatchAndCheckpointInSameUnitOfWork(t *testing.T) {
	t.Parallel()

	current := validTestOperation(t)
	current.Total = 1
	current.Lease = &Lease{
		OperationID: current.ID, Token: uuid.New(),
		Owner: "operation-worker", Until: time.Now().Add(time.Minute),
	}
	repository := &runnerTestRepository{operation: current, found: true}
	registry := NewRegistry()
	if err := RegisterJSON(registry, current.Kind, func(
		ctx context.Context,
		_ struct {
			Mode string `json:"mode"`
		},
		input BatchInput,
	) (BatchResult, error) {
		if ctx.Value(runnerTxKey{}) != true {
			t.Fatal("domain executor ran outside operation UnitOfWork")
		}
		if input.Sequence != 1 || input.Limit != 1 {
			t.Fatalf("batch input = %#v", input)
		}
		return BatchResult{
			Processed: 1, Succeeded: 1,
			NextCheckpoint: []byte(`{"after":"item-1"}`),
			Done:           true,
		}, nil
	}); err != nil {
		t.Fatalf("RegisterJSON: %v", err)
	}
	runner, err := NewRunner(
		repository,
		runnerTestUnitOfWork{},
		registry,
		RunnerOptions{
			Owner:         "operation-worker",
			LeaseDuration: 10 * time.Second,
			BatchTimeout:  5 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	claimed, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !claimed || !repository.sawTx || repository.committed.Processed != 1 {
		t.Fatalf(
			"result claimed=%t sawTx=%t committed=%#v",
			claimed,
			repository.sawTx,
			repository.committed,
		)
	}
}

func TestRunnerPersistsOnlyRetryClassification(t *testing.T) {
	t.Parallel()

	current := validTestOperation(t)
	current.Lease = &Lease{
		OperationID: current.ID, Token: uuid.New(),
		Owner: "operation-worker", Until: time.Now().Add(time.Minute),
	}
	repository := &runnerTestRepository{operation: current, found: true}
	registry := NewRegistry()
	privateCause := errors.New("provider returned a private diagnostic")
	if err := RegisterJSON(registry, current.Kind, func(
		context.Context,
		struct {
			Mode string `json:"mode"`
		},
		BatchInput,
	) (BatchResult, error) {
		return BatchResult{}, Retryable("dependency_unavailable", time.Second, privateCause)
	}); err != nil {
		t.Fatalf("RegisterJSON: %v", err)
	}
	runner, err := NewRunner(
		repository,
		runnerTestUnitOfWork{},
		registry,
		RunnerOptions{
			Owner:         "operation-worker",
			LeaseDuration: 10 * time.Second,
			BatchTimeout:  5 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	if _, err := runner.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if repository.retry.Code != "dependency_unavailable" {
		t.Fatalf("retry = %#v", repository.retry)
	}
}
