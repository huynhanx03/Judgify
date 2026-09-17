package usecase

import (
	"context"
	"testing"

	"github.com/huynhanx03/judgify/internal/ports"
)

type finalizationMetricRecorder struct {
	outcomes []FinalizationMetricOutcome
}

func (recorder *finalizationMetricRecorder) RecordFinalization(
	_ context.Context,
	outcome FinalizationMetricOutcome,
) {
	recorder.outcomes = append(recorder.outcomes, outcome)
}

type inertSubmissionFinalizer struct{}

func (inertSubmissionFinalizer) FinalizeContestantEvaluation(
	context.Context,
	ports.FinalizationRequest,
) (ports.FinalizationResult, error) {
	return ports.FinalizationResult{}, nil
}

func (inertSubmissionFinalizer) FinalizeSystemError(
	context.Context,
	ports.FatalGenerationRequest,
) (ports.FinalizationResult, error) {
	return ports.FinalizationResult{}, nil
}

func TestObservedFinalizationRejectsInvalidAttemptWithoutIdentifiers(t *testing.T) {
	observer := &finalizationMetricRecorder{}
	service, err := NewObservedFinalizationService(inertSubmissionFinalizer{}, observer)
	if err != nil {
		t.Fatalf("NewObservedFinalizationService: %v", err)
	}
	if _, err := service.FinalizeContestantEvaluation(
		context.Background(),
		ports.FinalizationRequest{},
	); err == nil {
		t.Fatal("invalid finalization unexpectedly succeeded")
	}
	if len(observer.outcomes) != 1 || observer.outcomes[0] != FinalizationMetricRejected {
		t.Fatalf("metric outcomes = %v", observer.outcomes)
	}
}
