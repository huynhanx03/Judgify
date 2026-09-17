package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"
	commonpassword "github.com/huynhanx03/go-common/pkg/security/password"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	defaultRecoveryVerificationTTL         = 24 * time.Hour
	maximumRecoveryVerificationTTL         = 7 * 24 * time.Hour
	defaultRecoveryVerificationTokenSize   = 256
	defaultRecoveryReauthenticationTTL     = 10 * time.Minute
	maximumRecoveryReauthenticationTTL     = time.Hour
	defaultRecoveryResendCooldown          = time.Minute
	maximumRecoveryResendCooldown          = time.Hour
	defaultRecoveryCommandReceiptTTL       = 7 * 24 * time.Hour
	maximumRecoveryCommandReceiptTTL       = 30 * 24 * time.Hour
	defaultRecoveryReplacementDailyLimit   = 3
	defaultRecoveryVerificationHourlyLimit = 5
	defaultRecoveryVerificationDailyLimit  = 10
)

var ErrInvalidRecoveryContactLifecycle = errors.New("identity: invalid recovery contact lifecycle")

type RecoveryContactConfig struct {
	VerificationTTL         time.Duration
	ReauthenticationTTL     time.Duration
	ResendCooldown          time.Duration
	CommandReceiptTTL       time.Duration
	ReplacementDailyLimit   int
	VerificationHourlyLimit int
	VerificationDailyLimit  int
	MaximumTokenBytes       int
	Clock                   func() time.Time
}

// RecoveryContactService coordinates cryptography and protected persistence.
// Recipient/token plaintext exists only long enough to seal the delivery
// payload and never enters a repository command or log field.
type RecoveryContactService struct {
	repository     ports.RecoveryContactStore
	scheduler      ports.SecurityEmailScheduler
	payloadCodec   *SecurityEmailPayloadCodec
	protector      *RecoveryContactProtector
	tx             tx.Manager
	journal        ports.SecurityMutationJournal
	credentials    ports.CredentialRepository
	sessions       ports.RecoveryContactSessionRepository
	config         RecoveryContactConfig
	passwordHasher *commonpassword.Hasher
}

func NewRecoveryContactService(
	repository ports.RecoveryContactStore,
	scheduler ports.SecurityEmailScheduler,
	payloadCodec *SecurityEmailPayloadCodec,
	protector *RecoveryContactProtector,
	txManager tx.Manager,
	journal ports.SecurityMutationJournal,
	config RecoveryContactConfig,
) (*RecoveryContactService, error) {
	if config.VerificationTTL == 0 {
		config.VerificationTTL = defaultRecoveryVerificationTTL
	}
	if config.MaximumTokenBytes == 0 {
		config.MaximumTokenBytes = defaultRecoveryVerificationTokenSize
	}
	if config.ReauthenticationTTL == 0 {
		config.ReauthenticationTTL = defaultRecoveryReauthenticationTTL
	}
	if config.ResendCooldown == 0 {
		config.ResendCooldown = defaultRecoveryResendCooldown
	}
	if config.CommandReceiptTTL == 0 {
		config.CommandReceiptTTL = defaultRecoveryCommandReceiptTTL
	}
	if config.ReplacementDailyLimit == 0 {
		config.ReplacementDailyLimit = defaultRecoveryReplacementDailyLimit
	}
	if config.VerificationHourlyLimit == 0 {
		config.VerificationHourlyLimit = defaultRecoveryVerificationHourlyLimit
	}
	if config.VerificationDailyLimit == 0 {
		config.VerificationDailyLimit = defaultRecoveryVerificationDailyLimit
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if isNilTokenDependency(repository) || isNilTokenDependency(scheduler) ||
		payloadCodec == nil || protector == nil || isNilTokenDependency(txManager) ||
		isNilTokenDependency(journal) ||
		config.VerificationTTL <= 0 || config.VerificationTTL > maximumRecoveryVerificationTTL ||
		config.ReauthenticationTTL <= 0 || config.ReauthenticationTTL > maximumRecoveryReauthenticationTTL ||
		config.ResendCooldown <= 0 || config.ResendCooldown > maximumRecoveryResendCooldown ||
		config.CommandReceiptTTL <= 0 || config.CommandReceiptTTL > maximumRecoveryCommandReceiptTTL ||
		config.ReplacementDailyLimit < 1 || config.ReplacementDailyLimit > 100 ||
		config.VerificationHourlyLimit < 1 || config.VerificationHourlyLimit > 100 ||
		config.VerificationDailyLimit < config.VerificationHourlyLimit || config.VerificationDailyLimit > 1000 ||
		config.MaximumTokenBytes < 64 || config.MaximumTokenBytes > 4096 || config.Clock == nil {
		return nil, ErrInvalidRecoveryContactLifecycle
	}
	defaultHasher, _ := commonpassword.New(commonpassword.DefaultPolicy())
	service := &RecoveryContactService{
		repository: repository, scheduler: scheduler, payloadCodec: payloadCodec,
		protector: protector, tx: txManager, journal: journal, config: config,
		passwordHasher: defaultHasher,
	}
	return service, nil
}

func (service *RecoveryContactService) verifyPassword(ctx context.Context, hash, pw string) error {
	if service.passwordHasher == nil {
		return ErrInvalidRecoveryContactLifecycle
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

// PrepareRegistration performs every pure/cryptographic operation before the
// account transaction starts. The returned plan contains protected material
// only and can be persisted without hashing, randomness or provider I/O.
func (service *RecoveryContactService) PrepareRegistration(
	userID uuid.UUID,
	recoveryEmail string,
) (*ports.PreparedRecoveryContactRegistration, error) {
	return service.prepareContactVerification(
		userID,
		recoveryEmail,
		entity.RecoveryContactPurposeRegistration,
		1,
	)
}

// PrepareInvitation creates the same protected contact/token material as
// registration, but binds it to the invitation-only activation purpose and
// email template. The raw recipient and token still never cross a repository
// boundary.
func (service *RecoveryContactService) PrepareInvitation(
	userID uuid.UUID,
	recoveryEmail string,
) (*ports.PreparedRecoveryContactRegistration, error) {
	return service.prepareContactVerification(
		userID,
		recoveryEmail,
		entity.RecoveryContactPurposeInvitation,
		1,
	)
}

func (service *RecoveryContactService) prepareContactVerification(
	userID uuid.UUID,
	recoveryEmail string,
	purpose entity.RecoveryContactPurpose,
	version uint64,
) (*ports.PreparedRecoveryContactRegistration, error) {
	if service == nil || userID == uuid.Nil || version == 0 ||
		(purpose != entity.RecoveryContactPurposeRegistration &&
			purpose != entity.RecoveryContactPurposeReplacement &&
			purpose != entity.RecoveryContactPurposeInvitation) {
		return nil, recoveryContactInternal("prepare registration recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	normalized, err := NormalizeRecoveryEmail(recoveryEmail, defaultMaximumRecoveryEmailBytes)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, constant.MsgInvalidRecoveryEmail, err)
	}
	contactID, err := uuid.NewV7()
	if err != nil {
		return nil, recoveryContactInternal("generate recovery contact identifier", err)
	}
	tokenID, err := uuid.NewV7()
	if err != nil {
		return nil, recoveryContactInternal("generate verification identifier", err)
	}
	deliveryID, err := uuid.NewV7()
	if err != nil {
		return nil, recoveryContactInternal("generate verification delivery identifier", err)
	}
	metadata := RecoveryContactMetadata{
		UserID: userID, ContactID: contactID, Kind: recoveryContactKindEmail, Version: version,
	}
	addressCiphertext, contactKeyID, err := service.protector.Protect(metadata, normalized)
	if err != nil {
		return nil, recoveryContactInternal("protect recovery contact", err)
	}
	lookupHash, fingerprintHash, hashKeyID, err := service.protector.ActiveHashes(normalized)
	if err != nil {
		return nil, recoveryContactInternal("index recovery contact", err)
	}
	if hashKeyID != contactKeyID {
		return nil, recoveryContactInternal("protect recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	var rawToken string
	var tokenHash entity.KeyedTokenHash
	if purpose == entity.RecoveryContactPurposeInvitation {
		rawToken, tokenHash, err = service.protector.NewInvitationToken()
	} else {
		rawToken, tokenHash, err = service.protector.NewVerificationToken()
	}
	if err != nil {
		return nil, recoveryContactInternal("generate verification credential", err)
	}
	now := service.config.Clock().UTC()
	expiresAt := now.Add(service.config.VerificationTTL)
	providerIdempotencyKey := securityEmailProviderIdempotencyKey(deliveryID)
	deliveryKind := ports.SecurityEmailKindRecoveryContactVerification
	if purpose == entity.RecoveryContactPurposeInvitation {
		deliveryKind = ports.SecurityEmailKindAccountInvitation
	}
	payloadMetadata := SecurityEmailPayloadMetadata{
		DeliveryID: deliveryID, CredentialID: tokenID,
		Kind:                   deliveryKind,
		ProviderIdempotencyKey: providerIdempotencyKey,
		ExpiresAt:              expiresAt,
	}
	var payloadCiphertext []byte
	var payloadKeyID string
	if purpose == entity.RecoveryContactPurposeInvitation {
		payloadCiphertext, payloadKeyID, err = service.payloadCodec.SealAccountInvitation(
			payloadMetadata,
			ports.AccountInvitationDeliveryMessage{
				Recipient: normalized, Token: rawToken, ExpiresAt: expiresAt,
				ProviderIdempotencyKey: providerIdempotencyKey,
			},
		)
	} else {
		payloadCiphertext, payloadKeyID, err = service.payloadCodec.SealRecoveryContactVerification(
			payloadMetadata,
			ports.RecoveryContactVerificationDeliveryMessage{
				Recipient: normalized, Token: rawToken, ExpiresAt: expiresAt,
				ProviderIdempotencyKey: providerIdempotencyKey,
			},
		)
	}
	if err != nil {
		return nil, recoveryContactInternal("protect verification delivery", err)
	}
	issue := ports.RecoveryContactIssue{
		Contact: entity.RecoveryContact{
			ID: contactID, UserID: userID, Kind: recoveryContactKindEmail,
			Purpose: purpose,
			Status:  entity.RecoveryContactStatusPending, Version: version,
			AddressCiphertext: addressCiphertext, KeyID: contactKeyID,
			CreatedAt: now, UpdatedAt: now,
		},
		LookupHash: lookupHash, FingerprintHash: fingerprintHash,
		TokenID: tokenID, Token: tokenHash, IssuedAt: now, ExpiresAt: expiresAt,
	}
	prepared := &ports.PreparedRecoveryContactRegistration{
		Issue: issue,
		Delivery: ports.SecurityEmailDeliveryIntent{
			ID: deliveryID, Kind: deliveryKind,
			VerificationTokenID: tokenID, ProviderIdempotencyKey: providerIdempotencyKey,
			PayloadCiphertext: payloadCiphertext, KeyID: payloadKeyID,
			PayloadExpiresAt: expiresAt, AvailableAt: now,
		},
	}
	return prepared, nil
}

// PersistRegistration joins the caller's account transaction. Repositories
// independently require that physical transaction, preventing partial state if
// a caller bypasses the intended orchestration.
func (service *RecoveryContactService) PersistRegistration(
	ctx context.Context,
	prepared *ports.PreparedRecoveryContactRegistration,
) error {
	return service.persistPreparedContact(
		ctx,
		prepared,
		entity.RecoveryContactPurposeRegistration,
	)
}

func (service *RecoveryContactService) PersistInvitation(
	ctx context.Context,
	prepared *ports.PreparedRecoveryContactRegistration,
) error {
	return service.persistPreparedContact(
		ctx,
		prepared,
		entity.RecoveryContactPurposeInvitation,
	)
}

func (service *RecoveryContactService) persistPreparedContact(
	ctx context.Context,
	prepared *ports.PreparedRecoveryContactRegistration,
	purpose entity.RecoveryContactPurpose,
) error {
	if service == nil || ctx == nil || ctx.Err() != nil ||
		!validPreparedRecoveryContact(prepared, purpose) {
		return recoveryContactInternal("persist prepared recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	issue := cloneRecoveryContactIssueForPersistence(prepared.Issue)
	delivery := cloneSecurityEmailIntentForPersistence(prepared.Delivery)
	if err := service.repository.CreatePending(ctx, issue); err != nil {
		return recoveryContactInternal("persist prepared recovery contact", err)
	}
	if err := service.scheduler.Enqueue(ctx, delivery); err != nil {
		return recoveryContactInternal("persist verification delivery", err)
	}
	logger.FromContext(ctx).Debug(
		"recovery contact staged",
		zap.String("user_id", issue.Contact.UserID.String()),
		zap.String("contact_id", issue.Contact.ID.String()),
		zap.String("verification_id", issue.TokenID.String()),
		zap.String("purpose", string(purpose)),
	)
	return nil
}

func validPreparedRecoveryContact(
	prepared *ports.PreparedRecoveryContactRegistration,
	purpose entity.RecoveryContactPurpose,
) bool {
	if prepared == nil {
		return false
	}
	issue := prepared.Issue
	delivery := prepared.Delivery
	expectedKind := ports.SecurityEmailKindRecoveryContactVerification
	if purpose == entity.RecoveryContactPurposeInvitation {
		expectedKind = ports.SecurityEmailKindAccountInvitation
	}
	return (purpose == entity.RecoveryContactPurposeRegistration ||
		purpose == entity.RecoveryContactPurposeInvitation) &&
		issue.Contact.ID != uuid.Nil && issue.Contact.UserID != uuid.Nil &&
		issue.Contact.Kind == recoveryContactKindEmail &&
		issue.Contact.Purpose == purpose &&
		issue.Contact.Status == entity.RecoveryContactStatusPending && issue.Contact.Version == 1 &&
		len(issue.Contact.AddressCiphertext) > 1 && issue.Contact.KeyID != "" &&
		len(issue.LookupHash) == 32 && len(issue.FingerprintHash) == 32 &&
		issue.TokenID != uuid.Nil && issue.Token.KeyID != "" && len(issue.Token.Hash) == 32 &&
		!issue.IssuedAt.IsZero() && issue.ExpiresAt.After(issue.IssuedAt) &&
		delivery.ID != uuid.Nil && delivery.Kind == expectedKind &&
		delivery.VerificationTokenID == issue.TokenID && delivery.PasswordResetTokenID == uuid.Nil &&
		delivery.ProviderIdempotencyKey == securityEmailProviderIdempotencyKey(delivery.ID) &&
		len(delivery.PayloadCiphertext) > 1 && delivery.KeyID != "" &&
		delivery.PayloadExpiresAt.Equal(issue.ExpiresAt) && delivery.AvailableAt.Equal(issue.IssuedAt)
}

func cloneRecoveryContactIssueForPersistence(source ports.RecoveryContactIssue) ports.RecoveryContactIssue {
	result := source
	result.Contact.AddressCiphertext = append([]byte(nil), source.Contact.AddressCiphertext...)
	result.LookupHash = append([]byte(nil), source.LookupHash...)
	result.FingerprintHash = append([]byte(nil), source.FingerprintHash...)
	result.Token.Hash = append([]byte(nil), source.Token.Hash...)
	return result
}

func cloneSecurityEmailIntentForPersistence(
	source ports.SecurityEmailDeliveryIntent,
) ports.SecurityEmailDeliveryIntent {
	result := source
	result.PayloadCiphertext = append([]byte(nil), source.PayloadCiphertext...)
	return result
}

func (service *RecoveryContactService) Verify(
	ctx context.Context,
	rawToken string,
) (*entity.RecoveryContactVerificationResult, error) {
	if service == nil || ctx == nil || ctx.Err() != nil {
		return nil, recoveryContactInternal("verify recovery contact", ErrInvalidRecoveryContactLifecycle)
	}
	candidates, err := service.protector.VerificationTokenCandidates(
		rawToken,
		service.config.MaximumTokenBytes,
	)
	if err != nil {
		return nil, rejectedRecoveryContact(err)
	}
	now := service.config.Clock().UTC()
	mutationID, err := uuid.NewV7()
	if err != nil {
		return nil, recoveryContactInternal("generate verification journal identifier", err)
	}
	var result *entity.RecoveryContactVerificationResult
	err = service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var verifyErr error
		result, verifyErr = service.repository.VerifyIssued(transactionContext, candidates, now)
		if verifyErr != nil {
			return verifyErr
		}
		if result == nil || result.UserID == uuid.Nil || result.ContactID == uuid.Nil ||
			result.Version == 0 ||
			(result.Purpose != entity.RecoveryContactPurposeRegistration &&
				result.Purpose != entity.RecoveryContactPurposeReplacement) {
			return ErrInvalidRecoveryContactLifecycle
		}
		contactID := result.ContactID
		return service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: mutationID, Kind: entity.SecurityMutationRecoveryContactVerified,
			UserID: result.UserID, ContactID: &contactID, Version: result.Version,
			OccurredAt: now,
		})
	})
	if err != nil {
		if errors.Is(err, ports.ErrRecoveryContactRejected) {
			return nil, rejectedRecoveryContact(err)
		}
		if transactionWasCommitted(err) {
			return nil, err
		}
		return nil, recoveryContactInternal("consume recovery contact verification", err)
	}
	logger.FromContext(ctx).Info(
		"recovery contact verified",
		zap.String("user_id", result.UserID.String()),
		zap.String("contact_id", result.ContactID.String()),
		zap.String("purpose", string(result.Purpose)),
		zap.Uint64("version", result.Version),
	)
	copy := *result
	return &copy, nil
}

func rejectedRecoveryContact(cause error) error {
	return apperr.New(
		apperr.CodeBadRequest,
		constant.MsgInvalidRecoveryVerification,
		errors.Join(ports.ErrRecoveryContactRejected, cause),
	)
}

func recoveryContactInternal(operation string, cause error) error {
	return apperr.New(
		apperr.CodeInternalServer,
		"authentication service unavailable",
		fmt.Errorf("%s: %w", operation, cause),
	)
}

var _ ports.RecoveryContactLifecycle = (*RecoveryContactService)(nil)
var _ ports.AccountInvitationLifecycle = (*RecoveryContactService)(nil)
var _ ports.RecoveryContactApplication = (*RecoveryContactService)(nil)
