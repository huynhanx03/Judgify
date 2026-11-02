package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type recoveryContactMemoryRepository struct {
	mu          sync.Mutex
	issue       ports.RecoveryContactIssue
	createCalls int
	verifyCalls int
	verifyErr   error
}

func (repository *recoveryContactMemoryRepository) CreatePending(
	_ context.Context,
	issue ports.RecoveryContactIssue,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.createCalls++
	repository.issue = cloneRecoveryContactIssue(issue)
	return nil
}

func (repository *recoveryContactMemoryRepository) VerifyIssued(
	_ context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) (*entity.RecoveryContactVerificationResult, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.verifyCalls++
	if repository.verifyErr != nil {
		return nil, repository.verifyErr
	}
	if repository.issue.TokenID == uuid.Nil || !repository.issue.ExpiresAt.After(now) {
		return nil, ports.ErrRecoveryContactRejected
	}
	matched := false
	for _, candidate := range candidates {
		if candidate.KeyID == repository.issue.Token.KeyID &&
			hmac.Equal(candidate.Hash, repository.issue.Token.Hash) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, ports.ErrRecoveryContactRejected
	}
	return &entity.RecoveryContactVerificationResult{
		UserID:    repository.issue.Contact.UserID,
		ContactID: repository.issue.Contact.ID,
		Purpose:   repository.issue.Contact.Purpose,
		Version:   repository.issue.Contact.Version,
	}, nil
}

func (*recoveryContactMemoryRepository) GetState(
	context.Context,
	uuid.UUID,
	uuid.UUID,
	time.Time,
) (*ports.RecoveryContactState, error) {
	return nil, errRecoveryContactTest
}

func (*recoveryContactMemoryRepository) StartReplacement(
	context.Context,
	ports.RecoveryContactReplacementCommand,
) (bool, error) {
	return false, errRecoveryContactTest
}

func (*recoveryContactMemoryRepository) ResendPending(
	context.Context,
	ports.RecoveryContactResendCommand,
) (bool, error) {
	return false, errRecoveryContactTest
}

func (*recoveryContactMemoryRepository) CancelPending(
	context.Context,
	ports.RecoveryContactVersionCommand,
) (bool, error) {
	return false, errRecoveryContactTest
}

func (*recoveryContactMemoryRepository) RemoveCurrent(
	context.Context,
	ports.RecoveryContactVersionCommand,
) (bool, error) {
	return false, errRecoveryContactTest
}

func cloneRecoveryContactIssue(source ports.RecoveryContactIssue) ports.RecoveryContactIssue {
	result := source
	result.Contact.AddressCiphertext = append([]byte(nil), source.Contact.AddressCiphertext...)
	result.LookupHash = append([]byte(nil), source.LookupHash...)
	result.FingerprintHash = append([]byte(nil), source.FingerprintHash...)
	result.Token.Hash = append([]byte(nil), source.Token.Hash...)
	return result
}

func TestRecoveryContactRegistrationPersistsOnlyProtectedMaterialAndEncryptedDelivery(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repository := &recoveryContactMemoryRepository{}
	manager := &lifecycleDeferredTx{}
	scheduler := &passwordResetScheduleCapture{manager: manager}
	service, codec, protector := newRecoveryContactTestService(
		t, repository, scheduler, manager, now,
	)
	userID := uuid.New()

	prepared, err := service.PrepareRegistration(userID, " Recovery.User@Example.COM ")
	require.NoError(t, err)
	repository.mu.Lock()
	require.Zero(t, repository.createCalls)
	repository.mu.Unlock()
	scheduler.mu.Lock()
	require.Zero(t, scheduler.enqueueCalls)
	scheduler.mu.Unlock()
	require.NoError(t, manager.DoInTx(context.Background(), func(ctx context.Context) error {
		return service.PersistRegistration(ctx, prepared)
	}))

	repository.mu.Lock()
	issue := cloneRecoveryContactIssue(repository.issue)
	createCalls := repository.createCalls
	repository.mu.Unlock()
	require.Equal(t, 1, createCalls)
	require.Equal(t, userID, issue.Contact.UserID)
	require.Equal(t, entity.RecoveryContactPurposeRegistration, issue.Contact.Purpose)
	require.Equal(t, entity.RecoveryContactStatusPending, issue.Contact.Status)
	require.Equal(t, uint64(1), issue.Contact.Version)
	require.NotEqual(t, uuid.Nil, issue.Contact.ID)
	require.Len(t, issue.LookupHash, 32)
	require.Len(t, issue.FingerprintHash, 32)
	require.Len(t, issue.Token.Hash, 32)
	require.NotContains(t, string(issue.Contact.AddressCiphertext), "recovery.user@example.com")
	require.Equal(t, now.Add(24*time.Hour), issue.ExpiresAt)

	recipient, err := protector.Open(RecoveryContactMetadata{
		UserID: userID, ContactID: issue.Contact.ID, Kind: issue.Contact.Kind,
		Version: issue.Contact.Version,
	}, issue.Contact.KeyID, issue.Contact.AddressCiphertext)
	require.NoError(t, err)
	require.Equal(t, "recovery.user@example.com", recipient)

	scheduler.mu.Lock()
	require.Equal(t, 1, scheduler.enqueueCalls)
	require.Len(t, scheduler.intents, 1)
	delivery := scheduler.intents[0]
	scheduler.mu.Unlock()
	require.Equal(t, ports.SecurityEmailKindRecoveryContactVerification, delivery.Kind)
	require.Equal(t, issue.TokenID, delivery.VerificationTokenID)
	require.Equal(t, uuid.Nil, delivery.PasswordResetTokenID)
	require.NotContains(t, string(delivery.PayloadCiphertext), recipient)
	message, err := codec.OpenRecoveryContactVerification(SecurityEmailPayloadMetadata{
		DeliveryID: delivery.ID, CredentialID: delivery.VerificationTokenID, Kind: delivery.Kind,
		ProviderIdempotencyKey: delivery.ProviderIdempotencyKey,
		ExpiresAt:              delivery.PayloadExpiresAt,
	}, delivery.KeyID, delivery.PayloadCiphertext, now)
	require.NoError(t, err)
	require.Equal(t, recipient, message.Recipient)
	require.True(t, bytes.HasPrefix([]byte(message.Token), []byte(recoveryVerificationTokenPrefix)))
	require.NotEqual(t, message.Token, string(issue.Token.Hash))
}

func TestRecoveryContactVerificationUsesOneSafePublicFailure(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repository := &recoveryContactMemoryRepository{verifyErr: ports.ErrRecoveryContactRejected}
	manager := &lifecycleDeferredTx{}
	service, _, _ := newRecoveryContactTestService(
		t, repository, &passwordResetScheduleCapture{manager: manager}, manager, now,
	)

	for _, token := range []string{"malformed", "jcv1." + string(bytes.Repeat([]byte{'A'}, 43))} {
		_, err := service.Verify(context.Background(), token)
		var applicationError *apperr.AppError
		require.ErrorAs(t, err, &applicationError)
		require.Equal(t, apperr.CodeBadRequest, applicationError.Code)
		require.Equal(t, "invalid or expired verification link", applicationError.Message)
	}
}

func TestRecoveryContactVerificationConsumesTheIssuedTokenOnce(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repository := &recoveryContactMemoryRepository{}
	manager := &lifecycleDeferredTx{}
	service, _, _ := newRecoveryContactTestService(
		t, repository, &passwordResetScheduleCapture{manager: manager}, manager, now,
	)
	userID := uuid.New()
	var token string
	prepared, err := service.PrepareRegistration(userID, "owner@example.com")
	require.NoError(t, err)
	require.NoError(t, manager.DoInTx(context.Background(), func(ctx context.Context) error {
		return service.PersistRegistration(ctx, prepared)
	}))
	// Read the credential from the encrypted capture, never from repository state.
	scheduler := service.scheduler.(*passwordResetScheduleCapture)
	scheduler.mu.Lock()
	delivery := scheduler.intents[0]
	scheduler.mu.Unlock()
	message, err := service.payloadCodec.OpenRecoveryContactVerification(
		SecurityEmailPayloadMetadata{
			DeliveryID: delivery.ID, CredentialID: delivery.VerificationTokenID, Kind: delivery.Kind,
			ProviderIdempotencyKey: delivery.ProviderIdempotencyKey,
			ExpiresAt:              delivery.PayloadExpiresAt,
		}, delivery.KeyID, delivery.PayloadCiphertext, now,
	)
	require.NoError(t, err)
	token = message.Token

	result, err := service.Verify(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, userID, result.UserID)
	require.Equal(t, entity.RecoveryContactPurposeRegistration, result.Purpose)
	mutations := service.journal.(*securityMutationJournalCapture).snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationRecoveryContactVerified, mutations[0].Kind)
	require.Equal(t, result.ContactID, *mutations[0].ContactID)
	require.Equal(t, result.Version, mutations[0].Version)

	repository.verifyErr = ports.ErrRecoveryContactRejected
	_, err = service.Verify(context.Background(), token)
	require.Error(t, err)
}

func TestRecoveryContactVerificationFailsClosedWhenSecurityJournalFails(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repository := &recoveryContactMemoryRepository{}
	manager := &lifecycleDeferredTx{}
	service, _, _ := newRecoveryContactTestService(
		t, repository, &passwordResetScheduleCapture{manager: manager}, manager, now,
	)
	prepared, err := service.PrepareRegistration(uuid.New(), "owner@example.com")
	require.NoError(t, err)
	require.NoError(t, manager.DoInTx(context.Background(), func(ctx context.Context) error {
		return service.PersistRegistration(ctx, prepared)
	}))
	scheduler := service.scheduler.(*passwordResetScheduleCapture)
	scheduler.mu.Lock()
	delivery := scheduler.intents[0]
	scheduler.mu.Unlock()
	message, err := service.payloadCodec.OpenRecoveryContactVerification(
		SecurityEmailPayloadMetadata{
			DeliveryID: delivery.ID, CredentialID: delivery.VerificationTokenID, Kind: delivery.Kind,
			ProviderIdempotencyKey: delivery.ProviderIdempotencyKey,
			ExpiresAt:              delivery.PayloadExpiresAt,
		}, delivery.KeyID, delivery.PayloadCiphertext, now,
	)
	require.NoError(t, err)
	journalErr := errors.New("security journal unavailable")
	service.journal.(*securityMutationJournalCapture).err = journalErr

	result, err := service.Verify(context.Background(), message.Token)

	require.Nil(t, result)
	require.ErrorIs(t, err, journalErr)
	require.Empty(t, service.journal.(*securityMutationJournalCapture).snapshot())
}

func newRecoveryContactTestService(
	t *testing.T,
	repository ports.RecoveryContactStore,
	scheduler ports.SecurityEmailScheduler,
	manager *lifecycleDeferredTx,
	now time.Time,
) (*RecoveryContactService, *SecurityEmailPayloadCodec, *RecoveryContactProtector) {
	t.Helper()
	secret := bytes.Repeat([]byte{0x61}, 32)
	protector, err := NewRecoveryContactProtector(RecoveryContactProtectorConfig{
		ActiveKeyID: "recovery-test-v1",
		Keys:        map[string][]byte{"recovery-test-v1": secret},
		Random:      bytes.NewReader(bytes.Repeat([]byte{0x41}, 8192)),
	})
	require.NoError(t, err)
	codec := newSecurityEmailPayloadCodecForTest(t, "recovery-test-v1", map[string][]byte{
		"recovery-test-v1": secret,
	}, now, 0x51)
	result, err := NewRecoveryContactService(
		repository, scheduler, codec, protector, manager, &securityMutationJournalCapture{},
		RecoveryContactConfig{
			VerificationTTL:   24 * time.Hour,
			MaximumTokenBytes: 256,
			Clock:             func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	return result, codec, protector
}

var _ ports.RecoveryContactStore = (*recoveryContactMemoryRepository)(nil)

var errRecoveryContactTest = errors.New("recovery contact test error")
