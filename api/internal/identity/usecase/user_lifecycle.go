package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	identitycontract "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

const maximumUserLifecyclePayloadBytes = 16 << 10

type userLifecycleDependencies struct {
	audit      audit.Appender
	outbox     eventing.OutboxAppender
	invitation ports.AccountInvitationRepository
	now        func() time.Time
}

func WithUserLifecycle(
	auditAppender audit.Appender,
	outboxAppender eventing.OutboxAppender,
	invitationRepository ports.AccountInvitationRepository,
) UserServiceOption {
	return func(service *userService) {
		service.lifecycle = &userLifecycleDependencies{
			audit:      auditAppender,
			outbox:     outboxAppender,
			invitation: invitationRepository,
			now:        func() time.Time { return time.Now().UTC() },
		}
	}
}

func (s *userService) PreviewLifecycle(
	ctx context.Context,
	id uuid.UUID,
	action string,
	req *dto.UserLifecycleCommandRequest,
) (*dto.UserLifecyclePreview, error) {
	if err := s.validateUserLifecycle(ctx); err != nil {
		return nil, err
	}
	if err := validateUserLifecycleRequest(id, action, req, false); err != nil {
		return nil, err
	}
	actorID, _, err := userLifecycleActor(ctx)
	if err != nil {
		return nil, err
	}
	if actorID == id {
		return nil, userLifecycleSelfConflict()
	}
	current, err := s.userRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	currentVersion, ok := userLifecycleVersion(current)
	if !ok {
		return nil, invalidUserLifecycle("invalid account state")
	}
	if currentVersion != req.ExpectedVersion {
		return nil, userLifecycleConflict(current)
	}
	before, after, _, err := userLifecycleProjection(
		current,
		action,
		s.lifecycle.now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	return &dto.UserLifecyclePreview{
		Action:           action,
		UserID:           id,
		ExpectedVersion:  req.ExpectedVersion,
		ResultingVersion: req.ExpectedVersion + 1,
		Before:           before,
		After:            after,
	}, nil
}

func (s *userService) ApplyLifecycle(
	ctx context.Context,
	id uuid.UUID,
	action string,
	req *dto.UserLifecycleCommandRequest,
	requestHash [sha256.Size]byte,
) (*dto.UserLifecycleReceipt, error) {
	if err := s.validateUserLifecycle(ctx); err != nil {
		return nil, err
	}
	if err := validateUserLifecycleRequest(id, action, req, true); err != nil ||
		requestHash == ([sha256.Size]byte{}) {
		return nil, invalidUserLifecycle("invalid user lifecycle command")
	}
	actorID, sessionID, err := userLifecycleActor(ctx)
	if err != nil {
		return nil, err
	}
	if actorID == id {
		return nil, userLifecycleSelfConflict()
	}
	eventType, ok := identitycontract.UserLifecycleEventType(action)
	if !ok {
		return nil, invalidUserLifecycle("invalid user lifecycle action")
	}
	receipt, found, err := s.loadUserLifecycleReceipt(
		ctx,
		req.CommandID,
		eventType,
		action,
		id,
		req.ExpectedVersion,
		requestHash,
		actorID,
		sessionID,
		true,
	)
	if err != nil || found {
		return receipt, err
	}

	var result *dto.UserLifecycleReceipt
	err = s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		current, err := s.userRepo.Get(txCtx, id)
		if err != nil {
			return err
		}
		currentVersion, valid := userLifecycleVersion(current)
		if !valid {
			return ports.ErrInvalidUserLifecycle
		}
		if currentVersion != req.ExpectedVersion {
			return ports.ErrUserLifecycleConflict
		}
		now := s.lifecycle.now().UTC()
		before, expectedAfter, targetStatus, err := userLifecycleProjection(
			current,
			action,
			now,
		)
		if err != nil {
			return err
		}
		if action == "delete" {
			if current.SecurityStatus == entity.UserSecurityStatusInvited {
				if err := s.lifecycle.invitation.RevokeInvitation(txCtx, id, now); err != nil {
					return err
				}
			}
			if err := s.authzService.SetUserRoles(txCtx, id, nil); err != nil {
				return err
			}
		}
		if current.SecurityStatus == entity.UserSecurityStatusActive &&
			(action == "suspend" || action == "delete") {
			// Delete removes roles first because that mutation owns the
			// process-local authorization writer gate. The guard then keeps the
			// same DB revision fence locked through the status transition.
			if err := s.authzService.EnsureCanDeactivateUser(txCtx, id); err != nil {
				return err
			}
		}
		if err := s.tokenLifecycle.RevokeAll(
			txCtx,
			id,
			userLifecycleRevocationReason(action),
		); err != nil {
			return err
		}
		updated, err := s.userRepo.TransitionLifecycleCAS(
			txCtx,
			id,
			req.ExpectedVersion,
			current.SecurityStatus,
			targetStatus,
			req.Reason,
			now,
			actorID,
		)
		if err != nil {
			return err
		}
		after := lifecycleProjection(updated)
		if !sameUserLifecycleProjection(expectedAfter, after) {
			return ports.ErrInvalidUserLifecycle
		}
		auditID, err := s.lifecycle.audit.Append(txCtx, audit.Entry{
			ActorUserID:    &actorID,
			ActorSessionID: &sessionID,
			Action:         "identity.user.lifecycle." + action,
			Resource:       "identity.user",
			ResourceID:     id.String(),
			Reason:         req.Reason,
			Before:         userLifecycleAudit(before),
			After:          userLifecycleAudit(after),
			OccurredAt:     now,
		})
		if err != nil {
			return err
		}
		payload := identitycontract.UserLifecycleChangedV1{
			Version:          identitycontract.UserLifecyclePayloadVersionV1,
			Action:           action,
			CommandID:        req.CommandID.String(),
			RequestHash:      hex.EncodeToString(requestHash[:]),
			AuditID:          auditID.String(),
			ActorID:          actorID.String(),
			ActorSessionID:   sessionID.String(),
			ExpectedVersion:  req.ExpectedVersion,
			ResultingVersion: after.Version,
			Before:           lifecycleContractProjection(before),
			After:            lifecycleContractProjection(after),
		}
		payloadBytes, err := commonjson.Marshal(payload)
		if err != nil || len(payloadBytes) == 0 ||
			len(payloadBytes) > maximumUserLifecyclePayloadBytes {
			return ports.ErrInvalidUserLifecycle
		}
		if err := s.lifecycle.outbox.Append(txCtx, eventing.Event{
			ID:               req.CommandID,
			Type:             eventType,
			AggregateType:    identitycontract.AggregateUser,
			AggregateID:      id,
			AggregateVersion: after.Version,
			Payload:          payloadBytes,
			OccurredAt:       now,
		}); err != nil {
			return err
		}
		result = &dto.UserLifecycleReceipt{
			CommandID:   req.CommandID,
			EventID:     req.CommandID,
			Action:      action,
			User:        after,
			CommittedAt: now,
		}
		return nil
	})
	if err != nil {
		receipt, found, replayErr := s.loadUserLifecycleReceipt(
			ctx,
			req.CommandID,
			eventType,
			action,
			id,
			req.ExpectedVersion,
			requestHash,
			actorID,
			sessionID,
			true,
		)
		if replayErr != nil || found {
			return receipt, replayErr
		}
		if errors.Is(err, ports.ErrUserLifecycleConflict) {
			return nil, s.currentUserLifecycleConflict(ctx, id)
		}
		if errors.Is(err, ports.ErrInvalidUserLifecycle) {
			return nil, invalidUserLifecycle("invalid account lifecycle state")
		}
		return nil, err
	}
	if result == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"user lifecycle unavailable",
			ports.ErrInvalidUserLifecycle,
		)
	}
	logger.FromContext(ctx).Info(
		"user lifecycle command committed",
		zap.String("user_id", id.String()),
		zap.String("action", action),
		zap.Int64("version", result.User.Version),
		zap.String("command_id", req.CommandID.String()),
	)
	return result, nil
}

func (s *userService) validateUserLifecycle(ctx context.Context) error {
	if s == nil || s.userRepo == nil || s.authzService == nil ||
		s.tokenLifecycle == nil || s.txMgr == nil || s.lifecycle == nil ||
		s.lifecycle.audit == nil || s.lifecycle.outbox == nil ||
		s.lifecycle.invitation == nil ||
		s.lifecycle.now == nil || ctx == nil || ctx.Err() != nil {
		return apperr.New(
			apperr.CodeInternalServer,
			"user lifecycle unavailable",
			ports.ErrInvalidUserLifecycle,
		)
	}
	return nil
}

func validateUserLifecycleRequest(
	id uuid.UUID,
	action string,
	req *dto.UserLifecycleCommandRequest,
	requireConfirmation bool,
) error {
	if id == uuid.Nil || req == nil || req.ID != id || req.CommandID == uuid.Nil ||
		req.ExpectedVersion < 1 || req.ExpectedVersion == math.MaxInt64 ||
		!validUserLifecycleAction(action) ||
		!validUserLifecycleReason(req.Reason) ||
		(requireConfirmation && req.ConfirmationToken == "") ||
		(!requireConfirmation && req.ConfirmationToken != "") {
		return invalidUserLifecycle("invalid user lifecycle command")
	}
	return nil
}

func validUserLifecycleAction(action string) bool {
	return action == "suspend" || action == "reactivate" || action == "delete"
}

func validUserLifecycleReason(reason string) bool {
	if !utf8.ValidString(reason) || strings.TrimSpace(reason) != reason ||
		len(reason) < 3 || len(reason) > 1024 {
		return false
	}
	for _, current := range reason {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}

func userLifecycleActor(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK || !sessionOK || actorID == uuid.Nil || sessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			nil,
		)
	}
	return actorID, sessionID, nil
}

func userLifecycleVersion(user *entity.User) (int64, bool) {
	if user == nil || user.ID == uuid.Nil || user.SecurityVersion == 0 ||
		user.SecurityVersion > math.MaxInt64 {
		return 0, false
	}
	return int64(user.SecurityVersion), true
}

func userLifecycleProjection(
	current *entity.User,
	action string,
	at time.Time,
) (
	dto.UserLifecycleProjection,
	dto.UserLifecycleProjection,
	entity.UserSecurityStatus,
	error,
) {
	version, ok := userLifecycleVersion(current)
	if !ok || current.Username == "" || strings.TrimSpace(current.Username) != current.Username ||
		at.IsZero() {
		return dto.UserLifecycleProjection{}, dto.UserLifecycleProjection{}, "",
			invalidUserLifecycle("invalid account state")
	}
	before := lifecycleProjection(current)
	after := before
	after.Version = version + 1
	switch action {
	case "suspend":
		if current.SecurityStatus != entity.UserSecurityStatusActive ||
			current.SuspendedAt != nil {
			return dto.UserLifecycleProjection{}, dto.UserLifecycleProjection{}, "",
				invalidUserLifecycle("only an active account can be suspended")
		}
		suspendedAt := at.UTC()
		after.Status = string(entity.UserSecurityStatusSuspended)
		after.SuspendedAt = &suspendedAt
		return before, after, entity.UserSecurityStatusSuspended, nil
	case "reactivate":
		if current.SecurityStatus != entity.UserSecurityStatusSuspended ||
			current.SuspendedAt == nil || current.SuspendedAt.IsZero() {
			return dto.UserLifecycleProjection{}, dto.UserLifecycleProjection{}, "",
				invalidUserLifecycle("only a suspended account can be reactivated")
		}
		after.Status = string(entity.UserSecurityStatusActive)
		after.SuspendedAt = nil
		return before, after, entity.UserSecurityStatusActive, nil
	case "delete":
		if (current.SecurityStatus != entity.UserSecurityStatusInvited &&
			current.SecurityStatus != entity.UserSecurityStatusActive &&
			current.SecurityStatus != entity.UserSecurityStatusSuspended) ||
			(current.SecurityStatus == entity.UserSecurityStatusSuspended &&
				(current.SuspendedAt == nil || current.SuspendedAt.IsZero())) {
			return dto.UserLifecycleProjection{}, dto.UserLifecycleProjection{}, "",
				invalidUserLifecycle("only an invited, active or suspended account can be deleted")
		}
		after.Status = string(entity.UserSecurityStatusDeleted)
		after.SuspendedAt = nil
		return before, after, entity.UserSecurityStatusDeleted, nil
	default:
		return dto.UserLifecycleProjection{}, dto.UserLifecycleProjection{}, "",
			invalidUserLifecycle("invalid user lifecycle action")
	}
}

func lifecycleProjection(user *entity.User) dto.UserLifecycleProjection {
	if user == nil {
		return dto.UserLifecycleProjection{}
	}
	var suspendedAt *time.Time
	if user.SuspendedAt != nil {
		copyValue := user.SuspendedAt.UTC()
		suspendedAt = &copyValue
	}
	version, _ := userLifecycleVersion(user)
	return dto.UserLifecycleProjection{
		ID:          user.ID,
		Username:    user.Username,
		Status:      string(user.SecurityStatus),
		Version:     version,
		SuspendedAt: suspendedAt,
	}
}

func sameUserLifecycleProjection(
	left dto.UserLifecycleProjection,
	right dto.UserLifecycleProjection,
) bool {
	if left.ID != right.ID || left.Username != right.Username ||
		left.Status != right.Status || left.Version != right.Version ||
		(left.SuspendedAt == nil) != (right.SuspendedAt == nil) {
		return false
	}
	return left.SuspendedAt == nil || left.SuspendedAt.Equal(*right.SuspendedAt)
}

func lifecycleContractProjection(
	projection dto.UserLifecycleProjection,
) identitycontract.UserLifecycleStateV1 {
	return identitycontract.UserLifecycleStateV1{
		UserID:      projection.ID.String(),
		Username:    projection.Username,
		Status:      projection.Status,
		Version:     projection.Version,
		SuspendedAt: projection.SuspendedAt,
	}
}

func lifecycleDTOProjection(
	projection identitycontract.UserLifecycleStateV1,
) (dto.UserLifecycleProjection, error) {
	userID, err := uuid.Parse(projection.UserID)
	if err != nil || userID == uuid.Nil {
		return dto.UserLifecycleProjection{}, ports.ErrInvalidUserLifecycle
	}
	result := dto.UserLifecycleProjection{
		ID:          userID,
		Username:    projection.Username,
		Status:      projection.Status,
		Version:     projection.Version,
		SuspendedAt: projection.SuspendedAt,
	}
	if result.SuspendedAt != nil {
		copyValue := result.SuspendedAt.UTC()
		result.SuspendedAt = &copyValue
	}
	return result, nil
}

func userLifecycleAudit(projection dto.UserLifecycleProjection) map[string]any {
	result := map[string]any{
		"user_id":  projection.ID.String(),
		"username": projection.Username,
		"status":   projection.Status,
		"version":  projection.Version,
	}
	if projection.SuspendedAt != nil {
		result["suspended_at"] = projection.SuspendedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func userLifecycleRevocationReason(action string) string {
	switch action {
	case "suspend":
		return "account_suspended"
	case "reactivate":
		return "account_reactivated"
	case "delete":
		return "account_deleted"
	default:
		return "account_state_changed"
	}
}

func (s *userService) loadUserLifecycleReceipt(
	ctx context.Context,
	commandID uuid.UUID,
	eventType string,
	action string,
	userID uuid.UUID,
	expectedVersion int64,
	requestHash [sha256.Size]byte,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	replayed bool,
) (*dto.UserLifecycleReceipt, bool, error) {
	receipt, found, err := s.userRepo.LoadCommandReceipt(ctx, commandID)
	if err != nil || !found {
		return nil, found, err
	}
	var payload identitycontract.UserLifecycleChangedV1
	decodeErr := commonjson.UnmarshalStrictLimit(
		receipt.Payload,
		&payload,
		maximumUserLifecyclePayloadBytes,
	)
	before, beforeErr := lifecycleDTOProjection(payload.Before)
	after, afterErr := lifecycleDTOProjection(payload.After)
	auditID, auditErr := uuid.Parse(payload.AuditID)
	if receipt.EventID != commandID || receipt.EventType != eventType ||
		receipt.AggregateType != identitycontract.AggregateUser ||
		receipt.AggregateID != userID ||
		receipt.AggregateVersion != expectedVersion+1 ||
		receipt.OccurredAt.IsZero() || decodeErr != nil ||
		payload.Version != identitycontract.UserLifecyclePayloadVersionV1 ||
		payload.Action != action || payload.CommandID != commandID.String() ||
		payload.RequestHash != hex.EncodeToString(requestHash[:]) ||
		payload.AuditID != auditID.String() || auditErr != nil || auditID == uuid.Nil ||
		payload.ActorID != actorID.String() ||
		payload.ActorSessionID != sessionID.String() ||
		payload.ExpectedVersion != expectedVersion ||
		payload.ResultingVersion != expectedVersion+1 ||
		beforeErr != nil || afterErr != nil ||
		before.ID != userID || after.ID != userID ||
		before.Username == "" || before.Username != after.Username ||
		before.Version != expectedVersion ||
		after.Version != expectedVersion+1 ||
		!validUserLifecycleTransitionProjection(action, before, after) {
		return nil, false, apperr.New(
			apperr.CodeConflict,
			"user lifecycle command identity conflict",
			ports.ErrInvalidUserLifecycle,
		)
	}
	return &dto.UserLifecycleReceipt{
		CommandID:        commandID,
		EventID:          receipt.EventID,
		Action:           action,
		User:             after,
		CommittedAt:      receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func validUserLifecycleTransitionProjection(
	action string,
	before dto.UserLifecycleProjection,
	after dto.UserLifecycleProjection,
) bool {
	if before.ID == uuid.Nil || after.ID != before.ID ||
		before.Version < 1 || after.Version != before.Version+1 {
		return false
	}
	switch action {
	case "suspend":
		return before.Status == string(entity.UserSecurityStatusActive) &&
			before.SuspendedAt == nil &&
			after.Status == string(entity.UserSecurityStatusSuspended) &&
			after.SuspendedAt != nil && !after.SuspendedAt.IsZero()
	case "reactivate":
		return before.Status == string(entity.UserSecurityStatusSuspended) &&
			before.SuspendedAt != nil && !before.SuspendedAt.IsZero() &&
			after.Status == string(entity.UserSecurityStatusActive) &&
			after.SuspendedAt == nil
	case "delete":
		return (before.Status == string(entity.UserSecurityStatusInvited) ||
			before.Status == string(entity.UserSecurityStatusActive) ||
			before.Status == string(entity.UserSecurityStatusSuspended)) &&
			after.Status == string(entity.UserSecurityStatusDeleted) &&
			after.SuspendedAt == nil
	default:
		return false
	}
}

func invalidUserLifecycle(message string) error {
	return apperr.New(apperr.CodeBadRequest, message, ports.ErrInvalidUserLifecycle)
}

func userLifecycleSelfConflict() error {
	return apperr.New(
		apperr.CodeConflict,
		"cannot change the current administrator account lifecycle",
		ports.ErrInvalidUserLifecycle,
	).WithDetails(map[string]any{"reason": "self_target_forbidden"})
}

func userLifecycleConflict(user *entity.User) error {
	result := apperr.New(
		apperr.CodeConflict,
		"user changed; reload and preview again",
		ports.ErrUserLifecycleConflict,
	)
	if version, ok := userLifecycleVersion(user); ok {
		return result.WithDetails(map[string]any{
			"reason":          "version_conflict",
			"current_version": version,
			"current_status":  string(user.SecurityStatus),
		})
	}
	return result.WithDetails(map[string]any{"reason": "version_conflict"})
}

func (s *userService) currentUserLifecycleConflict(
	ctx context.Context,
	id uuid.UUID,
) error {
	current, err := s.userRepo.Get(ctx, id)
	if err != nil {
		return userLifecycleConflict(nil)
	}
	return userLifecycleConflict(current)
}
