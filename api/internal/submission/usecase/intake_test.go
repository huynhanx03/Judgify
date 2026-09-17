package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/correlation"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestSubmissionCommandServiceBuildsServerOwnedRuntimeCommand(t *testing.T) {
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	executor := &submissionIntakeExecutorFake{
		result: intake.Result{
			SubmissionID:     uuid.New(),
			Generation:       1,
			Phase:            entity.PhaseQueued,
			AggregateVersion: 1,
			SubmittedAt:      now,
			CorrelationID:    "service-test-cid",
		},
	}
	service, err := NewSubmissionCommandService(executor)
	if err != nil {
		t.Fatalf("NewSubmissionCommandService() error = %v", err)
	}
	userID := uuid.New()
	ctx := correlation.WithContext(
		context.Background(),
		"service-test-cid",
	)
	response, err := service.Submit(
		ctx,
		userID,
		"service-test-key",
		&contract.CreateSubmissionCommand{
			ProblemID:  uuid.New(),
			RuntimeKey: "go",
			SourceCode: "package main\nfunc main() {}",
		},
	)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if response.ID != executor.result.SubmissionID ||
		executor.command.UserID != userID ||
		executor.command.RuntimeKey != "go" ||
		executor.command.CorrelationID != "service-test-cid" ||
		string(executor.command.Source) != "package main\nfunc main() {}" {
		t.Fatalf("response=%+v command=%+v", response, executor.command)
	}
}

func TestSubmissionCommandServiceMapsCapacityToRetryableApplicationCode(t *testing.T) {
	executor := &submissionIntakeExecutorFake{
		err: intake.ErrCapabilityUnavailable,
	}
	service, err := NewSubmissionCommandService(executor)
	if err != nil {
		t.Fatalf("NewSubmissionCommandService() error = %v", err)
	}
	ctx := correlation.WithContext(
		context.Background(),
		"service-test-cid",
	)
	_, err = service.Submit(
		ctx,
		uuid.New(),
		"service-test-key",
		&contract.CreateSubmissionCommand{
			ProblemID:  uuid.New(),
			RuntimeKey: "go",
			SourceCode: "package main\nfunc main() {}",
		},
	)
	var appError *apperr.AppError
	if !errors.As(err, &appError) ||
		appError.Code != ports.CodeJudgeCapacityUnavailable {
		t.Fatalf("Submit() error = %v, want judge capacity code", err)
	}
}

type submissionIntakeExecutorFake struct {
	command intake.Command
	result  intake.Result
	err     error
	calls   int
}

func (fake *submissionIntakeExecutorFake) Execute(
	_ context.Context,
	command intake.Command,
) (intake.Result, error) {
	fake.calls++
	fake.command = command
	return fake.result, fake.err
}
