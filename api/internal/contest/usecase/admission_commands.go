package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	contestcontract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	contestAggregateType      = contestcontract.AggregateContest
	contestLifecycleChangedV1 = contestcontract.EventLifecycleChangedV1
)

type LifecycleCommandDependencies struct {
	Transactions platformtx.UnitOfWork
	Repository   ports.LifecycleRepository
	Audit        audit.Appender
	Outbox       eventing.OutboxAppender
}

type lifecycleCommandService struct{ dependencies LifecycleCommandDependencies }

func NewLifecycleCommandService(dependencies LifecycleCommandDependencies) (ports.LifecycleService, error) {
	if nilContestDependency(dependencies.Transactions) || nilContestDependency(dependencies.Repository) ||
		nilContestDependency(dependencies.Audit) || nilContestDependency(dependencies.Outbox) {
		return nil, ports.ErrInvalidContestCommand
	}
	return &lifecycleCommandService{dependencies: dependencies}, nil
}

func (service *lifecycleCommandService) Transition(
	ctx context.Context,
	command ports.TransitionCommand,
) (ports.TransitionResult, error) {
	if service == nil || ctx == nil || command.ContestID == uuid.Nil ||
		command.IdempotencyKey == uuid.Nil || (!command.System && command.ActorUserID == uuid.Nil) ||
		(command.System && (command.ActorUserID != uuid.Nil || command.ActorSessionID != nil)) ||
		command.ExpectedVersion < 1 || !command.Target.Valid() ||
		entity.ValidateContestReason(command.Reason, true) != nil ||
		(command.ActorSessionID != nil && *command.ActorSessionID == uuid.Nil) {
		return ports.TransitionResult{}, ports.ErrInvalidContestCommand
	}
	if err := ctx.Err(); err != nil {
		return ports.TransitionResult{}, err
	}
	command.Reason = strings.Clone(command.Reason)
	command.CommandHash = hashLifecycleCommand(command)
	ctx = correlation.EnsureContext(ctx)
	ctx = logger.WithFields(
		ctx,
		logger.String("contest_id", command.ContestID.String()),
		logger.String("target", string(command.Target)),
		logger.String("expected_version", strconv.FormatInt(command.ExpectedVersion, 10)),
		logger.String("system", strconv.FormatBool(command.System)),
		logger.String("command_id", command.IdempotencyKey.String()),
	)
	logger.FromContext(ctx).Debug("contest lifecycle command started")

	var result ports.TransitionResult
	err := service.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
		transition, err := service.dependencies.Repository.Transition(txCtx, command)
		if err != nil {
			return err
		}
		if transition.ContestID != command.ContestID || transition.CommandID != command.IdempotencyKey ||
			transition.To != command.Target || transition.Version < 2 || transition.OccurredAt.IsZero() {
			return ports.ErrContestCommandDenied
		}
		result = transition
		if transition.ExactReplay {
			return nil
		}
		eventID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("create contest lifecycle event identity: %w", err)
		}
		payload, err := json.Marshal(contestcontract.LifecycleChangedV1{
			EventID:    eventID,
			ContestID:  transition.ContestID,
			From:       string(transition.From),
			To:         string(transition.To),
			Version:    transition.Version,
			OccurredAt: transition.OccurredAt.UTC(),
		})
		if err != nil {
			return fmt.Errorf("encode contest lifecycle event: %w", err)
		}
		var actorUserID *uuid.UUID
		if !command.System {
			actor := command.ActorUserID
			actorUserID = &actor
		}
		if _, err := service.dependencies.Audit.Append(txCtx, audit.Entry{
			ID: eventID, ActorUserID: actorUserID, ActorSessionID: command.ActorSessionID,
			Action: contestLifecycleChangedV1, Resource: contestAggregateType,
			ResourceID: transition.ContestID.String(), Reason: command.Reason,
			Before:        map[string]any{"state": transition.From, "version": transition.Version - 1},
			After:         map[string]any{"state": transition.To, "version": transition.Version},
			CorrelationID: correlation.FromContext(txCtx), OccurredAt: transition.OccurredAt,
		}); err != nil {
			return fmt.Errorf("append contest lifecycle audit: %w", err)
		}
		if err := service.dependencies.Outbox.Append(txCtx, eventing.Event{
			ID: eventID, Type: contestLifecycleChangedV1, AggregateType: contestAggregateType,
			AggregateID: transition.ContestID, AggregateVersion: transition.Version,
			CorrelationID: correlation.FromContext(txCtx), Payload: payload,
			OccurredAt: transition.OccurredAt,
		}); err != nil {
			return fmt.Errorf("append contest lifecycle event: %w", err)
		}
		return nil
	})
	if err != nil {
		logger.FromContext(ctx).Warn(
			"contest lifecycle command rejected",
			zap.Error(err),
		)
		return ports.TransitionResult{}, err
	}
	logger.FromContext(ctx).Info(
		"contest lifecycle command committed",
		zap.String("from", string(result.From)),
		zap.String("to", string(result.To)),
		zap.Int64("version", result.Version),
		zap.Bool("idempotent_replay", result.ExactReplay),
	)
	return result, nil
}

func hashLifecycleCommand(command ports.TransitionCommand) [32]byte {
	digest := sha256.New()
	writeContestHashPart(digest, []byte("judgify.contest-lifecycle-command.v1"))
	writeContestHashPart(digest, command.ContestID[:])
	writeContestHashPart(digest, []byte(command.Target))
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], uint64(command.ExpectedVersion))
	writeContestHashPart(digest, integer[:])
	if command.System {
		writeContestHashPart(digest, []byte{1})
	} else {
		writeContestHashPart(digest, []byte{0})
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

func writeContestHashPart(digest hash.Hash, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = digest.Write(size[:])
	_, _ = digest.Write(value)
}

func nilContestDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ ports.LifecycleService = (*lifecycleCommandService)(nil)
