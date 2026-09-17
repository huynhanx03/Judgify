package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestLifecycleCommandServiceAppendsAuditAndOutboxInUnitOfWork(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 28, 4, 0, 0, 0, time.UTC)
	contestID, actorID, commandID := uuid.New(), uuid.New(), uuid.New()
	repository := &lifecycleRepositoryFake{result: ports.TransitionResult{
		ContestID: contestID, From: entity.ContestDraft, To: entity.ContestUpcoming,
		Version: 2, OccurredAt: now, CommandID: commandID,
	}}
	auditor := &contestAuditFake{}
	outbox := &contestOutboxFake{}
	service, err := NewLifecycleCommandService(LifecycleCommandDependencies{
		Transactions: passContestUnitOfWork{}, Repository: repository, Audit: auditor, Outbox: outbox,
	})
	require.NoError(t, err)

	result, err := service.Transition(context.Background(), ports.TransitionCommand{
		ContestID: contestID, Target: entity.ContestUpcoming, ExpectedVersion: 1,
		IdempotencyKey: commandID, ActorUserID: actorID, Reason: "publish reviewed contest",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Version)
	require.Len(t, auditor.entries, 1)
	require.Len(t, outbox.events, 1)
	require.Equal(t, contestLifecycleChangedV1, outbox.events[0].Type)
	require.Equal(t, int64(2), outbox.events[0].AggregateVersion)
	require.NotEqual(t, [32]byte{}, repository.command.CommandHash)
}

func TestLifecycleCommandServiceExactReplayDoesNotAppendEffectsAgain(t *testing.T) {
	t.Parallel()

	repository := &lifecycleRepositoryFake{result: ports.TransitionResult{
		ContestID: uuid.New(), From: entity.ContestDraft, To: entity.ContestUpcoming,
		Version: 2, OccurredAt: time.Now().UTC(), CommandID: uuid.New(), ExactReplay: true,
	}}
	auditor := &contestAuditFake{}
	outbox := &contestOutboxFake{}
	service, err := NewLifecycleCommandService(LifecycleCommandDependencies{
		Transactions: passContestUnitOfWork{}, Repository: repository, Audit: auditor, Outbox: outbox,
	})
	require.NoError(t, err)
	command := ports.TransitionCommand{
		ContestID: repository.result.ContestID, Target: entity.ContestUpcoming,
		ExpectedVersion: 1, IdempotencyKey: repository.result.CommandID,
		ActorUserID: uuid.New(), Reason: "publish reviewed contest",
	}

	_, err = service.Transition(context.Background(), command)
	require.NoError(t, err)
	require.Empty(t, auditor.entries)
	require.Empty(t, outbox.events)
}

type passContestUnitOfWork struct{}

func (passContestUnitOfWork) Within(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

type lifecycleRepositoryFake struct {
	command ports.TransitionCommand
	result  ports.TransitionResult
	err     error
}

func (repository *lifecycleRepositoryFake) Transition(
	_ context.Context,
	command ports.TransitionCommand,
) (ports.TransitionResult, error) {
	repository.command = command
	return repository.result, repository.err
}

type contestAuditFake struct{ entries []audit.Entry }

func (fake *contestAuditFake) Append(_ context.Context, entry audit.Entry) (uuid.UUID, error) {
	fake.entries = append(fake.entries, entry)
	return entry.ID, nil
}

type contestOutboxFake struct{ events []eventing.Event }

func (fake *contestOutboxFake) Append(_ context.Context, events ...eventing.Event) error {
	fake.events = append(fake.events, events...)
	return nil
}
