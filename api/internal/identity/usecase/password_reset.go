package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	commonpassword "github.com/huynhanx03/go-common/pkg/security/password"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	passwordResetTokenPrefix        = "jpr1."
	passwordResetEntropyBytes       = 32
	maximumPasswordResetTTL         = time.Hour
	defaultPasswordResetBytes       = 256
	maximumResetPasswordBytes       = 256
	defaultPasswordResetShortWindow = 15 * time.Minute
	maximumPasswordResetShortWindow = time.Hour
	defaultPasswordResetShortLimit  = 3
	defaultPasswordResetDailyLimit  = 10
	passwordResetDecoyRecipient     = "decoy@invalid.invalid"
)

var ErrInvalidPasswordReset = errors.New("identity: invalid password reset lifecycle")

type PasswordResetConfig struct {
	TTL                   time.Duration
	MaximumBytes          int
	ShortWindow           time.Duration
	MaximumPerShortWindow int
	MaximumPerDay         int
	Clock                 func() time.Time
}

type PasswordResetService struct {
	repository     ports.PasswordResetRepository
	credentials    ports.CredentialRepository
	tokenLifecycle ports.TokenLifecycle
	scheduler      ports.SecurityEmailScheduler
	payloadCodec   *SecurityEmailPayloadCodec
	protector      *RecoveryContactProtector
	tx             tx.Manager
	journal        ports.SecurityMutationJournal
	config         PasswordResetConfig
	passwordHasher *commonpassword.Hasher
}

type preparedPasswordResetIssue struct {
	intentID               uuid.UUID
	deliveryID             uuid.UUID
	tokenHash              entity.KeyedTokenHash
	issuedAt               time.Time
	expiresAt              time.Time
	providerIdempotencyKey string
	payloadCiphertext      []byte
	payloadKeyID           string
}

func NewPasswordResetService(
	repository ports.PasswordResetRepository,
	credentials ports.CredentialRepository,
	tokenLifecycle ports.TokenLifecycle,
	scheduler ports.SecurityEmailScheduler,
	payloadCodec *SecurityEmailPayloadCodec,
	protector *RecoveryContactProtector,
	txManager tx.Manager,
	journal ports.SecurityMutationJournal,
	config PasswordResetConfig,
) (*PasswordResetService, error) {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.MaximumBytes == 0 {
		config.MaximumBytes = defaultPasswordResetBytes
	}
	if config.ShortWindow == 0 {
		config.ShortWindow = defaultPasswordResetShortWindow
	}
	if config.MaximumPerShortWindow == 0 {
		config.MaximumPerShortWindow = defaultPasswordResetShortLimit
	}
	if config.MaximumPerDay == 0 {
		config.MaximumPerDay = defaultPasswordResetDailyLimit
	}
	if isNilTokenDependency(repository) || isNilTokenDependency(credentials) ||
		isNilTokenDependency(tokenLifecycle) || isNilTokenDependency(scheduler) || payloadCodec == nil ||
		protector == nil || isNilTokenDependency(txManager) || isNilTokenDependency(journal) ||
		config.TTL <= 0 ||
		config.TTL > maximumPasswordResetTTL || config.MaximumBytes < 64 ||
		config.MaximumBytes > 4096 || config.ShortWindow <= 0 ||
		config.ShortWindow > maximumPasswordResetShortWindow ||
		config.MaximumPerShortWindow < 1 || config.MaximumPerShortWindow > 100 ||
		config.MaximumPerDay < config.MaximumPerShortWindow || config.MaximumPerDay > 1000 ||
		config.Clock == nil {
		return nil, ErrInvalidPasswordReset
	}
	defaultHasher, _ := commonpassword.New(commonpassword.DefaultPolicy())
	return &PasswordResetService{
		repository: repository, credentials: credentials,
		tokenLifecycle: tokenLifecycle, scheduler: scheduler, payloadCodec: payloadCodec,
		protector: protector, tx: txManager, journal: journal, config: config,
		passwordHasher: defaultHasher,
	}, nil
}

func (service *PasswordResetService) hashPassword(ctx context.Context, pw string) (string, error) {
	if service.passwordHasher == nil {
		return "", ErrInvalidPasswordReset
	}
	return service.passwordHasher.Hash(ctx, []byte(pw))
}

func (service *PasswordResetService) Issue(ctx context.Context, user *entity.User) error {
	if service == nil || ctx == nil || ctx.Err() != nil || user == nil || user.ID == uuid.Nil ||
		user.SecurityStatus != entity.UserSecurityStatusActive || user.SecurityVersion == 0 {
		return ErrInvalidPasswordReset
	}
	contact, err := service.repository.EligibleContact(ctx, user.ID, user.SecurityVersion)
	if err != nil {
		if obscureErr := service.Obscure(ctx); obscureErr != nil {
			return passwordResetInternal("obscure reset eligibility", errors.Join(err, obscureErr))
		}
		if errors.Is(err, ports.ErrPasswordResetRejected) {
			return rejectedPasswordReset(err)
		}
		return passwordResetInternal("resolve verified recovery contact", err)
	}
	if !validPasswordResetContact(contact, user.ID) {
		if obscureErr := service.Obscure(ctx); obscureErr != nil {
			return passwordResetInternal("obscure invalid reset contact", obscureErr)
		}
		return rejectedPasswordReset(ports.ErrPasswordResetRejected)
	}
	recipient, err := service.protector.Open(RecoveryContactMetadata{
		UserID: contact.UserID, ContactID: contact.ID, Kind: contact.Kind, Version: contact.Version,
	}, contact.KeyID, contact.AddressCiphertext)
	if err != nil {
		return passwordResetInternal("open verified recovery contact", err)
	}
	prepared, err := service.prepareIssue(recipient)
	if err != nil {
		return passwordResetInternal("prepare password reset intent", err)
	}
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		if err := service.repository.Issue(transactionContext, ports.PasswordResetIssue{
			ID: prepared.intentID, UserID: user.ID, ContactID: contact.ID,
			ContactVersion: contact.Version, UserSecurityVersion: user.SecurityVersion,
			Token: prepared.tokenHash, IssuedAt: prepared.issuedAt, ExpiresAt: prepared.expiresAt,
			ShortWindowStart:      prepared.issuedAt.Add(-service.config.ShortWindow),
			DailyWindowStart:      prepared.issuedAt.Add(-24 * time.Hour),
			MaximumPerShortWindow: service.config.MaximumPerShortWindow,
			MaximumPerDay:         service.config.MaximumPerDay,
		}); err != nil {
			return err
		}
		return service.scheduler.Enqueue(transactionContext, ports.SecurityEmailDeliveryIntent{
			ID: prepared.deliveryID, Kind: ports.SecurityEmailKindPasswordReset,
			PasswordResetTokenID:   prepared.intentID,
			ProviderIdempotencyKey: prepared.providerIdempotencyKey,
			PayloadCiphertext:      prepared.payloadCiphertext, KeyID: prepared.payloadKeyID,
			PayloadExpiresAt: prepared.expiresAt, AvailableAt: prepared.issuedAt,
		})
	})
	if err != nil {
		if transactionWasCommitted(err) {
			return err
		}
		if errors.Is(err, ports.ErrPasswordResetRateLimited) {
			return apperr.New(
				apperr.CodeTooManyRequests,
				"password reset request limit reached",
				err,
			)
		}
		return passwordResetInternal("persist password reset intent", err)
	}
	return nil
}

// Obscure executes the bounded random-token and AEAD work of an eligible
// request, then discards the result. It performs no database write and never
// schedules the decoy recipient.
func (service *PasswordResetService) Obscure(ctx context.Context) error {
	if service == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidPasswordReset
	}
	prepared, err := service.prepareIssue(passwordResetDecoyRecipient)
	if prepared != nil {
		clear(prepared.payloadCiphertext)
	}
	return err
}

func (service *PasswordResetService) prepareIssue(
	recipient string,
) (*preparedPasswordResetIssue, error) {
	rawToken, tokenHash, err := service.protector.NewPasswordResetToken()
	if err != nil {
		return nil, fmt.Errorf("generate reset credential: %w", err)
	}
	issuedAt := service.config.Clock().UTC()
	expiresAt := issuedAt.Add(service.config.TTL)
	intentID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate reset intent identifier: %w", err)
	}
	deliveryID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate delivery intent identifier: %w", err)
	}
	providerIdempotencyKey := securityEmailProviderIdempotencyKey(deliveryID)
	payloadCiphertext, payloadKeyID, err := service.payloadCodec.SealPasswordReset(
		SecurityEmailPayloadMetadata{
			DeliveryID: deliveryID, CredentialID: intentID, Kind: ports.SecurityEmailKindPasswordReset,
			ProviderIdempotencyKey: providerIdempotencyKey, ExpiresAt: expiresAt,
		},
		ports.PasswordResetDeliveryMessage{
			Recipient: recipient, Token: rawToken, ExpiresAt: expiresAt,
			ProviderIdempotencyKey: providerIdempotencyKey,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("protect password reset delivery: %w", err)
	}
	return &preparedPasswordResetIssue{
		intentID: intentID, deliveryID: deliveryID, tokenHash: tokenHash,
		issuedAt: issuedAt, expiresAt: expiresAt,
		providerIdempotencyKey: providerIdempotencyKey,
		payloadCiphertext:      payloadCiphertext, payloadKeyID: payloadKeyID,
	}, nil
}

func (service *PasswordResetService) Consume(
	ctx context.Context,
	rawToken string,
	newPassword string,
) error {
	if service == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidPasswordReset
	}
	if len(newPassword) < 8 || len(newPassword) > maximumResetPasswordBytes ||
		!utf8.ValidString(newPassword) {
		return apperr.New(apperr.CodeBadRequest, "invalid password", ErrInvalidPasswordReset)
	}
	candidates, err := service.protector.PasswordResetTokenCandidates(rawToken, service.config.MaximumBytes)
	if err != nil {
		return rejectedPasswordReset(err)
	}
	now := service.config.Clock().UTC()
	if err := service.repository.ValidateIssued(ctx, candidates, now); err != nil {
		if errors.Is(err, ports.ErrPasswordResetRejected) {
			return rejectedPasswordReset(err)
		}
		return passwordResetInternal("validate password reset intent", err)
	}
	newHash, err := service.hashPassword(ctx, newPassword)
	if err != nil {
		return passwordResetInternal("hash replacement password", err)
	}
	mutationID, err := uuid.NewV7()
	if err != nil {
		return passwordResetInternal("generate password reset journal identifier", err)
	}
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		intent, err := service.repository.LockIssued(transactionContext, candidates, now)
		if err != nil {
			return err
		}
		credential, err := service.credentials.GetByUserID(
			transactionContext,
			intent.UserID,
			credentialTypePassword,
		)
		if err != nil {
			if isAppErrorCode(err, apperr.CodeNotFound) {
				return ports.ErrPasswordResetRejected
			}
			return err
		}
		if credential == nil || credential.UserID != intent.UserID ||
			credential.Type != credentialTypePassword || credential.CredentialData == nil {
			return ports.ErrPasswordResetRejected
		}
		credential.CredentialData[credentialKeyHash] = newHash
		if err := service.credentials.Update(transactionContext, credential); err != nil {
			return err
		}
		if err := service.repository.Consume(transactionContext, intent, now); err != nil {
			return err
		}
		if err := service.tokenLifecycle.RevokeAll(transactionContext, intent.UserID, "password_reset"); err != nil {
			return err
		}
		contactID := intent.ContactID
		intentID := intent.ID
		return service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: mutationID, Kind: entity.SecurityMutationPasswordResetConsumed,
			UserID: intent.UserID, ContactID: &contactID, IntentID: &intentID,
			Version: intent.ContactVersion, SecurityVersion: intent.UserSecurityVersion + 1,
			OccurredAt: now,
		})
	})
	if err != nil {
		if errors.Is(err, ports.ErrPasswordResetRejected) {
			return rejectedPasswordReset(err)
		}
		if transactionWasCommitted(err) {
			return err
		}
		return passwordResetInternal("consume password reset", err)
	}
	return nil
}

func validPasswordResetContact(contact *entity.RecoveryContact, userID uuid.UUID) bool {
	return contact != nil && contact.ID != uuid.Nil && contact.UserID == userID &&
		contact.Kind == recoveryContactKindEmail &&
		contact.Status == entity.RecoveryContactStatusVerified && contact.Version > 0 &&
		contact.KeyID != "" && len(contact.AddressCiphertext) > 0
}

func rejectedPasswordReset(cause error) error {
	return apperr.New(
		apperr.CodeBadRequest,
		constant.MsgInvalidResetToken,
		errors.Join(ports.ErrPasswordResetRejected, cause),
	)
}

func passwordResetInternal(operation string, cause error) error {
	return apperr.New(
		apperr.CodeInternalServer,
		"authentication service unavailable",
		fmt.Errorf("%s: %w", operation, cause),
	)
}

var _ ports.PasswordResetLifecycle = (*PasswordResetService)(nil)
