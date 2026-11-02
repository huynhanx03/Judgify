package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestRegistrationCommandServiceEmitsRedactedAuditAndOutbox(t *testing.T) {
	t.Parallel()
	contestID, userID, actorID, commandID, registrationID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repository := &registrationCommandRepositoryFake{result: ports.RegistrationResult{
		Registration: entity.ContestRegistration{ID: registrationID, ContestID: contestID, UserID: userID,
			Status: entity.RegistrationBanned, Source: entity.RegistrationSourceAdmin, Version: 2},
		ContestVersion: 4, OccurredAt: time.Now().UTC(), CommandID: commandID,
	}}
	auditor := &contestAuditFake{}
	outbox := &contestOutboxFake{}
	commandService, err := NewRegistrationCommandService(RegistrationCommandDependencies{
		Transactions: passContestUnitOfWork{}, Repository: repository, Audit: auditor, Outbox: outbox,
	})
	require.NoError(t, err)

	_, err = commandService.Apply(context.Background(), ports.RegistrationCommand{
		ContestID: contestID, UserID: userID, ActorUserID: actorID,
		ExpectedContestVersion: 4, IdempotencyKey: commandID, Reason: "confirmed abuse",
	}, ports.RegistrationBan)
	require.NoError(t, err)
	require.Len(t, auditor.entries, 1)
	require.Len(t, outbox.events, 1)
	require.NotContains(t, string(outbox.events[0].Payload), "confirmed abuse")
	require.NotEqual(t, [32]byte{}, repository.command.CommandHash)
}

type registrationCommandRepositoryFake struct {
	command  ports.RegistrationCommand
	mutation ports.RegistrationMutation
	result   ports.RegistrationResult
	err      error
}

func (fake *registrationCommandRepositoryFake) Apply(
	_ context.Context,
	command ports.RegistrationCommand,
	mutation ports.RegistrationMutation,
) (ports.RegistrationResult, error) {
	fake.command, fake.mutation = command, mutation
	return fake.result, fake.err
}
