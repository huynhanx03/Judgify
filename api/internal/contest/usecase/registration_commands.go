package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	contestRegistrationAggregateType = contestcontract.AggregateRegistration
	contestRegistrationChangedV1     = contestcontract.EventRegistrationChangedV1
)

type RegistrationCommandDependencies struct {
	Transactions platformtx.UnitOfWork
	Repository   ports.RegistrationCommandRepository
	Audit        audit.Appender
	Outbox       eventing.OutboxAppender
}

type registrationCommandService struct {
	dependencies RegistrationCommandDependencies
}

func NewRegistrationCommandService(dependencies RegistrationCommandDependencies) (ports.RegistrationCommandService, error) {
	if nilContestDependency(dependencies.Transactions) || nilContestDependency(dependencies.Repository) ||
		nilContestDependency(dependencies.Audit) || nilContestDependency(dependencies.Outbox) {
		return nil, ports.ErrInvalidContestCommand
	}
	return &registrationCommandService{dependencies: dependencies}, nil
}

func (service *registrationCommandService) Apply(
	ctx context.Context,
	command ports.RegistrationCommand,
	mutation ports.RegistrationMutation,
) (ports.RegistrationResult, error) {
	if service == nil || ctx == nil || command.ContestID == uuid.Nil || command.UserID == uuid.Nil ||
		command.ActorUserID == uuid.Nil || command.IdempotencyKey == uuid.Nil ||
		command.ExpectedContestVersion < 1 ||
		(command.ExpectedRegistrationVersion != nil && *command.ExpectedRegistrationVersion < 0) ||
		(command.ActorSessionID != nil && *command.ActorSessionID == uuid.Nil) ||
		(mutation != ports.RegistrationRegister && mutation != ports.RegistrationWithdraw && mutation != ports.RegistrationBan) ||
		entity.ValidateContestReason(command.Reason, mutation == ports.RegistrationBan) != nil {
		return ports.RegistrationResult{}, ports.ErrInvalidContestCommand
	}
	if err := ctx.Err(); err != nil {
		return ports.RegistrationResult{}, err
	}
	command.CommandHash = hashRegistrationCommand(command, mutation)
	ctx = correlation.EnsureContext(ctx)
	var result ports.RegistrationResult
	err := service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
		stored, err := service.dependencies.Repository.Apply(txCtx, command, mutation)
		if err != nil {
			return err
		}
		if stored.CommandID != command.IdempotencyKey || stored.Registration.ID == uuid.Nil ||
			stored.Registration.ContestID != command.ContestID || stored.Registration.UserID != command.UserID ||
			stored.Registration.Version < 1 || stored.OccurredAt.IsZero() {
			return ports.ErrContestCommandDenied
		}
		result = stored
		if stored.ExactReplay {
			return nil
		}
		eventID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create registration event identity: %w", err)
		}
		payload, err := json.Marshal(struct {
			ContestID      uuid.UUID                 `json:"contest_id"`
			RegistrationID uuid.UUID                 `json:"registration_id"`
			UserID         uuid.UUID                 `json:"user_id"`
			Status         entity.RegistrationStatus `json:"status"`
			Source         entity.RegistrationSource `json:"source"`
			Version        int64                     `json:"version"`
			OccurredAt     string                    `json:"occurred_at"`
		}{
			ContestID: stored.Registration.ContestID, RegistrationID: stored.Registration.ID,
			UserID: stored.Registration.UserID, Status: stored.Registration.Status,
			Source: stored.Registration.Source, Version: stored.Registration.Version,
			OccurredAt: stored.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
		if err != nil {
			return fmt.Errorf("encode registration event: %w", err)
		}
		actor := command.ActorUserID
		if _, err := service.dependencies.Audit.Append(txCtx, audit.Entry{
			ID: eventID, ActorUserID: &actor, ActorSessionID: command.ActorSessionID,
			Action: contestRegistrationChangedV1, Resource: contestRegistrationAggregateType,
			ResourceID: stored.Registration.ID.String(), Reason: command.Reason,
			Before:        map[string]any{"version": stored.Registration.Version - 1},
			After:         map[string]any{"status": stored.Registration.Status, "version": stored.Registration.Version},
			CorrelationID: correlation.FromContext(txCtx), OccurredAt: stored.OccurredAt,
		}); err != nil {
			return fmt.Errorf("append registration audit: %w", err)
		}
		return service.dependencies.Outbox.Append(txCtx, eventing.Event{
			ID: eventID, Type: contestRegistrationChangedV1,
			AggregateType: contestRegistrationAggregateType, AggregateID: stored.Registration.ID,
			AggregateVersion: stored.Registration.Version, CorrelationID: correlation.FromContext(txCtx),
			Payload: payload, OccurredAt: stored.OccurredAt,
		})
	})
	if err != nil {
		return ports.RegistrationResult{}, err
	}
	return result, nil
}

func hashRegistrationCommand(command ports.RegistrationCommand, mutation ports.RegistrationMutation) [32]byte {
	digest := sha256.New()
	writeContestHashPart(digest, []byte("judgify.contest-registration-command.v1"))
	writeContestHashPart(digest, command.ContestID[:])
	writeContestHashPart(digest, command.UserID[:])
	writeContestHashPart(digest, []byte(mutation))
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], uint64(command.ExpectedContestVersion))
	writeContestHashPart(digest, integer[:])
	if command.ExpectedRegistrationVersion == nil {
		writeContestHashPart(digest, nil)
	} else {
		binary.BigEndian.PutUint64(integer[:], uint64(*command.ExpectedRegistrationVersion))
		writeContestHashPart(digest, integer[:])
	}
	writeContestHashPart(digest, command.ActorUserID[:])
	if command.ActorSessionID == nil {
		writeContestHashPart(digest, nil)
	} else {
		writeContestHashPart(digest, command.ActorSessionID[:])
	}
	writeContestHashPart(digest, []byte(command.Reason))
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

var _ ports.RegistrationCommandService = (*registrationCommandService)(nil)
