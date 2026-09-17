package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

// ConfigureManagement installs the authentication dependencies needed by the
// self-service surface. Composition calls it once before the service is made
// visible; request-time mutation of these dependencies is unsupported.
func (service *RecoveryContactService) ConfigureManagement(
	credentials ports.CredentialRepository,
	sessions ports.RecoveryContactSessionRepository,
) error {
	if service == nil || service.repository == nil ||
		isNilTokenDependency(credentials) || isNilTokenDependency(sessions) {
		return ErrInvalidRecoveryContactLifecycle
	}
	service.credentials = credentials
	service.sessions = sessions
	return nil
}

func (service *RecoveryContactService) Get(
	ctx context.Context,
	userID, sessionID uuid.UUID,
) (*entity.RecoveryContactProjection, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || service.repository == nil ||
		userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, recoveryContactInternal("read recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	now := service.config.Clock().UTC()
	state, err := service.repository.GetState(ctx, userID, sessionID, now)
	if err != nil {
		if errors.Is(err, ports.ErrRecoveryContactRejected) {
			return nil, apperr.New(apperr.CodeUnauthorized, constant.MsgUnauthorized, err)
		}
		return nil, recoveryContactInternal("read recovery contact", err)
	}
	if state == nil || state.UserID != userID || state.SessionID != sessionID {
		return nil, recoveryContactInternal("read recovery contact", ErrInvalidRecoveryContactLifecycle)
	}

	projection := &entity.RecoveryContactProjection{}
	if state.Current != nil {
		projection.Current, err = service.maskContact(*state.Current)
		if err != nil || projection.Current.Status != entity.RecoveryContactStatusVerified {
			return nil, recoveryContactInternal("open current recovery contact", errors.Join(ErrInvalidRecoveryContactLifecycle, err))
		}
	}
	if state.Pending != nil {
		view, maskErr := service.maskContact(*state.Pending)
		if maskErr != nil || view.Status != entity.RecoveryContactStatusPending ||
			state.PendingVerificationExpiresAt.IsZero() || state.PendingLastIssuedAt.IsZero() ||
			!validRecoveryDeliveryState(state.PendingDeliveryState) {
			return nil, recoveryContactInternal("open pending recovery contact", errors.Join(ErrInvalidRecoveryContactLifecycle, maskErr))
		}
		projection.Pending = &entity.PendingRecoveryContactView{
			RecoveryContactView:   *view,
			VerificationExpiresAt: state.PendingVerificationExpiresAt.UTC(),
			ResendAvailableAt:     state.PendingLastIssuedAt.UTC().Add(service.config.ResendCooldown),
			DeliveryState:         state.PendingDeliveryState,
		}
	}
	if !state.LastReauthenticatedAt.IsZero() {
		until := state.LastReauthenticatedAt.UTC().Add(service.config.ReauthenticationTTL)
		if until.After(now) {
			projection.ReauthenticatedUntil = &until
		}
	}
	projection.CanRemove = state.CanRemove && projection.Current != nil && projection.Pending == nil
	return projection, nil
}

func (service *RecoveryContactService) maskContact(
	contact entity.RecoveryContact,
) (*entity.RecoveryContactView, error) {
	if contact.ID == uuid.Nil || contact.UserID == uuid.Nil ||
		contact.Kind != recoveryContactKindEmail || contact.Version == 0 ||
		len(contact.AddressCiphertext) == 0 || contact.KeyID == "" {
		return nil, ErrInvalidRecoveryContactLifecycle
	}
	address, err := service.protector.Open(RecoveryContactMetadata{
		UserID: contact.UserID, ContactID: contact.ID, Kind: contact.Kind,
		Version: contact.Version,
	}, contact.KeyID, contact.AddressCiphertext)
	if err != nil {
		return nil, err
	}
	masked, err := MaskRecoveryEmail(address)
	if err != nil {
		return nil, err
	}
	return &entity.RecoveryContactView{
		Status: contact.Status, Purpose: contact.Purpose,
		MaskedAddress: masked, Version: contact.Version,
	}, nil
}

func (service *RecoveryContactService) Reauthenticate(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	password string,
) (time.Time, error) {
	if service == nil || ctx == nil || ctx.Err() != nil ||
		isNilTokenDependency(service.credentials) || isNilTokenDependency(service.sessions) ||
		userID == uuid.Nil || sessionID == uuid.Nil || password == "" || len(password) > 256 {
		return time.Time{}, recoveryContactInternal("reauthenticate recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	credential, err := service.credentials.GetByUserID(ctx, userID, credentialTypePassword)
	if err != nil {
		if applicationErrorCode(err) == apperr.CodeNotFound {
			return time.Time{}, recoveryContactReauthenticationRejected(err)
		}
		return time.Time{}, recoveryContactInternal("load reauthentication credential", err)
	}
	if credential == nil || credential.UserID != userID || credential.Type != credentialTypePassword ||
		credential.CredentialData == nil {
		return time.Time{}, recoveryContactInternal("load reauthentication credential", ErrInvalidRecoveryContactLifecycle)
	}
	hash, ok := credential.CredentialData[credentialKeyHash].(string)
	if !ok || hash == "" {
		return time.Time{}, recoveryContactInternal("load reauthentication credential", ErrInvalidRecoveryContactLifecycle)
	}
	if err := service.verifyPassword(ctx, hash, password); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			logger.FromContext(ctx).Debug("recovery contact reauthentication rejected", zap.String("user_id", userID.String()), zap.String("session_id", sessionID.String()))
			return time.Time{}, recoveryContactReauthenticationRejected(err)
		}
		return time.Time{}, recoveryContactInternal("verify reauthentication credential", err)
	}

	now := service.config.Clock().UTC()
	updated, err := service.sessions.MarkReauthenticated(ctx, sessionID, userID, now)
	if err != nil {
		return time.Time{}, recoveryContactInternal("persist reauthentication", err)
	}
	if !updated {
		return time.Time{}, apperr.New(apperr.CodeUnauthorized, constant.MsgUnauthorized, ports.ErrRecoveryContactRejected)
	}
	logger.FromContext(ctx).Info("recovery contact session reauthenticated", zap.String("user_id", userID.String()), zap.String("session_id", sessionID.String()))
	return now.Add(service.config.ReauthenticationTTL), nil
}

func (service *RecoveryContactService) Replace(
	ctx context.Context,
	userID, sessionID, idempotencyKey uuid.UUID,
	recoveryEmail string,
	expectedCurrentVersion uint64,
) (*entity.RecoveryContactProjection, error) {
	if err := service.validateMutationInput(ctx, userID, sessionID, idempotencyKey, expectedCurrentVersion); err != nil {
		return nil, err
	}
	if expectedCurrentVersion == math.MaxUint64 {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid recovery contact version", ports.ErrRecoveryContactVersionConflict)
	}
	prepared, err := service.prepareContactVerification(
		userID,
		recoveryEmail,
		entity.RecoveryContactPurposeReplacement,
		expectedCurrentVersion+1,
	)
	if err != nil {
		return nil, err
	}
	command, err := service.newMutationCommand(
		ports.RecoveryContactCommandReplace,
		userID,
		sessionID,
		idempotencyKey,
		expectedCurrentVersion,
		prepared.Issue.FingerprintHash,
	)
	if err != nil {
		return nil, err
	}
	replacement := ports.RecoveryContactReplacementCommand{
		Command: command, ExpectedCurrentVersion: expectedCurrentVersion,
		Issue:                  cloneRecoveryContactIssueForPersistence(prepared.Issue),
		ReplacementWindowStart: command.Now.Add(-24 * time.Hour),
		MaximumReplacements:    service.config.ReplacementDailyLimit,
	}
	applied := false
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var mutationErr error
		applied, mutationErr = service.repository.StartReplacement(transactionContext, replacement)
		if mutationErr != nil || !applied {
			return mutationErr
		}
		if err := service.scheduler.Enqueue(
			transactionContext,
			cloneSecurityEmailIntentForPersistence(prepared.Delivery),
		); err != nil {
			return err
		}
		contactID := prepared.Issue.Contact.ID
		sessionID := sessionID
		return service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: command.ReceiptID, Kind: entity.SecurityMutationRecoveryContactReplacementStarted,
			UserID: userID, SessionID: &sessionID, ContactID: &contactID,
			Version: prepared.Issue.Contact.Version, OccurredAt: command.Now,
		})
	})
	if err != nil {
		return nil, service.mapMutationError("replace recovery contact", err)
	}
	logger.FromContext(ctx).Info(
		"recovery contact replacement staged",
		zap.String("user_id", userID.String()),
		zap.String("session_id", sessionID.String()),
		zap.String("contact_id", prepared.Issue.Contact.ID.String()),
		zap.Bool("idempotent_replay", !applied),
	)
	return service.Get(ctx, userID, sessionID)
}

func (service *RecoveryContactService) Resend(
	ctx context.Context,
	userID, sessionID, idempotencyKey uuid.UUID,
	expectedPendingVersion uint64,
) (*entity.RecoveryContactProjection, error) {
	if err := service.validateMutationInput(ctx, userID, sessionID, idempotencyKey, expectedPendingVersion); err != nil {
		return nil, err
	}
	now := service.config.Clock().UTC()
	state, err := service.repository.GetState(ctx, userID, sessionID, now)
	if err != nil {
		return nil, service.mapMutationError("read pending recovery contact", err)
	}
	if state == nil || state.Pending == nil || state.Pending.Version != expectedPendingVersion {
		return nil, service.mapMutationError("read pending recovery contact", ports.ErrRecoveryContactVersionConflict)
	}
	pending := *state.Pending
	address, err := service.protector.Open(RecoveryContactMetadata{
		UserID: pending.UserID, ContactID: pending.ID, Kind: pending.Kind,
		Version: pending.Version,
	}, pending.KeyID, pending.AddressCiphertext)
	if err != nil {
		return nil, recoveryContactInternal("open pending recovery contact", err)
	}
	tokenID, token, delivery, err := service.prepareResendDelivery(pending, address, now)
	if err != nil {
		return nil, err
	}
	command, err := service.newMutationCommand(
		ports.RecoveryContactCommandResend,
		userID,
		sessionID,
		idempotencyKey,
		expectedPendingVersion,
		nil,
	)
	if err != nil {
		return nil, err
	}
	resend := ports.RecoveryContactResendCommand{
		Command: command, ContactID: pending.ID, ExpectedPendingVersion: expectedPendingVersion,
		TokenID: tokenID, Token: token, IssuedAt: now, ExpiresAt: delivery.PayloadExpiresAt,
		HourlyWindowStart: now.Add(-time.Hour), DailyWindowStart: now.Add(-24 * time.Hour),
		MaximumPerHour: service.config.VerificationHourlyLimit,
		MaximumPerDay:  service.config.VerificationDailyLimit,
		Cooldown:       service.config.ResendCooldown,
	}
	applied := false
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var mutationErr error
		applied, mutationErr = service.repository.ResendPending(transactionContext, resend)
		if mutationErr != nil || !applied {
			return mutationErr
		}
		if err := service.scheduler.Enqueue(transactionContext, cloneSecurityEmailIntentForPersistence(delivery)); err != nil {
			return err
		}
		contactID := pending.ID
		sessionID := sessionID
		return service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: command.ReceiptID, Kind: entity.SecurityMutationRecoveryContactVerificationResent,
			UserID: userID, SessionID: &sessionID, ContactID: &contactID,
			Version: expectedPendingVersion, OccurredAt: command.Now,
		})
	})
	if err != nil {
		return nil, service.mapMutationError("resend recovery verification", err)
	}
	logger.FromContext(ctx).Info(
		"recovery contact verification restaged",
		zap.String("user_id", userID.String()),
		zap.String("session_id", sessionID.String()),
		zap.String("contact_id", pending.ID.String()),
		zap.Bool("idempotent_replay", !applied),
	)
	return service.Get(ctx, userID, sessionID)
}

func (service *RecoveryContactService) PrepareInvitationResend(
	state ports.AccountInvitationResendState,
) (*ports.PreparedAccountInvitationResend, error) {
	contact := state.Contact
	if service == nil || service.protector == nil ||
		state.UserID == uuid.Nil || state.UserID != contact.UserID ||
		state.UserSecurityVersion == 0 || state.LastTokenID == uuid.Nil ||
		state.LastIssuedAt.IsZero() || state.ExpiresAt.IsZero() ||
		contact.ID == uuid.Nil || contact.Kind != recoveryContactKindEmail ||
		contact.Purpose != entity.RecoveryContactPurposeInvitation ||
		contact.Status != entity.RecoveryContactStatusPending ||
		contact.Version == 0 || contact.Version == math.MaxUint64 ||
		len(contact.AddressCiphertext) == 0 || contact.KeyID == "" {
		return nil, recoveryContactInternal(
			"prepare invitation resend",
			ErrInvalidRecoveryContactLifecycle,
		)
	}
	address, err := service.protector.Open(RecoveryContactMetadata{
		UserID: contact.UserID, ContactID: contact.ID,
		Kind: contact.Kind, Version: contact.Version,
	}, contact.KeyID, contact.AddressCiphertext)
	if err != nil {
		return nil, recoveryContactInternal("open invitation destination", err)
	}
	now := service.config.Clock().UTC()
	next := contact
	next.Version++
	next.UpdatedAt = now
	next.AddressCiphertext, next.KeyID, err = service.protector.Protect(
		RecoveryContactMetadata{
			UserID: next.UserID, ContactID: next.ID,
			Kind: next.Kind, Version: next.Version,
		},
		address,
	)
	if err != nil {
		return nil, recoveryContactInternal("rotate invitation destination protection", err)
	}
	tokenID, token, delivery, err := service.prepareResendDelivery(next, address, now)
	if err != nil {
		return nil, err
	}
	return &ports.PreparedAccountInvitationResend{
		Contact: next, TokenID: tokenID, Token: token,
		IssuedAt: now, ExpiresAt: delivery.PayloadExpiresAt,
		HourlyWindowStart: now.Add(-time.Hour),
		DailyWindowStart:  now.Add(-24 * time.Hour),
		MaximumPerHour:    service.config.VerificationHourlyLimit,
		MaximumPerDay:     service.config.VerificationDailyLimit,
		Cooldown:          service.config.ResendCooldown,
		Delivery:          delivery,
	}, nil
}

func (service *RecoveryContactService) CancelPending(
	ctx context.Context,
	userID, sessionID, idempotencyKey uuid.UUID,
	expectedPendingVersion uint64,
) (*entity.RecoveryContactProjection, error) {
	return service.transitionVersioned(
		ctx, ports.RecoveryContactCommandCancelPending,
		userID, sessionID, idempotencyKey, expectedPendingVersion,
		func(transactionContext context.Context, command ports.RecoveryContactVersionCommand) (bool, error) {
			return service.repository.CancelPending(transactionContext, command)
		},
	)
}

func (service *RecoveryContactService) Remove(
	ctx context.Context,
	userID, sessionID, idempotencyKey uuid.UUID,
	expectedCurrentVersion uint64,
) (*entity.RecoveryContactProjection, error) {
	return service.transitionVersioned(
		ctx, ports.RecoveryContactCommandRemove,
		userID, sessionID, idempotencyKey, expectedCurrentVersion,
		func(transactionContext context.Context, command ports.RecoveryContactVersionCommand) (bool, error) {
			return service.repository.RemoveCurrent(transactionContext, command)
		},
	)
}

func (service *RecoveryContactService) transitionVersioned(
	ctx context.Context,
	kind ports.RecoveryContactCommandKind,
	userID, sessionID, idempotencyKey uuid.UUID,
	expectedVersion uint64,
	transition func(context.Context, ports.RecoveryContactVersionCommand) (bool, error),
) (*entity.RecoveryContactProjection, error) {
	if err := service.validateMutationInput(ctx, userID, sessionID, idempotencyKey, expectedVersion); err != nil {
		return nil, err
	}
	if transition == nil {
		return nil, recoveryContactInternal("transition recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	command, err := service.newMutationCommand(kind, userID, sessionID, idempotencyKey, expectedVersion, nil)
	if err != nil {
		return nil, err
	}
	applied := false
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var mutationErr error
		applied, mutationErr = transition(transactionContext, ports.RecoveryContactVersionCommand{
			Command: command, ExpectedVersion: expectedVersion,
		})
		if mutationErr != nil || !applied {
			return mutationErr
		}
		mutationKind, mappingErr := recoveryTransitionMutationKind(kind)
		if mappingErr != nil {
			return mappingErr
		}
		actorSessionID := sessionID
		return service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: command.ReceiptID, Kind: mutationKind,
			UserID: userID, SessionID: &actorSessionID,
			Version: expectedVersion, OccurredAt: command.Now,
		})
	})
	if err != nil {
		return nil, service.mapMutationError("transition recovery contact", err)
	}
	logger.FromContext(ctx).Info(
		"recovery contact state changed",
		zap.String("user_id", userID.String()),
		zap.String("session_id", sessionID.String()),
		zap.String("operation", string(kind)),
		zap.Bool("idempotent_replay", !applied),
	)
	return service.Get(ctx, userID, sessionID)
}

func recoveryTransitionMutationKind(
	kind ports.RecoveryContactCommandKind,
) (entity.SecurityMutationKind, error) {
	switch kind {
	case ports.RecoveryContactCommandCancelPending:
		return entity.SecurityMutationRecoveryContactPendingCancelled, nil
	case ports.RecoveryContactCommandRemove:
		return entity.SecurityMutationRecoveryContactRemoved, nil
	default:
		return "", ErrInvalidRecoveryContactLifecycle
	}
}

func (service *RecoveryContactService) validateMutationInput(
	ctx context.Context,
	userID, sessionID, idempotencyKey uuid.UUID,
	expectedVersion uint64,
) error {
	if service == nil || ctx == nil || ctx.Err() != nil || service.repository == nil ||
		userID == uuid.Nil || sessionID == uuid.Nil ||
		idempotencyKey == uuid.Nil || expectedVersion == 0 {
		return recoveryContactInternal("validate recovery contact mutation", ErrInvalidRecoveryContactLifecycle)
	}
	return nil
}

func (service *RecoveryContactService) newMutationCommand(
	kind ports.RecoveryContactCommandKind,
	userID, sessionID, idempotencyKey uuid.UUID,
	expectedVersion uint64,
	extra []byte,
) (ports.RecoveryContactMutationCommand, error) {
	receiptID, err := uuid.NewV7()
	if err != nil {
		return ports.RecoveryContactMutationCommand{}, recoveryContactInternal("generate mutation receipt", err)
	}
	now := service.config.Clock().UTC()
	fingerprint := recoveryContactMutationFingerprint(kind, userID, sessionID, expectedVersion, extra)
	return ports.RecoveryContactMutationCommand{
		ReceiptID: receiptID, UserID: userID, SessionID: sessionID,
		IdempotencyKey: idempotencyKey, Kind: kind,
		RequestFingerprint: fingerprint, Now: now,
		RecentAfter:      now.Add(-service.config.ReauthenticationTTL),
		ReceiptExpiresAt: now.Add(service.config.CommandReceiptTTL),
	}, nil
}

func recoveryContactMutationFingerprint(
	kind ports.RecoveryContactCommandKind,
	userID, sessionID uuid.UUID,
	expectedVersion uint64,
	extra []byte,
) []byte {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("judgify/recovery-contact-command/v1\x00"))
	_, _ = hasher.Write([]byte(kind))
	_, _ = hasher.Write(userID[:])
	_, _ = hasher.Write(sessionID[:])
	var version [8]byte
	binary.BigEndian.PutUint64(version[:], expectedVersion)
	_, _ = hasher.Write(version[:])
	_, _ = hasher.Write(extra)
	return hasher.Sum(nil)
}

func (service *RecoveryContactService) prepareResendDelivery(
	contact entity.RecoveryContact,
	address string,
	now time.Time,
) (uuid.UUID, entity.KeyedTokenHash, ports.SecurityEmailDeliveryIntent, error) {
	if contact.Purpose != entity.RecoveryContactPurposeReplacement &&
		contact.Purpose != entity.RecoveryContactPurposeInvitation {
		return uuid.Nil, entity.KeyedTokenHash{}, ports.SecurityEmailDeliveryIntent{},
			recoveryContactInternal("prepare verification resend", ErrInvalidRecoveryContactLifecycle)
	}
	tokenID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, entity.KeyedTokenHash{}, ports.SecurityEmailDeliveryIntent{}, recoveryContactInternal("generate verification identifier", err)
	}
	deliveryID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, entity.KeyedTokenHash{}, ports.SecurityEmailDeliveryIntent{}, recoveryContactInternal("generate verification delivery identifier", err)
	}
	var rawToken string
	var token entity.KeyedTokenHash
	if contact.Purpose == entity.RecoveryContactPurposeInvitation {
		rawToken, token, err = service.protector.NewInvitationToken()
	} else {
		rawToken, token, err = service.protector.NewVerificationToken()
	}
	if err != nil {
		return uuid.Nil, entity.KeyedTokenHash{}, ports.SecurityEmailDeliveryIntent{}, recoveryContactInternal("generate verification credential", err)
	}
	expiresAt := now.Add(service.config.VerificationTTL)
	idempotencyKey := securityEmailProviderIdempotencyKey(deliveryID)
	deliveryKind := ports.SecurityEmailKindRecoveryContactVerification
	if contact.Purpose == entity.RecoveryContactPurposeInvitation {
		deliveryKind = ports.SecurityEmailKindAccountInvitation
	}
	metadata := SecurityEmailPayloadMetadata{
		DeliveryID: deliveryID, CredentialID: tokenID,
		Kind:                   deliveryKind,
		ProviderIdempotencyKey: idempotencyKey, ExpiresAt: expiresAt,
	}
	var ciphertext []byte
	var keyID string
	if contact.Purpose == entity.RecoveryContactPurposeInvitation {
		ciphertext, keyID, err = service.payloadCodec.SealAccountInvitation(
			metadata,
			ports.AccountInvitationDeliveryMessage{
				Recipient: address, Token: rawToken, ExpiresAt: expiresAt,
				ProviderIdempotencyKey: idempotencyKey,
			},
		)
	} else {
		ciphertext, keyID, err = service.payloadCodec.SealRecoveryContactVerification(
			metadata,
			ports.RecoveryContactVerificationDeliveryMessage{
				Recipient: address, Token: rawToken, ExpiresAt: expiresAt,
				ProviderIdempotencyKey: idempotencyKey,
			},
		)
	}
	if err != nil {
		return uuid.Nil, entity.KeyedTokenHash{}, ports.SecurityEmailDeliveryIntent{}, recoveryContactInternal("protect verification delivery", err)
	}
	return tokenID, token, ports.SecurityEmailDeliveryIntent{
		ID: deliveryID, Kind: deliveryKind,
		VerificationTokenID: tokenID, ProviderIdempotencyKey: idempotencyKey,
		PayloadCiphertext: ciphertext, KeyID: keyID,
		PayloadExpiresAt: expiresAt, AvailableAt: now,
	}, nil
}

func (service *RecoveryContactService) mapMutationError(operation string, cause error) error {
	switch {
	case errors.Is(cause, ports.ErrRecoveryContactRecentAuthenticationRequired):
		return apperr.New(apperr.CodeForbidden, "recent authentication required", cause)
	case errors.Is(cause, ports.ErrRecoveryContactVersionConflict):
		return apperr.New(apperr.CodeConflict, "recovery contact changed; reload and try again", cause)
	case errors.Is(cause, ports.ErrRecoveryContactIdempotencyConflict):
		return apperr.New(apperr.CodeConflict, "idempotency key conflict", cause)
	case errors.Is(cause, ports.ErrRecoveryContactRateLimited):
		return apperr.New(apperr.CodeTooManyRequests, "recovery contact request limit reached", cause)
	case errors.Is(cause, ports.ErrRecoveryContactRemovalRejected):
		return apperr.New(apperr.CodeConflict, "a password account must keep a verified recovery contact", cause)
	case errors.Is(cause, ports.ErrRecoveryContactRejected):
		return apperr.New(apperr.CodeUnauthorized, constant.MsgUnauthorized, cause)
	case transactionWasCommitted(cause):
		return cause
	default:
		return recoveryContactInternal(operation, cause)
	}
}

func recoveryContactReauthenticationRejected(cause error) error {
	return apperr.New(apperr.CodeUnauthorized, constant.MsgPassIncorrect, cause)
}

func applicationErrorCode(err error) int {
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		return applicationError.Code
	}
	return 0
}

func validRecoveryDeliveryState(state entity.RecoveryContactDeliveryState) bool {
	switch state {
	case entity.RecoveryContactDeliveryQueued,
		entity.RecoveryContactDeliverySent,
		entity.RecoveryContactDeliveryFailed:
		return true
	default:
		return false
	}
}

var _ ports.RecoveryContactManagement = (*RecoveryContactService)(nil)
