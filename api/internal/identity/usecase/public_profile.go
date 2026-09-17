package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	identitycontract "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	profilePrivacyAuditAction = "identity.user.profile_visibility.update"
	profilePrivacyAuditReason = "self_service_profile_visibility_change"
	profilePrivacyHashDomain  = "judgify/identity/profile-privacy-command/v1"
)

var canonicalPublicUsername = regexp.MustCompile(`^[A-Za-z0-9]{3,50}$`)

type publicProfileService struct {
	repository ports.PublicProfileRepository
	readTx     tx.Manager
	writeTx    tx.Manager
	audit      audit.Appender
	outbox     eventing.OutboxAppender
}

func NewPublicProfileService(
	repository ports.PublicProfileRepository,
	readTx tx.Manager,
	writeTx tx.Manager,
	auditAppender audit.Appender,
	outboxAppender eventing.OutboxAppender,
) (ports.PublicProfileService, error) {
	if repository == nil || readTx == nil || writeTx == nil ||
		auditAppender == nil || outboxAppender == nil {
		return nil, ports.ErrInvalidPublicProfile
	}
	return &publicProfileService{
		repository: repository,
		readTx:     readTx,
		writeTx:    writeTx,
		audit:      auditAppender,
		outbox:     outboxAppender,
	}, nil
}

func (service *publicProfileService) GetPublic(
	ctx context.Context,
	username string,
) (*dto.PublicProfileResponse, error) {
	if service == nil || service.repository == nil || service.readTx == nil ||
		ctx == nil || ctx.Err() != nil || !canonicalPublicUsername.MatchString(username) {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid public profile request",
			ports.ErrInvalidPublicProfile,
		)
	}
	var profile *entity.PublicProfile
	err := service.readTx.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		profile, err = service.repository.GetPublicByUsername(txCtx, username)
		return err
	})
	if err != nil {
		if errors.Is(err, ports.ErrPublicProfileNotFound) {
			return nil, apperr.New(
				apperr.CodeNotFound,
				"public profile not found",
				ports.ErrPublicProfileNotFound,
			)
		}
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"public profile unavailable",
			err,
		)
	}
	response, err := publicProfileResponse(profile)
	if err != nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"public profile unavailable",
			err,
		)
	}
	logger.FromContext(ctx).Debug(
		"public profile projected",
		zap.Int("attribute_count", len(response.Attributes)),
	)
	return response, nil
}

func (service *publicProfileService) GetPrivacy(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.ProfilePrivacyResponse, error) {
	if service == nil || service.repository == nil || service.readTx == nil ||
		ctx == nil || ctx.Err() != nil || userID == uuid.Nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid profile privacy request",
			ports.ErrInvalidProfilePrivacyCommand,
		)
	}
	var state *entity.ProfilePrivacyState
	err := service.readTx.DoInTx(ctx, func(txCtx context.Context) error {
		var err error
		state, err = service.repository.GetPrivacy(txCtx, userID)
		return err
	})
	if err != nil {
		if errors.Is(err, ports.ErrPublicProfileNotFound) {
			return nil, apperr.For("profile privacy").NotFound(err)
		}
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"profile privacy unavailable",
			err,
		)
	}
	response, err := profilePrivacyResponse(state)
	if err != nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"profile privacy unavailable",
			err,
		)
	}
	logger.FromContext(ctx).Debug(
		"profile privacy projected",
		zap.String("user_id", userID.String()),
		zap.String("visibility", response.Visibility),
		zap.Int64("profile_visibility_version", response.Version),
	)
	return response, nil
}

func (service *publicProfileService) UpdatePrivacy(
	ctx context.Context,
	command ports.UpdateProfilePrivacyCommand,
) (*dto.ProfilePrivacyMutationReceipt, error) {
	if err := service.validatePrivacyCommand(ctx, command); err != nil {
		return nil, err
	}
	requestHash, err := hashProfilePrivacyCommand(command)
	if err != nil {
		return nil, invalidProfilePrivacyCommand(err)
	}
	if receipt, found, err := service.loadPrivacyReceipt(
		ctx,
		command,
		requestHash,
	); err != nil {
		return nil, profilePrivacyReceiptError(err)
	} else if found {
		return receipt, nil
	}

	var result *dto.ProfilePrivacyMutationReceipt
	err = service.writeTx.DoInTx(ctx, func(txCtx context.Context) error {
		before, err := service.repository.GetPrivacy(txCtx, command.UserID)
		if err != nil {
			return err
		}
		if before == nil || !before.Valid() || before.UserID != command.UserID ||
			before.Version != command.ExpectedVersion ||
			before.Visibility == command.Visibility {
			return ports.ErrProfilePrivacyVersionConflict
		}
		after, err := service.repository.UpdatePrivacyCAS(
			txCtx,
			command.UserID,
			command.ExpectedVersion,
			command.Visibility,
		)
		if err != nil {
			return err
		}
		if after == nil || !after.Valid() || after.UserID != command.UserID ||
			after.Version != command.ExpectedVersion+1 ||
			after.Visibility != command.Visibility {
			return ports.ErrInvalidProfilePrivacyCommand
		}

		now := time.Now().UTC()
		auditID, err := service.audit.Append(txCtx, audit.Entry{
			ActorUserID:    &command.ActorID,
			ActorSessionID: &command.ActorSessionID,
			Action:         profilePrivacyAuditAction,
			Resource:       "identity.user_profile",
			ResourceID:     command.UserID.String(),
			Reason:         profilePrivacyAuditReason,
			Before:         profilePrivacyAuditState(*before),
			After:          profilePrivacyAuditState(*after),
			OccurredAt:     now,
		})
		if err != nil {
			return err
		}
		if auditID == uuid.Nil {
			return ports.ErrInvalidProfilePrivacyCommand
		}
		stateResponse, err := profilePrivacyResponse(after)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(identitycontract.ProfileVisibilityChangedV1{
			Version:          identitycontract.ProfileVisibilityPayloadVersionV1,
			ActorID:          command.ActorID.String(),
			ActorSessionID:   command.ActorSessionID.String(),
			CommandID:        command.CommandID.String(),
			RequestHash:      hex.EncodeToString(requestHash[:]),
			ExpectedVersion:  command.ExpectedVersion,
			ResultingVersion: after.Version,
			AuditID:          auditID.String(),
			State: identitycontract.ProfilePrivacyStateV1{
				Visibility: stateResponse.Visibility,
				Version:    stateResponse.Version,
				UpdatedAt:  stateResponse.UpdatedAt,
			},
		})
		if err != nil {
			return fmt.Errorf("marshal profile privacy receipt: %w", err)
		}
		if err := service.outbox.Append(txCtx, eventing.Event{
			ID:               command.CommandID,
			Type:             identitycontract.EventProfileVisibilityChangedV1,
			AggregateType:    identitycontract.AggregateUserProfilePrivacy,
			AggregateID:      command.UserID,
			AggregateVersion: after.Version,
			Payload:          payload,
			OccurredAt:       now,
		}); err != nil {
			return err
		}
		result = &dto.ProfilePrivacyMutationReceipt{
			CommandID:   command.CommandID,
			EventID:     command.CommandID,
			State:       *stateResponse,
			CommittedAt: now,
		}
		return nil
	})
	if err != nil {
		if receipt, found, replayErr := service.loadPrivacyReceipt(
			ctx,
			command,
			requestHash,
		); replayErr != nil {
			return nil, profilePrivacyReceiptError(replayErr)
		} else if found {
			return receipt, nil
		}
		if errors.Is(err, ports.ErrProfilePrivacyVersionConflict) {
			return nil, apperr.New(
				apperr.CodeConflict,
				"profile privacy changed; reload before retrying",
				err,
			)
		}
		if errors.Is(err, ports.ErrPublicProfileNotFound) {
			return nil, apperr.For("profile privacy").NotFound(err)
		}
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"profile privacy update unavailable",
			err,
		)
	}
	if result == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"profile privacy update unavailable",
			ports.ErrInvalidProfilePrivacyCommand,
		)
	}
	logger.FromContext(ctx).Info(
		"profile privacy updated",
		zap.String("user_id", command.UserID.String()),
		zap.String("visibility", string(command.Visibility)),
		zap.Int64("profile_visibility_version", result.State.Version),
		zap.String("event_id", result.EventID.String()),
	)
	return result, nil
}

func (service *publicProfileService) validatePrivacyCommand(
	ctx context.Context,
	command ports.UpdateProfilePrivacyCommand,
) error {
	if service == nil || service.repository == nil || service.writeTx == nil ||
		service.audit == nil || service.outbox == nil || ctx == nil ||
		ctx.Err() != nil || command.UserID == uuid.Nil ||
		command.ActorID == uuid.Nil || command.ActorID != command.UserID ||
		command.ActorSessionID == uuid.Nil || command.CommandID == uuid.Nil ||
		!command.Visibility.Valid() || command.ExpectedVersion < 1 {
		return invalidProfilePrivacyCommand(ports.ErrInvalidProfilePrivacyCommand)
	}
	return nil
}

func invalidProfilePrivacyCommand(cause error) error {
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid profile privacy command",
		cause,
	)
}

func profilePrivacyReceiptError(cause error) error {
	if errors.Is(cause, ports.ErrProfilePrivacyReceiptMismatch) {
		return cause
	}
	return apperr.New(
		apperr.CodeInternalServer,
		"profile privacy receipt unavailable",
		cause,
	)
}

func publicProfileResponse(
	profile *entity.PublicProfile,
) (*dto.PublicProfileResponse, error) {
	if profile == nil || profile.UserID == uuid.Nil ||
		!canonicalPublicUsername.MatchString(profile.Username) ||
		profile.JoinedAt.IsZero() ||
		len(profile.Attributes) > entity.MaximumPublicProfileAttributes {
		return nil, ports.ErrInvalidPublicProfile
	}
	response := &dto.PublicProfileResponse{
		Username:   profile.Username,
		JoinedAt:   profile.JoinedAt.UTC().Format(time.RFC3339Nano),
		Attributes: make([]dto.PublicProfileAttributeResponse, len(profile.Attributes)),
	}
	seen := make(map[string]struct{}, len(profile.Attributes))
	for index, attribute := range profile.Attributes {
		if !attributeDefinitionKeyPattern.MatchString(attribute.Key) ||
			!attribute.DataType.Valid() ||
			strings.TrimSpace(attribute.Label) != attribute.Label ||
			len(attribute.Label) < 2 || len(attribute.Label) > 128 ||
			len(attribute.Description) > 1_024 ||
			len(attribute.Value) == 0 ||
			len(attribute.Value) > attributevalue.MaxCanonicalBytes ||
			!json.Valid(attribute.Value) {
			return nil, ports.ErrInvalidPublicProfile
		}
		canonical, err := attributevalue.Canonicalize(
			attribute.Value,
			entity.AttributeDefinitionRevision{
				DataType:    attribute.DataType,
				Label:       attribute.Label,
				Description: attribute.Description,
				Visibility:  entity.AttributeVisibilityPublic,
				Validation:  json.RawMessage(`{}`),
			},
		)
		if err != nil || !bytes.Equal(canonical, attribute.Value) {
			return nil, ports.ErrInvalidPublicProfile
		}
		if _, duplicate := seen[attribute.Key]; duplicate {
			return nil, ports.ErrInvalidPublicProfile
		}
		seen[attribute.Key] = struct{}{}
		response.Attributes[index] = dto.PublicProfileAttributeResponse{
			Key:         attribute.Key,
			DataType:    string(attribute.DataType),
			Label:       attribute.Label,
			Description: attribute.Description,
			Value:       append(json.RawMessage(nil), attribute.Value...),
		}
	}
	return response, nil
}

func profilePrivacyResponse(
	state *entity.ProfilePrivacyState,
) (*dto.ProfilePrivacyResponse, error) {
	if state == nil || !state.Valid() {
		return nil, ports.ErrInvalidProfilePrivacyCommand
	}
	return &dto.ProfilePrivacyResponse{
		Visibility: string(state.Visibility),
		Version:    state.Version,
		UpdatedAt:  state.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func profilePrivacyAuditState(
	state entity.ProfilePrivacyState,
) map[string]any {
	return map[string]any{
		"visibility": string(state.Visibility),
		"version":    state.Version,
		"updated_at": state.UpdatedAt.UTC(),
	}
}

func hashProfilePrivacyCommand(
	command ports.UpdateProfilePrivacyCommand,
) ([sha256.Size]byte, error) {
	document, err := json.Marshal(struct {
		UserID          string `json:"user_id"`
		ActorID         string `json:"actor_id"`
		ActorSessionID  string `json:"actor_session_id"`
		Visibility      string `json:"visibility"`
		ExpectedVersion int64  `json:"expected_version"`
	}{
		UserID:          command.UserID.String(),
		ActorID:         command.ActorID.String(),
		ActorSessionID:  command.ActorSessionID.String(),
		Visibility:      string(command.Visibility),
		ExpectedVersion: command.ExpectedVersion,
	})
	if err != nil || command.UserID == uuid.Nil || command.ActorID == uuid.Nil ||
		command.ActorID != command.UserID || command.ActorSessionID == uuid.Nil ||
		!command.Visibility.Valid() || command.ExpectedVersion < 1 {
		return [sha256.Size]byte{}, ports.ErrInvalidProfilePrivacyCommand
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(profilePrivacyHashDomain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(document)
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func (service *publicProfileService) loadPrivacyReceipt(
	ctx context.Context,
	command ports.UpdateProfilePrivacyCommand,
	requestHash [sha256.Size]byte,
) (*dto.ProfilePrivacyMutationReceipt, bool, error) {
	receipt, found, err := service.repository.LoadPrivacyCommandReceipt(
		ctx,
		command.CommandID,
	)
	if err != nil || !found {
		return nil, false, err
	}
	payload, err := decodeProfilePrivacyReceipt(receipt.Payload)
	if err != nil || !matchesProfilePrivacyReceipt(
		receipt,
		payload,
		command,
		requestHash,
	) {
		return nil, false, apperr.New(
			apperr.CodeConflict,
			"idempotency key belongs to another profile privacy command",
			ports.ErrProfilePrivacyReceiptMismatch,
		)
	}
	return &dto.ProfilePrivacyMutationReceipt{
		CommandID: command.CommandID,
		EventID:   receipt.EventID,
		State: dto.ProfilePrivacyResponse{
			Visibility: payload.State.Visibility,
			Version:    payload.State.Version,
			UpdatedAt:  payload.State.UpdatedAt,
		},
		CommittedAt: receipt.OccurredAt.UTC(),
	}, true, nil
}

func decodeProfilePrivacyReceipt(
	raw json.RawMessage,
) (identitycontract.ProfileVisibilityChangedV1, error) {
	if len(raw) == 0 || len(raw) > 32<<10 {
		return identitycontract.ProfileVisibilityChangedV1{}, ports.ErrProfilePrivacyReceiptMismatch
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var payload identitycontract.ProfileVisibilityChangedV1
	if err := decoder.Decode(&payload); err != nil {
		return identitycontract.ProfileVisibilityChangedV1{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return identitycontract.ProfileVisibilityChangedV1{}, ports.ErrProfilePrivacyReceiptMismatch
	}
	return payload, nil
}

func matchesProfilePrivacyReceipt(
	receipt ports.ProfilePrivacyCommandReceipt,
	payload identitycontract.ProfileVisibilityChangedV1,
	command ports.UpdateProfilePrivacyCommand,
	requestHash [sha256.Size]byte,
) bool {
	storedHash, err := hex.DecodeString(payload.RequestHash)
	if err != nil || len(storedHash) != sha256.Size ||
		hex.EncodeToString(storedHash) != payload.RequestHash {
		return false
	}
	actorID, actorErr := uuid.Parse(payload.ActorID)
	actorSessionID, actorSessionErr := uuid.Parse(payload.ActorSessionID)
	commandID, commandErr := uuid.Parse(payload.CommandID)
	auditID, auditErr := uuid.Parse(payload.AuditID)
	updatedAt, updatedErr := time.Parse(time.RFC3339Nano, payload.State.UpdatedAt)
	return actorErr == nil && actorSessionErr == nil &&
		commandErr == nil && auditErr == nil &&
		updatedErr == nil && !updatedAt.IsZero() &&
		actorID.String() == payload.ActorID &&
		actorSessionID.String() == payload.ActorSessionID &&
		commandID.String() == payload.CommandID &&
		auditID.String() == payload.AuditID &&
		updatedAt.UTC().Format(time.RFC3339Nano) == payload.State.UpdatedAt &&
		receipt.EventID == command.CommandID &&
		receipt.EventType == identitycontract.EventProfileVisibilityChangedV1 &&
		receipt.AggregateType == identitycontract.AggregateUserProfilePrivacy &&
		receipt.AggregateID == command.UserID &&
		receipt.AggregateVersion == command.ExpectedVersion+1 &&
		!receipt.OccurredAt.IsZero() &&
		payload.Version == identitycontract.ProfileVisibilityPayloadVersionV1 &&
		actorID == command.ActorID && actorID == command.UserID &&
		actorSessionID == command.ActorSessionID &&
		commandID == command.CommandID && auditID != uuid.Nil &&
		payload.ExpectedVersion == command.ExpectedVersion &&
		payload.ResultingVersion == command.ExpectedVersion+1 &&
		payload.State.Visibility == string(command.Visibility) &&
		payload.State.Version == command.ExpectedVersion+1 &&
		subtle.ConstantTimeCompare(storedHash, requestHash[:]) == 1
}
