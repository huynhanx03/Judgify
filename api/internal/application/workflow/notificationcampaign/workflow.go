// Package notificationcampaign coordinates the cross-domain transaction that
// freezes an reviewed audience and executes its bounded delivery operation.
// Notification owns campaign content and recipients; Identity participates
// through a narrow eligibility port; operation owns only the control plane.
package notificationcampaign

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	entity "github.com/huynhanx03/judgify/internal/notification/domain"
	notificationservice "github.com/huynhanx03/judgify/internal/notification/usecase"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	identityports "github.com/huynhanx03/judgify/internal/ports"
	notificationports "github.com/huynhanx03/judgify/internal/ports"
)

type Config struct {
	MaximumRecipients       int
	BatchSize               int
	RecentAuthenticationTTL time.Duration
}

type Dependencies struct {
	Repository    notificationports.CampaignRepository
	Transactions  platformtx.UnitOfWork
	Audience      identityports.CampaignAudienceParticipant
	Capabilities  notificationports.CampaignCapabilityChecker
	Notifications notificationports.Service
	Operations    notificationports.OperationReader
	Starter       *operation.Starter
	Controller    *operation.Controller
	Confirmation  *notificationservice.CampaignConfirmationProtector
	Audit         audit.Appender
	Outbox        eventing.OutboxAppender
}

// DeliveryRequest is the immutable, canonical operation payload. It contains
// references to the frozen render tuple and audience snapshot, never the
// confirmation token or mutable content.
type DeliveryRequest struct {
	CampaignID               uuid.UUID         `json:"campaign_id"`
	CampaignRevisionID       uuid.UUID         `json:"campaign_revision_id"`
	RenderSourceID           uuid.UUID         `json:"render_source_id"`
	RenderSourceKind         string            `json:"render_source_kind"`
	RenderArtifactID         uuid.UUID         `json:"render_artifact_id"`
	NormalizedSourceChecksum [sha256.Size]byte `json:"normalized_source_checksum"`
	AudienceKind             string            `json:"audience_kind"`
	AudienceRoleID           *uuid.UUID        `json:"audience_role_id,omitempty"`
	AudienceFingerprint      [sha256.Size]byte `json:"audience_fingerprint"`
	RecipientCount           int64             `json:"recipient_count"`
}

type deliveryCheckpoint struct {
	AfterUserID *uuid.UUID `json:"after_user_id,omitempty"`
	Processed   int64      `json:"processed"`
	Failed      int64      `json:"failed"`
}

type scheduleIntent struct {
	CampaignID       uuid.UUID         `json:"campaign_id"`
	ActorID          uuid.UUID         `json:"actor_id"`
	SessionID        uuid.UUID         `json:"session_id"`
	ConfirmationHash [sha256.Size]byte `json:"confirmation_hash"`
	Reason           string            `json:"reason"`
}

type Workflow struct {
	repository    notificationports.CampaignRepository
	transactions  platformtx.UnitOfWork
	audience      identityports.CampaignAudienceParticipant
	capabilities  notificationports.CampaignCapabilityChecker
	notifications notificationports.Service
	operations    notificationports.OperationReader
	starter       *operation.Starter
	controller    *operation.Controller
	confirmation  *notificationservice.CampaignConfirmationProtector
	audit         audit.Appender
	outbox        eventing.OutboxAppender
	config        Config
	now           func() time.Time
	newID         func() (uuid.UUID, error)
}

func New(dependencies Dependencies, config Config) (*Workflow, error) {
	if dependencies.Repository == nil || dependencies.Transactions == nil ||
		dependencies.Audience == nil || dependencies.Capabilities == nil ||
		dependencies.Notifications == nil || dependencies.Operations == nil ||
		dependencies.Starter == nil || dependencies.Confirmation == nil ||
		dependencies.Controller == nil ||
		dependencies.Audit == nil || dependencies.Outbox == nil {
		return nil, notificationports.ErrInvalidRequest
	}
	if config.MaximumRecipients == 0 {
		config.MaximumRecipients = contract.DefaultMaximumCampaignRecipients
	}
	if config.BatchSize == 0 {
		config.BatchSize = contract.DefaultCampaignBatchSize
	}
	if config.RecentAuthenticationTTL == 0 {
		config.RecentAuthenticationTTL = 10 * time.Minute
	}
	if config.MaximumRecipients < 1 || config.MaximumRecipients > 1_000_000 ||
		config.BatchSize < 1 || config.BatchSize > operation.MaxBatchSize ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return nil, notificationports.ErrInvalidRequest
	}
	return &Workflow{
		repository:    dependencies.Repository,
		transactions:  dependencies.Transactions,
		audience:      dependencies.Audience,
		capabilities:  dependencies.Capabilities,
		notifications: dependencies.Notifications,
		operations:    dependencies.Operations,
		starter:       dependencies.Starter,
		controller:    dependencies.Controller,
		confirmation:  dependencies.Confirmation,
		audit:         dependencies.Audit,
		outbox:        dependencies.Outbox,
		config:        config,
		now:           func() time.Time { return time.Now().UTC() },
		newID:         uuid.NewV7,
	}, nil
}

var (
	_ notificationports.CampaignScheduler = (*Workflow)(nil)
	_ notificationports.CampaignCanceller = (*Workflow)(nil)
)

func (workflow *Workflow) Register(registry *operation.Registry) error {
	if workflow == nil || registry == nil {
		return operation.ErrInvalidExecutor
	}
	return operation.RegisterJSON(
		registry,
		contract.CampaignOperationKind,
		workflow.executeBatch,
	)
}

func (workflow *Workflow) Schedule(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	idempotencyKey uuid.UUID,
	request *contract.ScheduleCampaignRequest,
) (*contract.CampaignScheduleResponse, error) {
	if workflow == nil || ctx == nil || actorID == uuid.Nil ||
		sessionID == uuid.Nil || idempotencyKey == uuid.Nil ||
		request == nil || request.ID == uuid.Nil ||
		!contract.ValidCampaignReason(request.Reason) {
		return nil, notificationservice.MapCampaignError(notificationports.ErrInvalidRequest)
	}
	nonceHash, err := workflow.confirmation.Hash(request.ConfirmationToken)
	if err != nil {
		return nil, notificationservice.MapCampaignError(
			notificationports.ErrCampaignConfirmationInvalid,
		)
	}
	intentHash, err := hashScheduleIntent(scheduleIntent{
		CampaignID:       request.ID,
		ActorID:          actorID,
		SessionID:        sessionID,
		ConfirmationHash: nonceHash,
		Reason:           request.Reason,
	})
	if err != nil {
		return nil, notificationservice.MapCampaignError(err)
	}
	ctx = correlation.EnsureContext(ctx)
	now := workflow.now().UTC()
	var (
		view     entity.CampaignView
		started  operation.CreateResult
		replayed bool
	)
	err = workflow.transactions.Within(ctx, func(txCtx context.Context) error {
		confirmation, lockErr := workflow.repository.LockConfirmation(txCtx, nonceHash)
		if lockErr != nil {
			return lockErr
		}
		if confirmation.ActorID != actorID ||
			confirmation.SessionID != sessionID ||
			confirmation.CampaignID != request.ID {
			return notificationports.ErrCampaignConfirmationInvalid
		}
		if confirmation.ConsumedAt != nil {
			if confirmation.ConsumedByIdempotencyKey == nil ||
				*confirmation.ConsumedByIdempotencyKey != idempotencyKey ||
				!bytes.Equal(confirmation.ScheduleRequestHash, intentHash[:]) ||
				confirmation.OperationID == nil {
				return notificationports.ErrCampaignConfirmationInvalid
			}
			var replayErr error
			view, replayErr = workflow.repository.Get(txCtx, request.ID)
			if replayErr != nil {
				return replayErr
			}
			replayedOperation, replayErr := workflow.operations.Get(
				txCtx,
				*confirmation.OperationID,
			)
			if replayErr != nil {
				return replayErr
			}
			started = operation.CreateResult{
				Operation: replayedOperation,
				Replay:    true,
			}
			replayed = true
			return nil
		}
		if !confirmation.ExpiresAt.After(now) {
			return notificationports.ErrCampaignConfirmationExpired
		}

		view, lockErr = workflow.repository.Lock(txCtx, request.ID)
		if lockErr != nil {
			return lockErr
		}
		if err := validateReviewedCampaign(view, confirmation); err != nil {
			return err
		}
		if capabilityErr := workflow.capabilities.RequireAudienceCapability(
			txCtx,
			view.Revision.Audience.Kind,
		); capabilityErr != nil {
			return capabilityErr
		}
		if view.Revision.Audience.Kind != contract.AudienceExplicitUsers {
			if authErr := workflow.audience.RequireRecentReauthentication(
				txCtx,
				actorID,
				sessionID,
				now.Add(-workflow.config.RecentAuthenticationTTL),
			); authErr != nil {
				return mapIdentityError(authErr)
			}
		}
		identitySnapshot, snapshotErr := workflow.audience.SnapshotCampaignAudience(
			txCtx,
			toIdentityAudience(view.Revision.Audience),
			workflow.config.MaximumRecipients,
			now,
		)
		if snapshotErr != nil {
			return mapIdentityError(snapshotErr)
		}
		members := make([]entity.AudienceMember, len(identitySnapshot.Members))
		for index, member := range identitySnapshot.Members {
			members[index] = entity.AudienceMember{
				UserID:   member.UserID,
				Username: member.Username,
			}
		}
		snapshot, snapshotErr := entity.NewAudienceSnapshot(
			members,
			identitySnapshot.At,
		)
		if snapshotErr != nil {
			return snapshotErr
		}
		if snapshot.Fingerprint != confirmation.AudienceFingerprint ||
			int64(len(snapshot.Members)) != confirmation.DryRunRecipientCount {
			return notificationports.ErrCampaignAudienceChanged
		}

		operationID, idErr := workflow.newID()
		if idErr != nil || operationID == uuid.Nil {
			return fmt.Errorf("create campaign operation identity: %w", idErr)
		}
		view, lockErr = workflow.repository.FreezeSchedule(
			txCtx,
			notificationports.ScheduleFreeze{
				CampaignID:      request.ID,
				Revision:        view.Revision,
				ExpectedVersion: confirmation.ExpectedCampaignVersion,
				OperationID:     operationID,
				ScheduledFor:    cloneTime(confirmation.ScheduledFor),
				ActorID:         actorID,
				Reason:          request.Reason,
				At:              now,
			},
			snapshot.Members,
		)
		if lockErr != nil {
			return lockErr
		}
		if eventErr := workflow.appendCampaignTransition(
			txCtx,
			actorID,
			sessionID,
			"notification_campaign.schedule",
			request.Reason,
			view,
		); eventErr != nil {
			return eventErr
		}

		deliveryRequest := DeliveryRequest{
			CampaignID:               view.Campaign.ID,
			CampaignRevisionID:       view.Revision.ID,
			RenderSourceID:           view.Revision.BodyRenderSourceID,
			RenderSourceKind:         view.Revision.BodyRenderSourceKind,
			RenderArtifactID:         view.Revision.BodyRenderArtifactID,
			NormalizedSourceChecksum: view.Revision.NormalizedSourceChecksum,
			AudienceKind:             view.Revision.Audience.Kind,
			AudienceRoleID:           cloneUUID(view.Revision.Audience.RoleID),
			AudienceFingerprint:      snapshot.Fingerprint,
			RecipientCount:           int64(len(snapshot.Members)),
		}
		encodedRequest, requestHash, encodeErr := operation.EncodeRequest(
			deliveryRequest,
		)
		if encodeErr != nil {
			return encodeErr
		}
		checkpoint, encodeErr := json.Marshal(deliveryCheckpoint{})
		if encodeErr != nil {
			return encodeErr
		}
		started, lockErr = workflow.starter.StartInTransaction(
			txCtx,
			operation.StartRequest{
				Operation: operation.CreateCommand{
					ID:             operationID,
					Kind:           contract.CampaignOperationKind,
					IdempotencyKey: idempotencyKey,
					RequestHash:    requestHash,
					Request:        encodedRequest,
					Total:          deliveryRequest.RecipientCount,
					BatchSize:      workflow.config.BatchSize,
					Checkpoint:     checkpoint,
					ActorUserID:    actorID,
					ActorSessionID: cloneUUID(&sessionID),
					Reason:         request.Reason,
					CorrelationID:  correlation.FromContext(txCtx),
					NotBefore:      cloneTime(confirmation.ScheduledFor),
				},
				Audit: audit.Entry{
					Action:   "notification_campaign.delivery.start",
					Resource: "operation",
					Before:   map[string]any{},
					After: map[string]any{
						"kind":            contract.CampaignOperationKind,
						"campaign_id":     request.ID.String(),
						"recipient_count": deliveryRequest.RecipientCount,
					},
				},
			},
		)
		if lockErr != nil {
			return lockErr
		}
		if started.Operation.ID != operationID || started.Replay {
			return operation.ErrConflict
		}
		if consumeErr := workflow.repository.ConsumeConfirmation(
			txCtx,
			notificationports.ConfirmationConsumption{
				NonceHash:      nonceHash,
				IdempotencyKey: idempotencyKey,
				RequestHash:    intentHash,
				OperationID:    operationID,
				ConsumedAt:     now,
			},
		); consumeErr != nil {
			return consumeErr
		}
		return nil
	})
	if err != nil {
		return nil, notificationservice.MapCampaignError(err)
	}
	response := notificationservice.CampaignResponse(view, &started.Operation)
	logger.FromContext(ctx).Info(
		"notification campaign scheduled",
		zap.String("campaign_id", view.Campaign.ID.String()),
		zap.String("operation_id", started.Operation.ID.String()),
		zap.Int64("recipient_count", started.Operation.Total),
		zap.Bool("replayed", replayed),
	)
	return &contract.CampaignScheduleResponse{
		Campaign:  *response,
		Operation: *response.Operation,
		Replayed:  replayed,
	}, nil
}

func (workflow *Workflow) Cancel(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	idempotencyKey uuid.UUID,
	request *contract.CancelCampaignRequest,
) (*contract.CampaignCancelResponse, error) {
	if workflow == nil || ctx == nil || actorID == uuid.Nil ||
		sessionID == uuid.Nil || idempotencyKey == uuid.Nil ||
		request == nil || request.ID == uuid.Nil ||
		request.ExpectedCampaignVersion < 1 ||
		request.ExpectedOperationVersion < 1 ||
		!contract.ValidCampaignReason(request.Reason) {
		return nil, notificationservice.MapCampaignError(
			notificationports.ErrInvalidRequest,
		)
	}
	ctx = correlation.EnsureContext(ctx)
	now := workflow.now().UTC()
	var (
		view    entity.CampaignView
		control operation.ControlResult
		changed bool
	)
	err := workflow.transactions.Within(ctx, func(txCtx context.Context) error {
		before, loadErr := workflow.repository.Get(txCtx, request.ID)
		if loadErr != nil {
			return loadErr
		}
		active := before.Campaign.Status == contract.CampaignStatusScheduled ||
			before.Campaign.Status == contract.CampaignStatusSending
		exactReplay := before.Campaign.Status == contract.CampaignStatusCancelled &&
			before.Campaign.Version == request.ExpectedCampaignVersion+1 &&
			before.Campaign.CancelledBy != nil &&
			*before.Campaign.CancelledBy == actorID &&
			before.Campaign.CancelReason != nil &&
			*before.Campaign.CancelReason == request.Reason &&
			before.Campaign.CancelledAt != nil
		if !active && !exactReplay {
			return notificationports.ErrCampaignNotCancellable
		}
		if (active && before.Campaign.Version != request.ExpectedCampaignVersion) ||
			before.Campaign.OperationID == nil {
			return notificationports.ErrCampaignConflict
		}
		if capabilityErr := workflow.capabilities.RequireAudienceCapability(
			txCtx,
			before.Revision.Audience.Kind,
		); capabilityErr != nil {
			return capabilityErr
		}
		if active {
			if authErr := workflow.audience.RequireRecentReauthentication(
				txCtx,
				actorID,
				sessionID,
				now.Add(-workflow.config.RecentAuthenticationTTL),
			); authErr != nil {
				return mapIdentityError(authErr)
			}
		}
		operationID := *before.Campaign.OperationID
		control, loadErr = workflow.controller.ControlInTransaction(
			txCtx,
			operation.ControlCommand{
				OperationID:     operationID,
				Action:          operation.ControlCancel,
				ExpectedVersion: request.ExpectedOperationVersion,
				ActorUserID:     actorID,
				ActorSessionID:  sessionID,
				IdempotencyKey:  idempotencyKey,
				Reason:          request.Reason,
			},
		)
		if loadErr != nil {
			return loadErr
		}
		if exactReplay && !control.Replay {
			return notificationports.ErrCampaignConflict
		}
		view, changed, loadErr = workflow.repository.MarkCancelled(
			txCtx,
			notificationports.CampaignCancellation{
				CampaignID:      request.ID,
				OperationID:     operationID,
				ExpectedVersion: request.ExpectedCampaignVersion,
				ActorID:         actorID,
				Reason:          request.Reason,
				At:              now,
			},
		)
		if loadErr != nil {
			return loadErr
		}
		if !changed {
			return nil
		}
		return workflow.appendCancellationTransition(
			txCtx,
			actorID,
			sessionID,
			request.Reason,
			before,
			view,
		)
	})
	if err != nil {
		return nil, notificationservice.MapCampaignError(err)
	}
	currentOperation, err := workflow.operations.Get(
		ctx,
		control.Operation.ID,
	)
	if err != nil {
		return nil, notificationservice.MapCampaignError(err)
	}
	response := notificationservice.CampaignResponse(view, &currentOperation)
	if response.Operation == nil {
		return nil, notificationservice.MapCampaignError(
			notificationports.ErrCampaignConflict,
		)
	}
	logger.FromContext(ctx).Info(
		"notification campaign cancellation accepted",
		zap.String("campaign_id", view.Campaign.ID.String()),
		zap.String("operation_id", currentOperation.ID.String()),
		zap.String("operation_status", string(currentOperation.Status)),
		zap.String("requested_action", currentOperation.RequestedAction),
		zap.Bool("replayed", control.Replay && !changed),
	)
	return &contract.CampaignCancelResponse{
		Campaign:  *response,
		Operation: *response.Operation,
		Replayed:  control.Replay && !changed,
	}, nil
}

func (workflow *Workflow) executeBatch(
	ctx context.Context,
	request DeliveryRequest,
	input operation.BatchInput,
) (operation.BatchResult, error) {
	if workflow == nil || ctx == nil || input.OperationID == uuid.Nil ||
		input.Sequence < 1 || input.Limit < 1 || validateDeliveryRequest(request) != nil {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_campaign_delivery",
			notificationports.ErrInvalidRequest,
		)
	}
	checkpoint, err := decodeCheckpoint(input.Checkpoint)
	if err != nil || checkpoint.Processed > request.RecipientCount {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_campaign_checkpoint",
			err,
		)
	}
	now := workflow.now().UTC()
	sending, changed, err := workflow.repository.MarkSending(
		ctx,
		request.CampaignID,
		input.OperationID,
		now,
	)
	if err != nil {
		return operation.BatchResult{}, workflow.retryableDeliveryError(err)
	}
	if err := validateDeliveryView(sending, request, input.OperationID); err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"campaign_state_conflict",
			err,
		)
	}
	if changed {
		if err := workflow.appendAutomaticTransition(ctx, sending); err != nil {
			return operation.BatchResult{}, workflow.retryableDeliveryError(err)
		}
	}

	afterUserID := uuid.Nil
	if checkpoint.AfterUserID != nil {
		afterUserID = *checkpoint.AfterUserID
	}
	view, recipients, err := workflow.repository.LockDeliveryBatch(
		ctx,
		request.CampaignID,
		afterUserID,
		input.Limit,
		now,
	)
	if err != nil {
		return operation.BatchResult{}, workflow.retryableDeliveryError(err)
	}
	if err := validateDeliveryView(view, request, input.OperationID); err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"campaign_state_conflict",
			err,
		)
	}

	var (
		succeeded int64
		failed    int64
		failures  []operation.ItemFailure
	)
	for _, recipient := range recipients {
		eligibility, eligibilityErr := workflow.audience.CheckCampaignRecipient(
			ctx,
			recipient.UserID,
			request.AudienceKind,
			request.AudienceRoleID,
		)
		if eligibilityErr != nil {
			return operation.BatchResult{}, workflow.retryableDeliveryError(
				eligibilityErr,
			)
		}
		result := notificationports.RecipientResult{
			UserID:      recipient.UserID,
			AttemptedAt: now,
		}
		if !eligibility.Eligible {
			result.Status = contract.RecipientStatusSkipped
			result.SkipCode = eligibility.SkipCode
			if !contract.ValidCampaignCode(result.SkipCode) {
				result.SkipCode = "ineligible_recipient"
			}
			succeeded++
		} else {
			revisionID := request.CampaignRevisionID
			created, _, consumeErr := workflow.notifications.CreateFromEvent(
				ctx,
				notificationports.CreateCommand{
					UserID:             recipient.UserID,
					SourceEventID:      recipient.SourceEventID,
					Type:               "notification.campaign.delivered.v1",
					Category:           contract.CategoryCampaign,
					CampaignRevisionID: &revisionID,
					ActionPath:         cloneString(view.Revision.ActionPath),
					OccurredAt:         now,
				},
			)
			if consumeErr != nil {
				return operation.BatchResult{}, workflow.retryableDeliveryError(
					consumeErr,
				)
			}
			if created == nil {
				result.Status = contract.RecipientStatusSkipped
				result.SkipCode = "preference_disabled"
			} else {
				result.Status = contract.RecipientStatusCreated
				result.NotificationID = cloneUUID(&created.ID)
			}
			succeeded++
		}
		if applyErr := workflow.repository.ApplyRecipientResult(
			ctx,
			request.CampaignID,
			result,
		); applyErr != nil {
			return operation.BatchResult{}, workflow.retryableDeliveryError(
				applyErr,
			)
		}
	}
	processed := int64(len(recipients))
	checkpoint.Processed += processed
	checkpoint.Failed += failed
	if len(recipients) > 0 {
		checkpoint.AfterUserID = cloneUUID(
			&recipients[len(recipients)-1].UserID,
		)
	}
	if checkpoint.Processed > request.RecipientCount {
		return operation.BatchResult{}, operation.Terminal(
			"campaign_progress_overflow",
			notificationports.ErrCampaignConflict,
		)
	}
	done := checkpoint.Processed == request.RecipientCount
	if !done && processed == 0 {
		return operation.BatchResult{}, operation.Terminal(
			"campaign_recipient_gap",
			notificationports.ErrCampaignConflict,
		)
	}
	nextCheckpoint, err := json.Marshal(checkpoint)
	if err != nil {
		return operation.BatchResult{}, operation.Terminal(
			"invalid_campaign_checkpoint",
			err,
		)
	}
	failureCode := ""
	if done {
		status := contract.CampaignStatusCompleted
		if checkpoint.Failed > 0 {
			status = contract.CampaignStatusFailed
			failureCode = "campaign_recipient_failure"
		}
		terminal, terminalErr := workflow.repository.MarkTerminal(
			ctx,
			request.CampaignID,
			input.OperationID,
			status,
			failureCode,
			now,
		)
		if terminalErr != nil {
			return operation.BatchResult{}, workflow.retryableDeliveryError(
				terminalErr,
			)
		}
		if eventErr := workflow.appendAutomaticTransition(ctx, terminal); eventErr != nil {
			return operation.BatchResult{}, workflow.retryableDeliveryError(
				eventErr,
			)
		}
	}
	logger.FromContext(ctx).Debug(
		"notification campaign delivery batch applied",
		zap.String("campaign_id", request.CampaignID.String()),
		zap.String("operation_id", input.OperationID.String()),
		zap.Int64("batch_sequence", input.Sequence),
		zap.Int64("processed_count", processed),
		zap.Int64("campaign_processed_count", checkpoint.Processed),
		zap.Bool("done", done),
	)
	return operation.BatchResult{
		Processed:      processed,
		Succeeded:      succeeded,
		Failed:         failed,
		NextCheckpoint: nextCheckpoint,
		Failures:       failures,
		Done:           done,
		FailureCode:    failureCode,
	}, nil
}

func (workflow *Workflow) retryableDeliveryError(err error) error {
	return operation.Retryable("campaign_delivery_unavailable", 0, err)
}

func validateReviewedCampaign(
	view entity.CampaignView,
	confirmation entity.CampaignConfirmation,
) error {
	if view.Campaign.Status != contract.CampaignStatusDraft {
		return notificationports.ErrCampaignNotDraft
	}
	if view.Campaign.Version != confirmation.ExpectedCampaignVersion ||
		view.Revision.ID != confirmation.CampaignRevisionID ||
		view.Revision.BodyRenderSourceID != confirmation.RenderSourceID ||
		view.Revision.BodyRenderSourceKind != confirmation.RenderSourceKind ||
		view.Revision.BodyRenderArtifactID != confirmation.RenderArtifactID ||
		view.Revision.NormalizedSourceChecksum !=
			confirmation.NormalizedSourceChecksum {
		return notificationports.ErrCampaignConflict
	}
	return nil
}

func validateDeliveryRequest(request DeliveryRequest) error {
	if request.CampaignID == uuid.Nil ||
		request.CampaignRevisionID == uuid.Nil ||
		request.RenderSourceID == uuid.Nil ||
		request.RenderSourceKind != "notification_campaign_body" ||
		request.RenderArtifactID == uuid.Nil ||
		request.NormalizedSourceChecksum == ([sha256.Size]byte{}) ||
		request.AudienceFingerprint == ([sha256.Size]byte{}) ||
		request.RecipientCount < 0 {
		return notificationports.ErrInvalidRequest
	}
	switch request.AudienceKind {
	case contract.AudienceRole:
		if request.AudienceRoleID == nil || *request.AudienceRoleID == uuid.Nil {
			return notificationports.ErrInvalidRequest
		}
	case contract.AudienceExplicitUsers, contract.AudienceAllActive:
		if request.AudienceRoleID != nil {
			return notificationports.ErrInvalidRequest
		}
	default:
		return notificationports.ErrInvalidRequest
	}
	return nil
}

func validateDeliveryView(
	view entity.CampaignView,
	request DeliveryRequest,
	operationID uuid.UUID,
) error {
	if view.Campaign.OperationID == nil ||
		*view.Campaign.OperationID != operationID ||
		view.Revision.ID != request.CampaignRevisionID ||
		view.Revision.BodyRenderSourceID != request.RenderSourceID ||
		view.Revision.BodyRenderSourceKind != request.RenderSourceKind ||
		view.Revision.BodyRenderArtifactID != request.RenderArtifactID ||
		view.Revision.NormalizedSourceChecksum !=
			request.NormalizedSourceChecksum ||
		view.Revision.Audience.Kind != request.AudienceKind ||
		!sameUUID(view.Revision.Audience.RoleID, request.AudienceRoleID) ||
		view.Counts.Total != request.RecipientCount {
		return notificationports.ErrCampaignConflict
	}
	return nil
}

func decodeCheckpoint(raw json.RawMessage) (deliveryCheckpoint, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var checkpoint deliveryCheckpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return deliveryCheckpoint{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return deliveryCheckpoint{}, operation.ErrInvalidCheckpoint
	}
	if checkpoint.Processed < 0 || checkpoint.Failed < 0 ||
		checkpoint.Failed > checkpoint.Processed ||
		(checkpoint.AfterUserID != nil && *checkpoint.AfterUserID == uuid.Nil) {
		return deliveryCheckpoint{}, operation.ErrInvalidCheckpoint
	}
	return checkpoint, nil
}

func hashScheduleIntent(intent scheduleIntent) ([sha256.Size]byte, error) {
	payload, err := json.Marshal(intent)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(payload), nil
}

func (workflow *Workflow) appendCampaignTransition(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	action string,
	reason string,
	view entity.CampaignView,
) error {
	eventID, err := workflow.newID()
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("create campaign transition identity: %w", err)
	}
	actor := actorID
	session := sessionID
	if _, err := workflow.audit.Append(ctx, audit.Entry{
		ID:             eventID,
		ActorUserID:    &actor,
		ActorSessionID: &session,
		Action:         action,
		Resource:       contract.CampaignAggregateType,
		ResourceID:     view.Campaign.ID.String(),
		Reason:         reason,
		Before: map[string]any{
			"status":  contract.CampaignStatusDraft,
			"version": view.Campaign.Version - 1,
		},
		After: map[string]any{
			"status":          view.Campaign.Status,
			"version":         view.Campaign.Version,
			"revision_id":     view.Revision.ID.String(),
			"operation_id":    view.Campaign.OperationID.String(),
			"recipient_count": view.Counts.Total,
			"scheduled_for":   view.Campaign.ScheduledFor,
		},
	}); err != nil {
		return err
	}
	return workflow.appendStatusEvent(ctx, eventID, view)
}

func (workflow *Workflow) appendCancellationTransition(
	ctx context.Context,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	reason string,
	before entity.CampaignView,
	after entity.CampaignView,
) error {
	eventID, err := workflow.newID()
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("create campaign cancellation identity: %w", err)
	}
	actor := actorID
	session := sessionID
	if _, err := workflow.audit.Append(ctx, audit.Entry{
		ID:             eventID,
		ActorUserID:    &actor,
		ActorSessionID: &session,
		Action:         "notification_campaign.cancel",
		Resource:       contract.CampaignAggregateType,
		ResourceID:     after.Campaign.ID.String(),
		Reason:         reason,
		Before: map[string]any{
			"status":       before.Campaign.Status,
			"version":      before.Campaign.Version,
			"operation_id": before.Campaign.OperationID.String(),
		},
		After: map[string]any{
			"status":       after.Campaign.Status,
			"version":      after.Campaign.Version,
			"operation_id": after.Campaign.OperationID.String(),
			"cancelled_by": actorID.String(),
			"cancelled_at": after.Campaign.CancelledAt,
		},
	}); err != nil {
		return err
	}
	return workflow.appendStatusEvent(ctx, eventID, after)
}

func (workflow *Workflow) appendAutomaticTransition(
	ctx context.Context,
	view entity.CampaignView,
) error {
	eventID, err := workflow.newID()
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("create campaign status identity: %w", err)
	}
	return workflow.appendStatusEvent(ctx, eventID, view)
}

func (workflow *Workflow) appendStatusEvent(
	ctx context.Context,
	eventID uuid.UUID,
	view entity.CampaignView,
) error {
	event := contract.CampaignLifecycleEvent{
		CampaignID:  view.Campaign.ID,
		Status:      view.Campaign.Status,
		Version:     view.Campaign.Version,
		OperationID: cloneUUID(view.Campaign.OperationID),
	}
	if err := event.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return workflow.outbox.Append(ctx, eventing.Event{
		ID:               eventID,
		Type:             contract.EventCampaignStatus,
		AggregateType:    contract.CampaignAggregateType,
		AggregateID:      view.Campaign.ID,
		AggregateVersion: view.Campaign.Version,
		Payload:          payload,
		OccurredAt:       workflow.now().UTC(),
	})
}

func mapIdentityError(err error) error {
	switch {
	case errors.Is(err, identityports.IdentityErrCampaignAudienceTooLarge):
		return notificationports.ErrCampaignAudienceTooLarge
	case errors.Is(err, identityports.ErrCampaignRecentAuthenticationRequired):
		return notificationports.ErrCampaignReauthenticationRequired
	case errors.Is(err, identityports.ErrCampaignRoleNotFound),
		errors.Is(err, identityports.ErrInvalidCampaignAudience):
		return notificationports.ErrInvalidRequest
	default:
		return err
	}
}

func toIdentityAudience(
	value entity.AudienceSelector,
) identityports.CampaignAudienceSelector {
	return identityports.CampaignAudienceSelector{
		Kind:    value.Kind,
		RoleID:  cloneUUID(value.RoleID),
		UserIDs: append([]uuid.UUID(nil), value.UserIDs...),
	}
}

func sameUUID(left, right *uuid.UUID) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
