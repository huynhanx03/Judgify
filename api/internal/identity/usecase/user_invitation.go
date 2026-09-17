package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"
	"github.com/huynhanx03/go-common/pkg/logger"
	commonpassword "github.com/huynhanx03/go-common/pkg/security/password"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	identitycontract "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumUserInvitationPayloadBytes = 32 << 10
	maximumInvitationTokenBytes       = 4096
	initialInvitationSecurityVersion  = uint64(1)
)

type userInvitationService struct {
	users          ports.UserRepository
	credentials    ports.CredentialRepository
	roles          ports.RoleRepository
	authorization  ports.AuthorizationService
	attributes     ports.UserAttributeValueRepository
	profileSchema  ports.ProfileSchemaService
	recovery       ports.AccountInvitationLifecycle
	invitations    ports.AccountInvitationRepository
	scheduler      ports.SecurityEmailScheduler
	protector      *RecoveryContactProtector
	transactions   tx.Manager
	audit          audit.Appender
	outbox         eventing.OutboxAppender
	clock          func() time.Time
	passwordHasher *commonpassword.Hasher
}

func NewUserInvitationService(
	users ports.UserRepository,
	credentials ports.CredentialRepository,
	roles ports.RoleRepository,
	authorization ports.AuthorizationService,
	attributes ports.UserAttributeValueRepository,
	profileSchema ports.ProfileSchemaService,
	recovery ports.AccountInvitationLifecycle,
	invitations ports.AccountInvitationRepository,
	scheduler ports.SecurityEmailScheduler,
	protector *RecoveryContactProtector,
	transactions tx.Manager,
	auditAppender audit.Appender,
	outboxAppender eventing.OutboxAppender,
) (ports.UserInvitationService, error) {
	defaultHasher, _ := commonpassword.New(commonpassword.DefaultPolicy())
	service := &userInvitationService{
		users: users, credentials: credentials, roles: roles,
		authorization: authorization, attributes: attributes,
		profileSchema: profileSchema, recovery: recovery,
		invitations: invitations, scheduler: scheduler, protector: protector,
		transactions: transactions, audit: auditAppender,
		outbox: outboxAppender, clock: func() time.Time { return time.Now().UTC() },
		passwordHasher: defaultHasher,
	}
	if service.validateDependencies() != nil {
		return nil, ports.ErrInvalidUserInvitation
	}
	return service, nil
}

func (service *userInvitationService) hashPassword(ctx context.Context, pw string) (string, error) {
	if service.passwordHasher == nil {
		return "", ports.ErrInvalidUserInvitation
	}
	return service.passwordHasher.Hash(ctx, []byte(pw))
}

func (service *userInvitationService) verifyPassword(ctx context.Context, hash, pw string) error {
	if service.passwordHasher == nil {
		return ports.ErrInvalidUserInvitation
	}
	result, err := service.passwordHasher.Verify(ctx, hash, []byte(pw))
	if err != nil {
		return err
	}
	if !result.Valid {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return nil
}

func (service *userInvitationService) PreviewInvitationResend(
	ctx context.Context,
	request *dto.UserInvitationResendCommandRequest,
) (*dto.UserInvitationResendPreview, error) {
	if err := service.validateResendRequest(ctx, request, false); err != nil {
		return nil, err
	}
	state, err := service.invitations.GetInvitationResendState(ctx, request.ID)
	if err != nil {
		return nil, service.mapInvitationResendError("read account invitation", err)
	}
	if err := validateInvitationResendState(state, request); err != nil {
		return nil, err
	}
	masked, err := service.maskInvitationState(*state)
	if err != nil {
		return nil, invitationInternal("open invitation destination", err)
	}
	projection := invitationStateProjection(*state)
	logger.FromContext(ctx).Debug(
		"administrative invitation resend previewed",
		zap.String("user_id", state.UserID.String()),
		zap.Uint64("invitation_version", state.Contact.Version),
	)
	return &dto.UserInvitationResendPreview{
		User: projection, MaskedDestination: masked,
		ExpectedInvitationVersion:  state.Contact.Version,
		ResultingInvitationVersion: state.Contact.Version + 1,
		CurrentExpiresAt:           state.ExpiresAt.UTC(),
	}, nil
}

func (service *userInvitationService) ApplyInvitationResend(
	ctx context.Context,
	request *dto.UserInvitationResendCommandRequest,
	requestHash [sha256.Size]byte,
) (*dto.UserInvitationResendReceipt, error) {
	if err := service.validateResendRequest(ctx, request, true); err != nil ||
		requestHash == ([sha256.Size]byte{}) {
		return nil, invalidUserInvitation("invalid invitation resend command", err)
	}
	actorID, sessionID, err := invitationActor(ctx)
	if err != nil {
		return nil, err
	}
	if receipt, found, loadErr := service.loadResendReceipt(
		ctx, request, requestHash, actorID, sessionID, true,
	); loadErr != nil || found {
		return receipt, loadErr
	}
	state, err := service.invitations.GetInvitationResendState(ctx, request.ID)
	if err != nil {
		return nil, service.mapInvitationResendError("read account invitation", err)
	}
	if err := validateInvitationResendState(state, request); err != nil {
		return nil, err
	}
	prepared, err := service.recovery.PrepareInvitationResend(*state)
	if err != nil {
		return nil, err
	}
	if prepared == nil || prepared.Contact.Version != request.ExpectedInvitationVersion+1 ||
		prepared.Contact.UserID != request.ID || prepared.TokenID == uuid.Nil ||
		prepared.Delivery.VerificationTokenID != prepared.TokenID {
		return nil, invitationInternal("prepare invitation resend", ports.ErrInvalidUserInvitation)
	}
	projection := invitationStateProjection(*state)
	var result *dto.UserInvitationResendReceipt
	err = service.transactions.DoInTx(ctx, func(transactionContext context.Context) error {
		if err := service.invitations.RotateInvitation(
			transactionContext,
			ports.AccountInvitationResendCommand{
				UserID:                      request.ID,
				ExpectedUserSecurityVersion: uint64(request.ExpectedUserVersion),
				ExpectedContactVersion:      request.ExpectedInvitationVersion,
				ExpectedTokenID:             state.LastTokenID,
				Prepared:                    *prepared,
			},
		); err != nil {
			return err
		}
		if err := service.scheduler.Enqueue(
			transactionContext,
			cloneSecurityEmailIntentForPersistence(prepared.Delivery),
		); err != nil {
			return err
		}
		now := prepared.IssuedAt.UTC()
		auditID, err := service.audit.Append(transactionContext, audit.Entry{
			ActorUserID: &actorID, ActorSessionID: &sessionID,
			Action:   "identity.user.invitation.resend",
			Resource: "identity.user", ResourceID: request.ID.String(),
			Reason: request.Reason,
			Before: map[string]any{
				"user_id":               request.ID.String(),
				"status":                string(entity.UserSecurityStatusInvited),
				"user_version":          request.ExpectedUserVersion,
				"invitation_version":    request.ExpectedInvitationVersion,
				"delivery_state":        string(state.DeliveryState),
				"invitation_expires_at": state.ExpiresAt.UTC().Format(time.RFC3339Nano),
			},
			After: map[string]any{
				"user_id":               request.ID.String(),
				"status":                string(entity.UserSecurityStatusInvited),
				"user_version":          request.ExpectedUserVersion,
				"invitation_version":    prepared.Contact.Version,
				"delivery_state":        "queued",
				"invitation_expires_at": prepared.ExpiresAt.UTC().Format(time.RFC3339Nano),
			},
			OccurredAt: now,
		})
		if err != nil {
			return err
		}
		payload := identitycontract.UserInvitationResentV1{
			Version:     identitycontract.UserInvitationPayloadVersionV1,
			CommandID:   request.CommandID.String(),
			RequestHash: hex.EncodeToString(requestHash[:]),
			AuditID:     auditID.String(), ActorID: actorID.String(),
			ActorSessionID:             sessionID.String(),
			User:                       lifecycleContractProjection(projection),
			ExpectedInvitationVersion:  request.ExpectedInvitationVersion,
			ResultingInvitationVersion: prepared.Contact.Version,
			InvitationExpiresAt:        prepared.ExpiresAt.UTC(),
		}
		payloadBytes, err := commonjson.Marshal(payload)
		if err != nil || len(payloadBytes) == 0 ||
			len(payloadBytes) > maximumUserInvitationPayloadBytes {
			return ports.ErrInvalidUserInvitation
		}
		if err := service.outbox.Append(transactionContext, eventing.Event{
			ID: request.CommandID, Type: identitycontract.EventUserInvitationResentV1,
			AggregateType:    identitycontract.AggregateUserInvitation,
			AggregateID:      request.ID,
			AggregateVersion: int64(prepared.Contact.Version),
			Payload:          payloadBytes, OccurredAt: now,
		}); err != nil {
			return err
		}
		result = &dto.UserInvitationResendReceipt{
			CommandID: request.CommandID, EventID: request.CommandID,
			User: projection, InvitationVersion: prepared.Contact.Version,
			InvitationExpiresAt: prepared.ExpiresAt.UTC(),
			DeliveryState:       "queued", CommittedAt: now,
		}
		return nil
	})
	if err != nil {
		if receipt, found, replayErr := service.loadResendReceipt(
			ctx, request, requestHash, actorID, sessionID, true,
		); replayErr != nil || found {
			return receipt, replayErr
		}
		return nil, service.mapInvitationResendError("resend account invitation", err)
	}
	if result == nil {
		return nil, invitationInternal("resend account invitation", ports.ErrInvalidUserInvitation)
	}
	logger.FromContext(ctx).Info(
		"administrative account invitation resent",
		zap.String("command_id", request.CommandID.String()),
		zap.String("user_id", request.ID.String()),
		zap.Uint64("invitation_version", result.InvitationVersion),
	)
	return result, nil
}

func (service *userInvitationService) PreviewInvitation(
	ctx context.Context,
	request *dto.UserInvitationCommandRequest,
) (*dto.UserInvitationPreview, error) {
	if err := service.validateRequest(ctx, request, false); err != nil {
		return nil, err
	}
	username, recoveryEmail, roles, err := service.prepareInput(ctx, request)
	if err != nil {
		return nil, err
	}
	exists, err := service.users.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, invitationUsernameConflict()
	}
	var valueCount int
	err = service.transactions.DoInTx(ctx, func(transactionContext context.Context) error {
		mutations, resolveErr := service.profileSchema.ResolveRequiredValues(
			transactionContext,
			request.CommandID,
			request.ProfileSchemaEvidence,
		)
		if resolveErr != nil {
			return resolveErr
		}
		valueCount = len(mutations)
		return nil
	})
	if err != nil {
		return nil, err
	}
	masked, err := MaskRecoveryEmail(recoveryEmail)
	if err != nil {
		return nil, invalidUserInvitation("invalid recovery destination", err)
	}
	logger.FromContext(ctx).Debug(
		"administrative user invitation previewed",
		zap.String("username", username),
		zap.Int("role_count", len(roles)),
		zap.Int("profile_attribute_count", valueCount),
	)
	return &dto.UserInvitationPreview{
		Username: username, MaskedDestination: masked,
		Roles: roles, ProfileValueCount: valueCount,
	}, nil
}

func (service *userInvitationService) ApplyInvitation(
	ctx context.Context,
	request *dto.UserInvitationCommandRequest,
	requestHash [sha256.Size]byte,
) (*dto.UserInvitationReceipt, error) {
	if err := service.validateRequest(ctx, request, true); err != nil ||
		requestHash == ([sha256.Size]byte{}) {
		return nil, invalidUserInvitation("invalid invitation command", err)
	}
	actorID, sessionID, err := invitationActor(ctx)
	if err != nil {
		return nil, err
	}
	if receipt, found, loadErr := service.loadReceipt(
		ctx,
		request,
		requestHash,
		actorID,
		sessionID,
		true,
	); loadErr != nil || found {
		return receipt, loadErr
	}
	username, recoveryEmail, roles, err := service.prepareInput(ctx, request)
	if err != nil {
		return nil, err
	}
	exists, err := service.users.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, invitationUsernameConflict()
	}
	userID, err := uuid.NewV7()
	if err != nil {
		return nil, invitationInternal("generate invited user identifier", err)
	}
	preparedRecovery, err := service.recovery.PrepareInvitation(userID, recoveryEmail)
	if err != nil {
		return nil, err
	}
	if preparedRecovery == nil || preparedRecovery.Issue.Contact.UserID != userID ||
		preparedRecovery.Issue.Contact.Purpose != entity.RecoveryContactPurposeInvitation ||
		preparedRecovery.Issue.ExpiresAt.IsZero() {
		return nil, invitationInternal("prepare account invitation", ports.ErrInvalidUserInvitation)
	}
	invitedUser := &entity.User{
		ID: userID, Username: username,
		SecurityStatus:  entity.UserSecurityStatusInvited,
		SecurityVersion: initialInvitationSecurityVersion,
	}
	var result *dto.UserInvitationReceipt
	err = service.transactions.DoInTx(ctx, func(transactionContext context.Context) error {
		mutations, resolveErr := service.profileSchema.ResolveRequiredValues(
			transactionContext,
			userID,
			request.ProfileSchemaEvidence,
		)
		if resolveErr != nil {
			return resolveErr
		}
		if err := persistPreparedAccount(
			transactionContext,
			service.users,
			service.credentials,
			service.attributes,
			service.authorization,
			invitedUser,
			nil,
			mutations,
			roles,
		); err != nil {
			return err
		}
		if err := service.recovery.PersistInvitation(transactionContext, preparedRecovery); err != nil {
			return err
		}
		now := service.clock().UTC()
		projection := lifecycleProjection(invitedUser)
		if projection.Status != string(entity.UserSecurityStatusInvited) || projection.Version != 1 {
			return ports.ErrInvalidUserInvitation
		}
		auditID, err := service.audit.Append(transactionContext, audit.Entry{
			ActorUserID: &actorID, ActorSessionID: &sessionID,
			Action:   "identity.user.invitation.create",
			Resource: "identity.user", ResourceID: userID.String(),
			Reason: request.Reason,
			Before: map[string]any{}, After: userInvitationAudit(projection, roles, len(mutations)),
			OccurredAt: now,
		})
		if err != nil {
			return err
		}
		payload := identitycontract.UserInvitationCreatedV1{
			Version:     identitycontract.UserInvitationPayloadVersionV1,
			CommandID:   request.CommandID.String(),
			RequestHash: hex.EncodeToString(requestHash[:]),
			AuditID:     auditID.String(), ActorID: actorID.String(),
			ActorSessionID:      sessionID.String(),
			User:                lifecycleContractProjection(projection),
			Roles:               append([]string(nil), roles...),
			ProfileValueCount:   len(mutations),
			InvitationExpiresAt: preparedRecovery.Issue.ExpiresAt.UTC(),
		}
		payloadBytes, err := commonjson.Marshal(payload)
		if err != nil || len(payloadBytes) == 0 ||
			len(payloadBytes) > maximumUserInvitationPayloadBytes {
			return ports.ErrInvalidUserInvitation
		}
		if err := service.outbox.Append(transactionContext, eventing.Event{
			ID: request.CommandID, Type: identitycontract.EventUserInvitationCreatedV1,
			AggregateType: identitycontract.AggregateUser,
			AggregateID:   userID, AggregateVersion: projection.Version,
			Payload: payloadBytes, OccurredAt: now,
		}); err != nil {
			return err
		}
		result = &dto.UserInvitationReceipt{
			CommandID: request.CommandID, EventID: request.CommandID,
			User: projection, Roles: append([]string(nil), roles...),
			ProfileValueCount:   len(mutations),
			InvitationExpiresAt: preparedRecovery.Issue.ExpiresAt.UTC(),
			DeliveryState:       "queued", CommittedAt: now,
		}
		return nil
	})
	if err != nil {
		if receipt, found, replayErr := service.loadReceipt(
			ctx,
			request,
			requestHash,
			actorID,
			sessionID,
			true,
		); replayErr != nil || found {
			return receipt, replayErr
		}
		if errors.Is(err, ports.ErrProfileSchemaConflict) {
			return nil, err
		}
		// The optimistic preflight is only a UX fast path. Resolve a concurrent
		// unique-username winner to the same stable domain conflict after this
		// transaction rolls back, without parsing provider-specific SQL text.
		if exists, lookupErr := service.users.ExistsByUsername(ctx, username); lookupErr == nil && exists {
			return nil, invitationUsernameConflict()
		}
		return nil, err
	}
	if result == nil {
		return nil, invitationInternal("persist account invitation", ports.ErrInvalidUserInvitation)
	}
	logger.FromContext(ctx).Info(
		"administrative user invitation committed",
		zap.String("command_id", request.CommandID.String()),
		zap.String("user_id", result.User.ID.String()),
		zap.String("username", result.User.Username),
		zap.Int("role_count", len(result.Roles)),
		zap.Int("profile_attribute_count", result.ProfileValueCount),
	)
	return result, nil
}

func (service *userInvitationService) ActivateInvitation(
	ctx context.Context,
	request *dto.ActivateUserInvitationRequest,
) (*dto.ActivateUserInvitationResponse, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || request == nil ||
		len(request.Token) == 0 || len(request.Token) > maximumInvitationTokenBytes ||
		len(request.NewPassword) < 8 || len(request.NewPassword) > 256 ||
		!utf8.ValidString(request.NewPassword) {
		return nil, rejectedUserInvitation(ports.ErrAccountInvitationRejected)
	}
	candidates, err := service.protector.InvitationTokenCandidates(
		request.Token,
		maximumInvitationTokenBytes,
	)
	if err != nil {
		return nil, rejectedUserInvitation(err)
	}
	passwordHash, err := service.hashPassword(ctx, request.NewPassword)
	if err != nil {
		return nil, invitationInternal("hash invitation password", err)
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return nil, invitationInternal("generate invitation activation event", err)
	}
	now := service.clock().UTC()
	var activatedUserID uuid.UUID
	err = service.transactions.DoInTx(ctx, func(transactionContext context.Context) error {
		authority, lockErr := service.invitations.LockInvitation(
			transactionContext,
			candidates,
			now,
		)
		if lockErr != nil {
			return lockErr
		}
		if authority == nil || authority.UserID == uuid.Nil ||
			authority.UserSecurityVersion == 0 || authority.UserSecurityVersion >= math.MaxInt64 {
			return ports.ErrAccountInvitationRejected
		}
		activatedUserID = authority.UserID
		if authority.Accepted {
			credential, credentialErr := service.credentials.GetByUserID(
				transactionContext,
				authority.UserID,
				credentialTypePassword,
			)
			if credentialErr != nil {
				return credentialErr
			}
			hash, valid := invitationCredentialHash(credential, authority.UserID)
			if !valid {
				return ports.ErrAccountInvitationConflict
			}
			if compareErr := service.verifyPassword(ctx, hash, request.NewPassword); compareErr != nil {
				if errors.Is(compareErr, bcrypt.ErrMismatchedHashAndPassword) {
					return ports.ErrAccountInvitationRejected
				}
				return compareErr
			}
			return nil
		}
		credential := &entity.Credential{
			UserID: authority.UserID, Type: credentialTypePassword,
			CredentialData: map[string]any{credentialKeyHash: passwordHash},
		}
		if err := service.credentials.Create(transactionContext, credential); err != nil {
			return err
		}
		updated, err := service.invitations.AcceptInvitation(transactionContext, *authority, now)
		if err != nil {
			return err
		}
		if updated == nil || updated.ID != authority.UserID ||
			updated.SecurityStatus != entity.UserSecurityStatusActive ||
			updated.SecurityVersion != authority.UserSecurityVersion+1 {
			return ports.ErrAccountInvitationConflict
		}
		before := dto.UserLifecycleProjection{
			ID: authority.UserID, Username: updated.Username,
			Status:  string(entity.UserSecurityStatusInvited),
			Version: int64(authority.UserSecurityVersion),
		}
		after := lifecycleProjection(updated)
		auditID, err := service.audit.Append(transactionContext, audit.Entry{
			Action:   "identity.user.invitation.accept",
			Resource: "identity.user", ResourceID: authority.UserID.String(),
			Reason: "account_invitation_accepted",
			Before: userLifecycleAudit(before), After: userLifecycleAudit(after),
			OccurredAt: now,
		})
		if err != nil {
			return err
		}
		payloadBytes, err := commonjson.Marshal(identitycontract.UserInvitationAcceptedV1{
			Version: identitycontract.UserInvitationPayloadVersionV1,
			AuditID: auditID.String(), UserID: authority.UserID.String(),
			Before: lifecycleContractProjection(before),
			After:  lifecycleContractProjection(after), AcceptedAt: now,
		})
		if err != nil || len(payloadBytes) == 0 ||
			len(payloadBytes) > maximumUserInvitationPayloadBytes {
			return ports.ErrInvalidUserInvitation
		}
		return service.outbox.Append(transactionContext, eventing.Event{
			ID: eventID, Type: identitycontract.EventUserInvitationAcceptedV1,
			AggregateType: identitycontract.AggregateUser,
			AggregateID:   authority.UserID, AggregateVersion: after.Version,
			Payload: payloadBytes, OccurredAt: now,
		})
	})
	if err != nil {
		if errors.Is(err, ports.ErrAccountInvitationRejected) {
			logger.FromContext(ctx).Debug("account invitation activation rejected")
			return nil, rejectedUserInvitation(err)
		}
		if errors.Is(err, ports.ErrAccountInvitationConflict) {
			return nil, invitationInternal("activate account invitation", err)
		}
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"account invitation activated",
		zap.String("user_id", activatedUserID.String()),
	)
	return &dto.ActivateUserInvitationResponse{Success: true}, nil
}

func (service *userInvitationService) prepareInput(
	ctx context.Context,
	request *dto.UserInvitationCommandRequest,
) (string, string, []string, error) {
	username := strings.TrimSpace(request.Username)
	if !validInvitationUsername(username) {
		return "", "", nil, invalidUserInvitation("invalid username", nil)
	}
	recoveryEmail, err := NormalizeRecoveryEmail(
		request.RecoveryEmail,
		defaultMaximumRecoveryEmailBytes,
	)
	if err != nil {
		return "", "", nil, invalidUserInvitation("invalid recovery destination", err)
	}
	roles, err := canonicalInvitationRoles(request.Roles)
	if err != nil {
		return "", "", nil, invalidUserInvitation("invalid role selection", err)
	}
	for _, roleKey := range roles {
		role, roleErr := service.roles.GetByKey(ctx, roleKey)
		if roleErr != nil {
			return "", "", nil, roleErr
		}
		if role == nil || role.Key != roleKey || role.Status != entity.RoleStatusActive {
			return "", "", nil, invalidUserInvitation("role is not active", ports.ErrInvalidUserInvitation)
		}
	}
	return username, recoveryEmail, roles, nil
}

func (service *userInvitationService) validateDependencies() error {
	if service == nil || isNilTokenDependency(service.users) ||
		isNilTokenDependency(service.credentials) || isNilTokenDependency(service.roles) ||
		isNilTokenDependency(service.authorization) || isNilTokenDependency(service.attributes) ||
		isNilTokenDependency(service.profileSchema) || isNilTokenDependency(service.recovery) ||
		isNilTokenDependency(service.invitations) || isNilTokenDependency(service.scheduler) ||
		service.protector == nil ||
		isNilTokenDependency(service.transactions) || isNilTokenDependency(service.audit) ||
		isNilTokenDependency(service.outbox) || service.clock == nil {
		return ports.ErrInvalidUserInvitation
	}
	return nil
}

func (service *userInvitationService) validateResendRequest(
	ctx context.Context,
	request *dto.UserInvitationResendCommandRequest,
	requireConfirmation bool,
) error {
	if err := service.validateDependencies(); err != nil || ctx == nil || ctx.Err() != nil ||
		request == nil || request.ID == uuid.Nil || request.CommandID == uuid.Nil ||
		request.ExpectedUserVersion < 1 ||
		request.ExpectedInvitationVersion >= uint64(math.MaxInt64) ||
		(requireConfirmation && request.ExpectedInvitationVersion == 0) ||
		!validInvitationReason(request.Reason) ||
		(requireConfirmation && strings.TrimSpace(request.ConfirmationToken) == "") ||
		len(request.ConfirmationToken) > 2048 {
		return invalidUserInvitation("invalid invitation resend command", err)
	}
	return nil
}

func validateInvitationResendState(
	state *ports.AccountInvitationResendState,
	request *dto.UserInvitationResendCommandRequest,
) error {
	if state == nil || request == nil || state.UserID != request.ID ||
		state.Username == "" || state.UserSecurityVersion == 0 ||
		state.UserSecurityVersion >= math.MaxInt64 ||
		int64(state.UserSecurityVersion) != request.ExpectedUserVersion ||
		state.Contact.ID == uuid.Nil || state.Contact.UserID != request.ID ||
		state.Contact.Purpose != entity.RecoveryContactPurposeInvitation ||
		state.Contact.Status != entity.RecoveryContactStatusPending ||
		state.Contact.Version == 0 || state.Contact.Version >= uint64(math.MaxInt64) ||
		state.LastTokenID == uuid.Nil || state.LastIssuedAt.IsZero() ||
		state.ExpiresAt.IsZero() ||
		(request.ExpectedInvitationVersion != 0 &&
			state.Contact.Version != request.ExpectedInvitationVersion) {
		return apperr.New(
			apperr.CodeConflict,
			"account invitation changed; reload and try again",
			ports.ErrAccountInvitationConflict,
		).WithDetails(map[string]any{"reason": "invitation_changed"})
	}
	return nil
}

func (service *userInvitationService) maskInvitationState(
	state ports.AccountInvitationResendState,
) (string, error) {
	contact := state.Contact
	address, err := service.protector.Open(RecoveryContactMetadata{
		UserID: contact.UserID, ContactID: contact.ID,
		Kind: contact.Kind, Version: contact.Version,
	}, contact.KeyID, contact.AddressCiphertext)
	if err != nil {
		return "", err
	}
	return MaskRecoveryEmail(address)
}

func invitationStateProjection(
	state ports.AccountInvitationResendState,
) dto.UserLifecycleProjection {
	return dto.UserLifecycleProjection{
		ID: state.UserID, Username: state.Username,
		Status:  string(entity.UserSecurityStatusInvited),
		Version: int64(state.UserSecurityVersion),
	}
}

func (service *userInvitationService) loadResendReceipt(
	ctx context.Context,
	request *dto.UserInvitationResendCommandRequest,
	requestHash [sha256.Size]byte,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	replayed bool,
) (*dto.UserInvitationResendReceipt, bool, error) {
	receipt, found, err := service.users.LoadCommandReceipt(ctx, request.CommandID)
	if err != nil || !found {
		return nil, found, err
	}
	var payload identitycontract.UserInvitationResentV1
	decodeErr := commonjson.UnmarshalStrictLimit(
		receipt.Payload,
		&payload,
		maximumUserInvitationPayloadBytes,
	)
	projection, projectionErr := lifecycleDTOProjection(payload.User)
	auditID, auditErr := uuid.Parse(payload.AuditID)
	if receipt.EventID != request.CommandID ||
		receipt.EventType != identitycontract.EventUserInvitationResentV1 ||
		receipt.AggregateType != identitycontract.AggregateUserInvitation ||
		receipt.AggregateID != request.ID ||
		receipt.AggregateVersion != int64(request.ExpectedInvitationVersion+1) ||
		receipt.OccurredAt.IsZero() || decodeErr != nil || projectionErr != nil ||
		payload.Version != identitycontract.UserInvitationPayloadVersionV1 ||
		payload.CommandID != request.CommandID.String() ||
		payload.RequestHash != hex.EncodeToString(requestHash[:]) ||
		payload.AuditID != auditID.String() || auditErr != nil || auditID == uuid.Nil ||
		payload.ActorID != actorID.String() || payload.ActorSessionID != sessionID.String() ||
		projection.ID != request.ID || projection.Username == "" ||
		projection.Status != string(entity.UserSecurityStatusInvited) ||
		projection.Version != request.ExpectedUserVersion || projection.SuspendedAt != nil ||
		payload.ExpectedInvitationVersion != request.ExpectedInvitationVersion ||
		payload.ResultingInvitationVersion != request.ExpectedInvitationVersion+1 ||
		payload.ResultingInvitationVersion > math.MaxInt64 ||
		!payload.InvitationExpiresAt.After(receipt.OccurredAt) {
		return nil, false, invitationCommandConflict()
	}
	return &dto.UserInvitationResendReceipt{
		CommandID: request.CommandID, EventID: receipt.EventID,
		User: projection, InvitationVersion: payload.ResultingInvitationVersion,
		InvitationExpiresAt: payload.InvitationExpiresAt.UTC(),
		DeliveryState:       "queued", CommittedAt: receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func (service *userInvitationService) mapInvitationResendError(
	operation string,
	cause error,
) error {
	switch {
	case errors.Is(cause, ports.ErrRecoveryContactRateLimited):
		return apperr.New(
			apperr.CodeTooManyRequests,
			"account invitation resend limit reached",
			cause,
		).WithDetails(map[string]any{"reason": "invitation_rate_limited"})
	case errors.Is(cause, ports.ErrAccountInvitationConflict):
		return apperr.New(
			apperr.CodeConflict,
			"account invitation changed; reload and try again",
			cause,
		).WithDetails(map[string]any{"reason": "invitation_changed"})
	default:
		return invitationInternal(operation, cause)
	}
}

func (service *userInvitationService) validateRequest(
	ctx context.Context,
	request *dto.UserInvitationCommandRequest,
	requireConfirmation bool,
) error {
	if err := service.validateDependencies(); err != nil || ctx == nil || ctx.Err() != nil ||
		request == nil || request.CommandID == uuid.Nil ||
		!validInvitationReason(request.Reason) ||
		(requireConfirmation && strings.TrimSpace(request.ConfirmationToken) == "") ||
		len(request.ConfirmationToken) > 2048 {
		return invalidUserInvitation("invalid invitation command", err)
	}
	return nil
}

func (service *userInvitationService) loadReceipt(
	ctx context.Context,
	request *dto.UserInvitationCommandRequest,
	requestHash [sha256.Size]byte,
	actorID uuid.UUID,
	sessionID uuid.UUID,
	replayed bool,
) (*dto.UserInvitationReceipt, bool, error) {
	receipt, found, err := service.users.LoadCommandReceipt(ctx, request.CommandID)
	if err != nil || !found {
		return nil, found, err
	}
	var payload identitycontract.UserInvitationCreatedV1
	decodeErr := commonjson.UnmarshalStrictLimit(
		receipt.Payload,
		&payload,
		maximumUserInvitationPayloadBytes,
	)
	projection, projectionErr := lifecycleDTOProjection(payload.User)
	auditID, auditErr := uuid.Parse(payload.AuditID)
	canonicalRoles, rolesErr := canonicalInvitationRoles(payload.Roles)
	if receipt.EventID != request.CommandID ||
		receipt.EventType != identitycontract.EventUserInvitationCreatedV1 ||
		receipt.AggregateType != identitycontract.AggregateUser ||
		receipt.AggregateID == uuid.Nil || receipt.AggregateID != projection.ID ||
		receipt.AggregateVersion != 1 || receipt.OccurredAt.IsZero() ||
		decodeErr != nil || projectionErr != nil ||
		payload.Version != identitycontract.UserInvitationPayloadVersionV1 ||
		payload.CommandID != request.CommandID.String() ||
		payload.RequestHash != hex.EncodeToString(requestHash[:]) ||
		payload.AuditID != auditID.String() || auditErr != nil || auditID == uuid.Nil ||
		payload.ActorID != actorID.String() || payload.ActorSessionID != sessionID.String() ||
		projection.ID == uuid.Nil || projection.Username != strings.TrimSpace(request.Username) ||
		projection.Status != string(entity.UserSecurityStatusInvited) ||
		projection.Version != 1 || projection.SuspendedAt != nil ||
		rolesErr != nil || !sameInvitationStrings(canonicalRoles, payload.Roles) ||
		!sameInvitationStrings(canonicalRoles, request.Roles) ||
		payload.ProfileValueCount != len(request.ProfileValues) ||
		payload.ProfileValueCount < 1 ||
		!payload.InvitationExpiresAt.After(receipt.OccurredAt) {
		return nil, false, invitationCommandConflict()
	}
	return &dto.UserInvitationReceipt{
		CommandID: request.CommandID, EventID: receipt.EventID,
		User: projection, Roles: canonicalRoles,
		ProfileValueCount:   payload.ProfileValueCount,
		InvitationExpiresAt: payload.InvitationExpiresAt.UTC(),
		DeliveryState:       "queued", CommittedAt: receipt.OccurredAt.UTC(),
		IdempotentReplay: replayed,
	}, true, nil
}

func canonicalInvitationRoles(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 64 {
		return nil, ports.ErrInvalidUserInvitation
	}
	result := make([]string, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if len(value) < 2 || len(value) > 50 || !utf8.ValidString(value) {
			return nil, ports.ErrInvalidUserInvitation
		}
		result[index] = value
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index] == result[index-1] {
			return nil, ports.ErrInvalidUserInvitation
		}
	}
	return result, nil
}

func validInvitationUsername(value string) bool {
	if len(value) < 3 || len(value) > 50 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validInvitationReason(value string) bool {
	return validUserLifecycleReason(value)
}

func invitationActor(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	actorID, actorOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !actorOK || !sessionOK || actorID == uuid.Nil || sessionID == uuid.Nil {
		return uuid.Nil, uuid.Nil, apperr.New(
			apperr.CodeUnauthorized,
			"authentication required",
			ports.ErrInvalidUserInvitation,
		)
	}
	return actorID, sessionID, nil
}

func invitationCredentialHash(
	credential *entity.Credential,
	userID uuid.UUID,
) (string, bool) {
	if credential == nil || credential.UserID != userID ||
		credential.Type != credentialTypePassword || credential.CredentialData == nil {
		return "", false
	}
	hash, ok := credential.CredentialData[credentialKeyHash].(string)
	return hash, ok && hash != ""
}

func sameInvitationStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func userInvitationAudit(
	projection dto.UserLifecycleProjection,
	roles []string,
	profileValueCount int,
) map[string]any {
	result := userLifecycleAudit(projection)
	result["roles"] = append([]string(nil), roles...)
	result["profile_value_count"] = profileValueCount
	result["delivery_state"] = "queued"
	return result
}

func invalidUserInvitation(message string, cause error) error {
	if cause == nil {
		cause = ports.ErrInvalidUserInvitation
	}
	return apperr.New(apperr.CodeBadRequest, message, errors.Join(ports.ErrInvalidUserInvitation, cause))
}

func rejectedUserInvitation(cause error) error {
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid or expired account invitation",
		errors.Join(ports.ErrAccountInvitationRejected, cause),
	)
}

func invitationUsernameConflict() error {
	return apperr.New(
		apperr.CodeConflict,
		"username already exists",
		ports.ErrUserInvitationConflict,
	).WithDetails(map[string]any{"reason": "username_exists"})
}

func invitationCommandConflict() error {
	return apperr.New(
		apperr.CodeConflict,
		"invitation command identity conflict",
		ports.ErrUserInvitationConflict,
	).WithDetails(map[string]any{"reason": "idempotency_conflict"})
}

func invitationInternal(message string, cause error) error {
	return apperr.New(
		apperr.CodeInternalServer,
		message,
		errors.Join(ports.ErrInvalidUserInvitation, cause),
	)
}

var _ ports.UserInvitationService = (*userInvitationService)(nil)
