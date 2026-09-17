package judge

import (
	"context"
	"errors"
	"testing"
	"time"

	cultivationports "github.com/huynhanx03/judgify/internal/ports"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

type reconciliationProjector struct {
	results []int
	err     error
	calls   int
}

func (*reconciliationProjector) ApplySolveTransition(
	context.Context,
	submissioncontract.ProblemSolveStatusChangedV1,
) (cultivationports.SolveRewardResult, error) {
	return cultivationports.SolveRewardResult{}, nil
}

func (projector *reconciliationProjector) ReconcileSolveRewards(
	context.Context,
	int,
) (int, error) {
	projector.calls++
	if projector.err != nil {
		return 0, projector.err
	}
	if len(projector.results) == 0 {
		return 0, nil
	}
	result := projector.results[0]
	projector.results = projector.results[1:]
	return result, nil
}

func TestStartupSolveRewardReconciliationConvergesBeforeLiveDelivery(t *testing.T) {
	projector := &reconciliationProjector{results: []int{2, 1}}
	processed, err := reconcileSolveRewards(
		context.Background(),
		projector,
		SolveRewardWorkerOptions{
			ReconcileInterval:   time.Minute,
			ReconcileBatch:      2,
			MaxReconcileBatches: 4,
		},
	)
	if err != nil || processed != 3 || projector.calls != 2 {
		t.Fatalf(
			"startup reconciliation = processed %d, calls %d, error %v",
			processed,
			projector.calls,
			err,
		)
	}
}

func TestStartupSolveRewardReconciliationFailsClosedWhenBudgetIsExhausted(t *testing.T) {
	projector := &reconciliationProjector{results: []int{2, 2}}
	processed, err := reconcileSolveRewards(
		context.Background(),
		projector,
		SolveRewardWorkerOptions{
			ReconcileInterval:   time.Minute,
			ReconcileBatch:      2,
			MaxReconcileBatches: 2,
		},
	)
	if processed != 4 || !errors.Is(
		err,
		ErrSolveRewardReconciliationIncomplete,
	) {
		t.Fatalf("budget exhaustion = processed %d, error %v", processed, err)
	}
}
