package usecase

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	maximumSessionAbsoluteTTL = 90 * 24 * time.Hour
	maximumRecoveryTTL        = 30 * time.Minute
	minimumSecretBytes        = 32
	requestFingerprintBytes   = 32
)

var (
	ErrInvalidTokenLifecycle = errors.New("identity: invalid token lifecycle")
	ErrRefreshRejected       = errors.New("identity: refresh credential rejected")
	ErrRefreshReplay         = errors.New("identity: refresh credential replay")
	ErrIdempotencyConflict   = errors.New("identity: idempotency key reused with different input")
)

type TokenLifecycleConfig struct {
	AccessTTL              time.Duration
	RefreshTTL             time.Duration
	SessionIdleTTL         time.Duration
	SessionAbsoluteTTL     time.Duration
	RecoveryReceiptTTL     time.Duration
	Secret                 []byte
	SecretKeyID            string
	MaximumCredentialBytes int
	MaximumUserAgentBytes  int
	Clock                  func() time.Time
	Random                 io.Reader
}

type TokenLifecycleService struct {
	issuer        ports.AccessTokenIssuer
	tx            tx.Manager
	sessions      ports.SessionRepository
	refreshTokens ports.RefreshTokenRepository
	issuance      ports.SessionIssuanceReceiptRepository
	rotations     ports.RefreshRotationRepository
	authenticator ports.SessionAuthenticator
	protector     *tokenProtector
	config        TokenLifecycleConfig
}

// afterCommitRegistrar is an optional transaction-manager capability. Keeping
// the structural interface here preserves the application layer's dependency
// on the generic tx.Manager while the Ent adapter owns physical commit timing.
type afterCommitRegistrar interface {
	RegisterAfterCommit(context.Context, func(context.Context) error) error
}

type recoveryBundle struct {
	SessionID             string `json:"sid"`
	AccessToken           string `json:"access"`
	AccessTokenExpiresAt  int64  `json:"access_expires_at"`
	RefreshToken          string `json:"refresh"`
	RefreshTokenExpiresAt int64  `json:"refresh_expires_at"`
}

func NewTokenLifecycleService(
	issuer ports.AccessTokenIssuer,
	txManager tx.Manager,
	sessions ports.SessionRepository,
	refreshTokens ports.RefreshTokenRepository,
	issuance ports.SessionIssuanceReceiptRepository,
	rotations ports.RefreshRotationRepository,
	authenticator ports.SessionAuthenticator,
	config TokenLifecycleConfig,
) (*TokenLifecycleService, error) {
	config = normalizeTokenLifecycleConfig(config)
	if isNilTokenDependency(issuer) || isNilTokenDependency(txManager) ||
		isNilTokenDependency(sessions) || isNilTokenDependency(refreshTokens) ||
		isNilTokenDependency(issuance) || isNilTokenDependency(rotations) ||
		isNilTokenDependency(authenticator) || validateTokenLifecycleConfig(config) != nil {
		return nil, ErrInvalidTokenLifecycle
	}
	protector, err := newTokenProtector(config.Secret, config.Random)
	if err != nil {
		return nil, errors.Join(ErrInvalidTokenLifecycle, err)
	}
	config.Secret = nil
	return &TokenLifecycleService{
		issuer: issuer, tx: txManager, sessions: sessions,
		refreshTokens: refreshTokens, issuance: issuance,
		rotations: rotations, authenticator: authenticator,
		protector: protector, config: config,
	}, nil
}

func (service *TokenLifecycleService) Establish(
	ctx context.Context,
	command ports.SessionEstablishment,
) (entity.SessionCredentials, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || validateEstablishment(command, service.config) != nil {
		return entity.SessionCredentials{}, ErrInvalidTokenLifecycle
	}
	fingerprint := service.protector.fingerprint(command.RequestFingerprint)
	selector := issuanceSelector(
		command.Purpose,
		command.User.ID,
		command.PreSessionID,
		command.RequestOrTicketID,
		command.IdempotencyKey,
	)
	registrationRequiresProof := command.Purpose == entity.SessionIssuancePasswordRegistration
	if !registrationRequiresProof {
		if recovered, found, err := service.recoverIssuance(ctx, selector, fingerprint, nil); err != nil {
			return entity.SessionCredentials{}, err
		} else if found {
			return recovered, nil
		}
	}

	now := service.config.Clock().UTC()
	sessionID := uuid.Must(uuid.NewV7())
	refreshID := uuid.Must(uuid.NewV7())
	rawRefresh, refreshHash, err := service.protector.newRefreshToken()
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("generate refresh credential", err)
	}
	absoluteExpiresAt := now.Add(service.config.SessionAbsoluteTTL)
	idleExpiresAt := earliestTime(now.Add(service.config.SessionIdleTTL), absoluteExpiresAt)
	refreshExpiresAt := earliestTime(now.Add(service.config.RefreshTTL), absoluteExpiresAt)
	accessExpiresAt := earliestTime(now.Add(service.config.AccessTTL), idleExpiresAt)
	access, err := service.issuer.Issue(ctx, authentication.IssueRequest{
		Purpose: authentication.PurposeAccess,
		Subject: command.User.ID.String(), UserID: command.User.ID.String(),
		SessionID: sessionID.String(), Username: command.User.Username,
		TokenID:         uuid.Must(uuid.NewV7()).String(),
		AuthenticatedAt: now, SecurityVersion: command.User.SecurityVersion,
		IssuedAt: now, ExpiresAt: accessExpiresAt,
	})
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("issue access credential", err)
	}
	credentials := entity.SessionCredentials{
		SessionID: sessionID, AccessToken: access.Raw,
		AccessTokenExpiresAt: access.ExpiresAt.UTC(),
		RefreshToken:         rawRefresh, RefreshTokenExpiresAt: refreshExpiresAt.UTC(),
	}
	ciphertext, err := service.sealCredentials(credentials, issuanceAAD(
		command.Purpose,
		issuanceRequestIdentity(command.Purpose, command.PreSessionID, command.RequestOrTicketID),
		command.IdempotencyKey,
		sessionID,
		refreshID,
	))
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("protect issuance receipt", err)
	}
	cid := correlation.FromContext(ctx)
	if cid == "" {
		cid = correlation.New()
	}
	userAgent := strings.TrimSpace(command.UserAgent)
	session := &entity.Session{
		ID: sessionID, UserID: command.User.ID, FamilyID: sessionID,
		Status: entity.SessionStatusActive, UserSecurityVersion: command.User.SecurityVersion,
		CreatedAt: now, UpdatedAt: now, LastSeenAt: now, LastReauthenticatedAt: now,
		IdleExpiresAt: idleExpiresAt, AbsoluteExpiresAt: absoluteExpiresAt,
		IPHash: append([]byte(nil), command.IPHash...), CreatedCID: cid,
	}
	if userAgent != "" {
		session.UserAgent = &userAgent
	}
	refresh := &entity.RefreshToken{
		ID: refreshID, SessionID: sessionID, TokenHash: refreshHash,
		Status: entity.RefreshTokenStatusIssued, IssuedAt: now,
		ExpiresAt: refreshExpiresAt, CreatedAt: now, UpdatedAt: now,
	}
	receipt := newIssuanceReceipt(command, fingerprint, sessionID, refreshID, ciphertext, service.config, now)
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		if err := service.sessions.Create(transactionContext, session); err != nil {
			return err
		}
		if err := service.refreshTokens.Create(transactionContext, refresh); err != nil {
			return err
		}
		return service.issuance.Create(transactionContext, receipt)
	})
	if err != nil {
		// A concurrent retry may have committed the same logical command while
		// this transaction lost the unique-index race. Recover only the exact
		// fingerprint; otherwise preserve the original storage failure.
		if !registrationRequiresProof {
			if recovered, found, recoveryErr := service.recoverIssuance(ctx, selector, fingerprint, nil); recoveryErr == nil && found {
				return recovered, nil
			}
		}
		return entity.SessionCredentials{}, internalTokenError("persist authentication session", err)
	}
	logger.FromContext(ctx).Info(
		"authentication session established",
		zap.String("user_id", command.User.ID.String()),
		zap.String("session_id", sessionID.String()),
		zap.String("purpose", string(command.Purpose)),
	)
	return credentials, nil
}

func (service *TokenLifecycleService) RecoverRegistration(
	ctx context.Context,
	requestID uuid.UUID,
	idempotencyKey uuid.UUID,
	fingerprintMaterial []byte,
	verifier ports.RegistrationRecoveryVerifier,
) (entity.SessionCredentials, bool, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || requestID == uuid.Nil || idempotencyKey == uuid.Nil ||
		len(fingerprintMaterial) == 0 || verifier == nil {
		return entity.SessionCredentials{}, false, ErrInvalidTokenLifecycle
	}
	return service.recoverIssuance(
		ctx,
		issuanceSelector(
			entity.SessionIssuancePasswordRegistration,
			uuid.Nil,
			uuid.Nil,
			requestID,
			idempotencyKey,
		),
		service.protector.fingerprint(fingerprintMaterial),
		verifier,
	)
}

func (service *TokenLifecycleService) RecoverOAuth(
	ctx context.Context,
	ticketID uuid.UUID,
	idempotencyKey uuid.UUID,
	fingerprintMaterial []byte,
) (entity.SessionCredentials, bool, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || ticketID == uuid.Nil ||
		idempotencyKey == uuid.Nil || len(fingerprintMaterial) == 0 ||
		len(fingerprintMaterial) > 16<<10 {
		return entity.SessionCredentials{}, false, ErrInvalidTokenLifecycle
	}
	fingerprint := service.protector.fingerprint(fingerprintMaterial)
	for _, purpose := range []entity.SessionIssuancePurpose{
		entity.SessionIssuanceOAuthLogin,
		entity.SessionIssuanceOAuthRegistration,
	} {
		credentials, found, err := service.recoverIssuance(
			ctx,
			issuanceSelector(
				purpose,
				uuid.Nil,
				uuid.Nil,
				ticketID,
				idempotencyKey,
			),
			fingerprint,
			nil,
		)
		if err != nil || found {
			return credentials, found, err
		}
	}
	return entity.SessionCredentials{}, false, nil
}

func (service *TokenLifecycleService) Rotate(
	ctx context.Context,
	command ports.RefreshRotation,
) (entity.SessionCredentials, error) {
	if service == nil || ctx == nil || ctx.Err() != nil || command.IdempotencyKey == uuid.Nil {
		return entity.SessionCredentials{}, ErrRefreshRejected
	}
	now := service.config.Clock().UTC()
	presentedHash, err := service.protector.parseAndHashRefresh(
		command.RawRefreshToken,
		service.config.MaximumCredentialBytes,
	)
	if err != nil {
		logger.FromContext(ctx).Debug("refresh credential format rejected")
		return entity.SessionCredentials{}, unauthorizedRefresh(err)
	}
	refreshContext, err := service.rotations.ContextByHash(ctx, presentedHash, now)
	if err != nil {
		return entity.SessionCredentials{}, classifyRefreshStorageFailure(ctx, "refresh context lookup failed", err)
	}
	rawReplacement, replacementHash, err := service.protector.newRefreshToken()
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("generate replacement credential", err)
	}
	replacementID := uuid.Must(uuid.NewV7())
	accessExpiresAt := earliestTime(
		now.Add(service.config.AccessTTL),
		earliestTime(refreshContext.IdleExpiresAt, refreshContext.AbsoluteExpiresAt),
	)
	access, err := service.issuer.Issue(ctx, authentication.IssueRequest{
		Purpose: authentication.PurposeAccess,
		Subject: refreshContext.UserID.String(), UserID: refreshContext.UserID.String(),
		SessionID: refreshContext.SessionID.String(), Username: refreshContext.Username,
		TokenID:         uuid.Must(uuid.NewV7()).String(),
		AuthenticatedAt: refreshContext.AuthenticatedAt,
		SecurityVersion: refreshContext.SecurityVersion,
		IssuedAt:        now, ExpiresAt: accessExpiresAt,
	})
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("issue replacement access credential", err)
	}
	refreshExpiresAt := earliestTime(now.Add(service.config.RefreshTTL), refreshContext.AbsoluteExpiresAt)
	credentials := entity.SessionCredentials{
		SessionID: refreshContext.SessionID, AccessToken: access.Raw,
		AccessTokenExpiresAt: access.ExpiresAt.UTC(),
		RefreshToken:         rawReplacement, RefreshTokenExpiresAt: refreshExpiresAt,
	}
	receiptID := uuid.Must(uuid.NewV7())
	ciphertext, err := service.sealCredentials(
		credentials,
		refreshAAD(refreshContext.SessionID, refreshContext.TokenID, command.IdempotencyKey, replacementID),
	)
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("protect refresh receipt", err)
	}
	presentedID := refreshContext.TokenID
	mutation := ports.RefreshRotationMutation{
		PresentedHash: presentedHash,
		Expected:      refreshContext,
		Replacement: &entity.RefreshToken{
			ID: replacementID, SessionID: refreshContext.SessionID,
			TokenHash: replacementHash, ParentID: &presentedID,
			Status: entity.RefreshTokenStatusIssued, IssuedAt: now,
			ExpiresAt: refreshExpiresAt, CreatedAt: now, UpdatedAt: now,
		},
		Receipt: &entity.RefreshReceipt{
			ID: receiptID, SessionID: refreshContext.SessionID,
			PresentedTokenID: refreshContext.TokenID, IdempotencyKey: command.IdempotencyKey,
			ReplacementTokenID: replacementID, RecoveryCiphertext: ciphertext,
			KeyID:     service.config.SecretKeyID,
			ExpiresAt: now.Add(service.config.RecoveryReceiptTTL),
			CreatedAt: now, UpdatedAt: now,
		},
	}
	var rotation entity.RefreshRotationResult
	invalidationScheduled := false
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var rotateErr error
		rotation, rotateErr = service.rotations.Rotate(transactionContext, mutation, now)
		if rotateErr != nil {
			return rotateErr
		}
		if rotation.Outcome == entity.RefreshRotationRecovered {
			return service.rotations.MarkReceiptRecovered(transactionContext, rotation.ReceiptID, now)
		}
		if rotation.Outcome == entity.RefreshRotationReplayContained {
			invalidationScheduled, rotateErr = registerAfterCommit(
				service.tx,
				transactionContext,
				service.replayInvalidation(rotation.SessionID),
			)
			return rotateErr
		}
		return nil
	})
	if err != nil {
		return entity.SessionCredentials{}, classifyRefreshStorageFailure(ctx, "refresh rotation failed", err)
	}
	if rotation.Outcome == entity.RefreshRotationReplayContained {
		if !invalidationScheduled {
			if err := invokeAfterCommitFallback(ctx, service.replayInvalidation(rotation.SessionID)); err != nil {
				return entity.SessionCredentials{}, internalTokenError("invalidate replayed authentication session", err)
			}
		}
		return entity.SessionCredentials{}, unauthorizedRefresh(ErrRefreshReplay)
	}
	recovered, err := service.openCredentials(
		rotation.RecoveryCiphertext,
		refreshAAD(rotation.SessionID, refreshContext.TokenID, command.IdempotencyKey, rotation.ReplacementTokenID),
	)
	if err != nil {
		return entity.SessionCredentials{}, internalTokenError("recover refresh response", err)
	}
	if rotation.Outcome == entity.RefreshRotationRecovered {
		logger.FromContext(ctx).Info(
			"refresh response recovered",
			zap.String("session_id", rotation.SessionID.String()),
		)
	}
	return recovered, nil
}

func (service *TokenLifecycleService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	if service == nil || ctx == nil || ctx.Err() != nil || sessionID == uuid.Nil {
		return ErrInvalidTokenLifecycle
	}
	now := service.config.Clock().UTC()
	invalidationScheduled := false
	err := service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		// Every revocation path takes the session row before its credential rows.
		// Refresh rotation uses user -> session -> token, so this suffix order
		// cannot form a session/token deadlock with a concurrent rotation.
		if err := service.sessions.Revoke(transactionContext, sessionID, "logout", now); err != nil {
			return err
		}
		if err := service.refreshTokens.RevokeIssuedForSession(transactionContext, sessionID, now); err != nil {
			return err
		}
		var registerErr error
		invalidationScheduled, registerErr = registerAfterCommit(
			service.tx,
			transactionContext,
			service.sessionInvalidation(sessionID),
		)
		return registerErr
	})
	if err != nil {
		if transactionWasCommitted(err) {
			return err
		}
		return internalTokenError("revoke authentication session", err)
	}
	if !invalidationScheduled {
		if err := invokeAfterCommitFallback(ctx, service.sessionInvalidation(sessionID)); err != nil {
			return internalTokenError("invalidate authentication session", err)
		}
	}
	return nil
}

// RevokeAll is the atomic account-security fence used by password and account
// state changes. It revokes every live refresh credential and session before
// synchronously invalidating the process-local user index.
func (service *TokenLifecycleService) RevokeAll(ctx context.Context, userID uuid.UUID, reason string) error {
	if service == nil || ctx == nil || ctx.Err() != nil || userID == uuid.Nil ||
		!validBoundedText(reason, 256) || strings.TrimSpace(reason) != reason || reason == "" {
		return ErrInvalidTokenLifecycle
	}
	now := service.config.Clock().UTC()
	invalidationScheduled := false
	err := service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		if err := service.sessions.RevokeAll(transactionContext, userID, reason, now); err != nil {
			return err
		}
		if err := service.refreshTokens.RevokeIssuedForUser(transactionContext, userID, now); err != nil {
			return err
		}
		var registerErr error
		invalidationScheduled, registerErr = registerAfterCommit(
			service.tx,
			transactionContext,
			service.userSessionsInvalidation(userID, reason),
		)
		return registerErr
	})
	if err != nil {
		if transactionWasCommitted(err) {
			return err
		}
		return internalTokenError("revoke user authentication sessions", err)
	}
	if !invalidationScheduled {
		if err := invokeAfterCommitFallback(ctx, service.userSessionsInvalidation(userID, reason)); err != nil {
			return internalTokenError("invalidate user authentication sessions", err)
		}
	}
	return nil
}

func (service *TokenLifecycleService) sessionInvalidation(sessionID uuid.UUID) func(context.Context) error {
	return func(callbackContext context.Context) error {
		if err := service.authenticator.InvalidateSession(callbackContext, sessionID); err != nil {
			return err
		}
		logger.FromContext(callbackContext).Info(
			"authentication session revoked",
			zap.String("session_id", sessionID.String()),
		)
		return nil
	}
}

func (service *TokenLifecycleService) replayInvalidation(sessionID uuid.UUID) func(context.Context) error {
	return func(callbackContext context.Context) error {
		err := service.authenticator.InvalidateSession(callbackContext, sessionID)
		logger.FromContext(callbackContext).Warn(
			"refresh credential replay contained",
			zap.String("session_id", sessionID.String()),
		)
		return err
	}
}

func (service *TokenLifecycleService) userSessionsInvalidation(
	userID uuid.UUID,
	reason string,
) func(context.Context) error {
	return func(callbackContext context.Context) error {
		if err := service.authenticator.InvalidateUserSessions(callbackContext, userID); err != nil {
			return err
		}
		logger.FromContext(callbackContext).Info(
			"user authentication sessions revoked",
			zap.String("user_id", userID.String()),
			zap.String("reason", reason),
		)
		return nil
	}
}

func registerAfterCommit(
	manager tx.Manager,
	ctx context.Context,
	callback func(context.Context) error,
) (bool, error) {
	registrar, ok := manager.(afterCommitRegistrar)
	if !ok {
		return false, nil
	}
	return true, registrar.RegisterAfterCommit(ctx, callback)
}

func invokeAfterCommitFallback(
	ctx context.Context,
	callback func(context.Context) error,
) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrInvalidTokenLifecycle
		}
	}()
	return callback(context.WithoutCancel(ctx))
}

func transactionWasCommitted(err error) bool {
	var committed interface{ Committed() bool }
	return errors.As(err, &committed) && committed.Committed()
}

func (service *TokenLifecycleService) recoverIssuance(
	ctx context.Context,
	selector ports.IssuanceReceiptSelector,
	fingerprint []byte,
	verifier ports.RegistrationRecoveryVerifier,
) (entity.SessionCredentials, bool, error) {
	var credentials entity.SessionCredentials
	found := false
	now := service.config.Clock().UTC()
	err := service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		receipt, err := service.issuance.Get(transactionContext, selector)
		if err != nil {
			if isAppErrorCode(err, apperr.CodeNotFound) {
				return nil
			}
			return err
		}
		if receipt == nil || receipt.ErasedAt != nil || !receipt.ExpiresAt.After(now) ||
			receipt.KeyID != service.config.SecretKeyID || receipt.FingerprintKeyID == nil ||
			*receipt.FingerprintKeyID != service.config.SecretKeyID ||
			len(receipt.RequestFingerprint) != requestFingerprintBytes ||
			!hmac.Equal(receipt.RequestFingerprint, fingerprint) ||
			!validIssuanceOfferEvidence(receipt) {
			return ErrIdempotencyConflict
		}
		if receipt.Purpose == entity.SessionIssuancePasswordRegistration && verifier == nil {
			return ErrIdempotencyConflict
		}
		if verifier != nil {
			if receipt.Purpose != entity.SessionIssuancePasswordRegistration ||
				receipt.UserID == nil || *receipt.UserID == uuid.Nil {
				return ErrIdempotencyConflict
			}
			if err := verifier(transactionContext, *receipt.UserID); err != nil {
				return err
			}
		}
		credentials, err = service.openCredentials(
			receipt.RecoveryCiphertext,
			issuanceAAD(
				receipt.Purpose,
				receiptRequestIdentity(receipt),
				receipt.IdempotencyKey,
				receipt.SessionID,
				receipt.RefreshTokenID,
			),
		)
		if err != nil {
			return err
		}
		if err := service.issuance.MarkRecovered(transactionContext, receipt.ID, now); err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) {
			return entity.SessionCredentials{}, false, apperr.New(apperr.CodeConflict, "idempotency key conflict", err)
		}
		return entity.SessionCredentials{}, false, internalTokenError("recover session issuance", err)
	}
	return credentials, found, nil
}

func (service *TokenLifecycleService) sealCredentials(
	credentials entity.SessionCredentials,
	aad []byte,
) ([]byte, error) {
	bundle := recoveryBundle{
		SessionID: credentials.SessionID.String(), AccessToken: credentials.AccessToken,
		AccessTokenExpiresAt:  credentials.AccessTokenExpiresAt.Unix(),
		RefreshToken:          credentials.RefreshToken,
		RefreshTokenExpiresAt: credentials.RefreshTokenExpiresAt.Unix(),
	}
	plaintext, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	return service.protector.seal(plaintext, aad)
}

func (service *TokenLifecycleService) openCredentials(ciphertext, aad []byte) (entity.SessionCredentials, error) {
	plaintext, err := service.protector.open(ciphertext, aad)
	if err != nil {
		return entity.SessionCredentials{}, err
	}
	var bundle recoveryBundle
	if err := json.Unmarshal(plaintext, &bundle); err != nil {
		return entity.SessionCredentials{}, errInvalidProtectedCredential
	}
	sessionID, err := uuid.Parse(bundle.SessionID)
	if err != nil || sessionID == uuid.Nil || sessionID.String() != bundle.SessionID ||
		bundle.AccessToken == "" || bundle.RefreshToken == "" {
		return entity.SessionCredentials{}, errInvalidProtectedCredential
	}
	credentials := entity.SessionCredentials{
		SessionID: sessionID, AccessToken: bundle.AccessToken,
		AccessTokenExpiresAt:  time.Unix(bundle.AccessTokenExpiresAt, 0).UTC(),
		RefreshToken:          bundle.RefreshToken,
		RefreshTokenExpiresAt: time.Unix(bundle.RefreshTokenExpiresAt, 0).UTC(),
	}
	if credentials.AccessTokenExpiresAt.IsZero() || credentials.RefreshTokenExpiresAt.IsZero() ||
		credentials.RefreshTokenExpiresAt.Before(credentials.AccessTokenExpiresAt) {
		return entity.SessionCredentials{}, errInvalidProtectedCredential
	}
	return credentials, nil
}

func newIssuanceReceipt(
	command ports.SessionEstablishment,
	fingerprint []byte,
	sessionID, refreshID uuid.UUID,
	ciphertext []byte,
	config TokenLifecycleConfig,
	now time.Time,
) *entity.SessionIssuanceReceipt {
	keyID := config.SecretKeyID
	receipt := &entity.SessionIssuanceReceipt{
		ID: uuid.Must(uuid.NewV7()), Purpose: command.Purpose,
		RequestFingerprint: append([]byte(nil), fingerprint...), FingerprintKeyID: &keyID,
		IdempotencyKey: command.IdempotencyKey, SessionID: sessionID, RefreshTokenID: refreshID,
		RecoveryCiphertext: append([]byte(nil), ciphertext...), KeyID: config.SecretKeyID,
		ExpiresAt: now.Add(config.RecoveryReceiptTTL), CreatedAt: now, UpdatedAt: now,
	}
	if command.TraitOfferID != uuid.Nil {
		offerID := command.TraitOfferID
		receipt.TraitOfferID = &offerID
		receipt.OfferChecksum = append([]byte(nil), command.OfferChecksum...)
	}
	if command.Purpose == entity.SessionIssuancePasswordLogin {
		userID := command.User.ID
		preSessionID := command.PreSessionID
		receipt.UserID = &userID
		receipt.PreSessionID = &preSessionID
	} else {
		requestID := command.RequestOrTicketID
		receipt.RequestOrTicketID = &requestID
		userID := command.User.ID
		receipt.UserID = &userID
		preSessionID := command.PreSessionID
		receipt.PreSessionID = &preSessionID
	}
	return receipt
}

func issuanceSelector(
	purpose entity.SessionIssuancePurpose,
	userID, preSessionID, requestID, idempotencyKey uuid.UUID,
) ports.IssuanceReceiptSelector {
	selector := ports.IssuanceReceiptSelector{Purpose: purpose, IdempotencyKey: idempotencyKey}
	if purpose == entity.SessionIssuancePasswordLogin {
		selector.UserID = userID
		selector.PreSessionID = preSessionID
	} else {
		selector.RequestID = requestID
	}
	return selector
}

func issuanceAAD(
	purpose entity.SessionIssuancePurpose,
	requestID, idempotencyKey, sessionID, refreshID uuid.UUID,
) []byte {
	return []byte(fmt.Sprintf("issuance|%s|%s|%s|%s|%s", purpose, requestID, idempotencyKey, sessionID, refreshID))
}

func issuanceRequestIdentity(
	purpose entity.SessionIssuancePurpose,
	preSessionID, requestOrTicketID uuid.UUID,
) uuid.UUID {
	if purpose == entity.SessionIssuancePasswordLogin {
		return preSessionID
	}
	return requestOrTicketID
}

func receiptRequestIdentity(receipt *entity.SessionIssuanceReceipt) uuid.UUID {
	if receipt == nil {
		return uuid.Nil
	}
	if receipt.Purpose == entity.SessionIssuancePasswordLogin {
		if receipt.PreSessionID == nil {
			return uuid.Nil
		}
		return *receipt.PreSessionID
	}
	if receipt.RequestOrTicketID == nil {
		return uuid.Nil
	}
	return *receipt.RequestOrTicketID
}

func refreshAAD(sessionID, presentedID, idempotencyKey, replacementID uuid.UUID) []byte {
	return []byte(fmt.Sprintf("refresh|%s|%s|%s|%s", sessionID, presentedID, idempotencyKey, replacementID))
}

func validateEstablishment(command ports.SessionEstablishment, config TokenLifecycleConfig) error {
	registration := command.Purpose == entity.SessionIssuancePasswordRegistration ||
		command.Purpose == entity.SessionIssuanceOAuthRegistration
	if command.User == nil || command.User.ID == uuid.Nil || command.User.Username == "" ||
		command.User.SecurityVersion == 0 || command.User.SecurityStatus != entity.UserSecurityStatusActive ||
		command.PreSessionID == uuid.Nil || command.IdempotencyKey == uuid.Nil ||
		(command.Purpose != entity.SessionIssuancePasswordLogin && command.RequestOrTicketID == uuid.Nil) ||
		len(command.RequestFingerprint) == 0 ||
		len(command.RequestFingerprint) > 16<<10 || len(command.IPHash) != 0 && len(command.IPHash) != 32 ||
		(registration && (command.TraitOfferID == uuid.Nil || len(command.OfferChecksum) != 32)) ||
		(!registration && (command.TraitOfferID != uuid.Nil || len(command.OfferChecksum) != 0)) ||
		!validSessionIssuancePurpose(command.Purpose) ||
		!validBoundedText(command.UserAgent, config.MaximumUserAgentBytes) {
		return ErrInvalidTokenLifecycle
	}
	return nil
}

func validIssuanceOfferEvidence(receipt *entity.SessionIssuanceReceipt) bool {
	if receipt == nil {
		return false
	}
	registration := receipt.Purpose == entity.SessionIssuancePasswordRegistration ||
		receipt.Purpose == entity.SessionIssuanceOAuthRegistration
	hasOffer := receipt.TraitOfferID != nil && *receipt.TraitOfferID != uuid.Nil &&
		len(receipt.OfferChecksum) == 32
	return registration == hasOffer
}

func validSessionIssuancePurpose(purpose entity.SessionIssuancePurpose) bool {
	switch purpose {
	case entity.SessionIssuancePasswordLogin,
		entity.SessionIssuancePasswordRegistration,
		entity.SessionIssuanceOAuthLogin,
		entity.SessionIssuanceOAuthRegistration:
		return true
	default:
		return false
	}
}

func validBoundedText(value string, maximum int) bool {
	if !utf8.ValidString(value) || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.Is(unicode.Bidi_Control, character) {
			return false
		}
	}
	return true
}

func normalizeTokenLifecycleConfig(config TokenLifecycleConfig) TokenLifecycleConfig {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.Random == nil {
		config.Random = defaultCryptoRandom()
	}
	return config
}

func validateTokenLifecycleConfig(config TokenLifecycleConfig) error {
	if config.AccessTTL <= 0 || config.RefreshTTL <= config.AccessTTL ||
		config.SessionIdleTTL <= config.AccessTTL || config.SessionAbsoluteTTL < config.SessionIdleTTL ||
		config.SessionAbsoluteTTL > maximumSessionAbsoluteTTL ||
		config.RecoveryReceiptTTL <= 0 || config.RecoveryReceiptTTL > maximumRecoveryTTL ||
		config.RecoveryReceiptTTL >= config.AccessTTL ||
		len(config.Secret) < minimumSecretBytes || strings.Contains(string(config.Secret), "${") ||
		!validBoundedText(config.SecretKeyID, 128) || strings.TrimSpace(config.SecretKeyID) != config.SecretKeyID ||
		config.MaximumCredentialBytes < 256 || config.MaximumCredentialBytes > 64<<10 ||
		config.MaximumUserAgentBytes < 1 || config.MaximumUserAgentBytes > 512 ||
		config.Clock == nil || config.Random == nil {
		return ErrInvalidTokenLifecycle
	}
	return nil
}

func isNilTokenDependency(value any) bool {
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

func unauthorizedRefresh(cause error) error {
	return apperr.New(apperr.CodeUnauthorized, "invalid refresh credential", errors.Join(ErrRefreshRejected, cause))
}

func classifyRefreshStorageFailure(ctx context.Context, message string, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var appError *apperr.AppError
	if errors.As(err, &appError) &&
		(appError.Code == apperr.CodeDatabaseError || appError.Code == apperr.CodeInternalServer) {
		logger.FromContext(ctx).Warn(message, zap.Error(err))
		return internalTokenError("refresh storage unavailable", err)
	}
	logger.FromContext(ctx).Debug(message)
	return unauthorizedRefresh(err)
}

func internalTokenError(operation string, err error) error {
	return apperr.New(apperr.CodeInternalServer, "authentication service unavailable", fmt.Errorf("%s: %w", operation, err))
}

var _ ports.TokenLifecycle = (*TokenLifecycleService)(nil)
