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
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	passwordsecurity "github.com/huynhanx03/go-common/pkg/security/password"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	idmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

type passwordResetMemoryRepository struct {
	mu       sync.Mutex
	issue    ports.PasswordResetIssue
	issueErr error
	contact  *entity.RecoveryContact
	consumed bool
}

func (repository *passwordResetMemoryRepository) EligibleContact(
	_ context.Context,
	userID uuid.UUID,
	securityVersion uint64,
) (*entity.RecoveryContact, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.contact == nil || repository.contact.UserID != userID ||
		securityVersion == 0 || repository.contact.Status != entity.RecoveryContactStatusVerified {
		return nil, ports.ErrPasswordResetRejected
	}
	copy := *repository.contact
	copy.AddressCiphertext = append([]byte(nil), repository.contact.AddressCiphertext...)
	return &copy, nil
}

func (repository *passwordResetMemoryRepository) Issue(
	_ context.Context,
	issue ports.PasswordResetIssue,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.issueErr != nil {
		return repository.issueErr
	}
	repository.issue = issue
	repository.consumed = false
	return nil
}

func TestPasswordResetIssuePreservesPersistedQuotaClassification(t *testing.T) {
	controller := gomock.NewController(t)
	repository := &passwordResetMemoryRepository{issueErr: ports.ErrPasswordResetRateLimited}
	service, _ := newPasswordResetTestService(
		t,
		repository,
		idmocks.NewMockCredentialRepository(controller),
		&passwordResetTokenLifecycleSpy{},
		&passwordResetScheduleCapture{},
		lifecycleTestTx{},
	)

	err := service.Issue(context.Background(), passwordResetTestUser())
	require.ErrorIs(t, err, ports.ErrPasswordResetRateLimited)
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, apperr.CodeTooManyRequests, applicationError.Code)
}

func (repository *passwordResetMemoryRepository) LockIssued(
	_ context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) (*entity.PasswordResetIntent, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.consumed || repository.issue.ID == uuid.Nil ||
		!matchesPasswordResetCandidate(repository.issue.Token, candidates) ||
		!repository.issue.ExpiresAt.After(now) {
		return nil, ports.ErrPasswordResetRejected
	}
	return &entity.PasswordResetIntent{
		ID: repository.issue.ID, UserID: repository.issue.UserID,
		ContactID: repository.issue.ContactID, ContactVersion: repository.issue.ContactVersion,
		UserSecurityVersion: repository.issue.UserSecurityVersion,
		ExpiresAt:           repository.issue.ExpiresAt,
	}, nil
}

func (repository *passwordResetMemoryRepository) ValidateIssued(
	_ context.Context,
	candidates []entity.KeyedTokenHash,
	now time.Time,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.consumed || repository.issue.ID == uuid.Nil ||
		!matchesPasswordResetCandidate(repository.issue.Token, candidates) ||
		!repository.issue.ExpiresAt.After(now) {
		return ports.ErrPasswordResetRejected
	}
	return nil
}

func matchesPasswordResetCandidate(
	want entity.KeyedTokenHash,
	candidates []entity.KeyedTokenHash,
) bool {
	for _, candidate := range candidates {
		if candidate.KeyID == want.KeyID && hmac.Equal(candidate.Hash, want.Hash) {
			return true
		}
	}
	return false
}

func (repository *passwordResetMemoryRepository) Consume(
	_ context.Context,
	intent *entity.PasswordResetIntent,
	_ time.Time,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.consumed || intent == nil || intent.ID != repository.issue.ID {
		return ports.ErrPasswordResetRejected
	}
	repository.consumed = true
	return nil
}

type passwordResetScheduleCapture struct {
	mu           sync.Mutex
	intents      []ports.SecurityEmailDeliveryIntent
	enqueueCalls int
	err          error
	manager      *lifecycleDeferredTx
}

func (capture *passwordResetScheduleCapture) Enqueue(
	ctx context.Context,
	intent ports.SecurityEmailDeliveryIntent,
) error {
	capture.mu.Lock()
	capture.enqueueCalls++
	err := capture.err
	capture.mu.Unlock()
	if err != nil {
		return err
	}
	commit := func() {
		capture.mu.Lock()
		capture.intents = append(capture.intents, intent)
		capture.mu.Unlock()
	}
	if capture.manager == nil {
		commit()
		return nil
	}
	return capture.manager.RegisterAfterCommit(ctx, func(context.Context) error {
		commit()
		return nil
	})
}

type passwordResetTokenLifecycleSpy struct {
	ports.TokenLifecycle
	mu     sync.Mutex
	userID uuid.UUID
	reason string
}

func (spy *passwordResetTokenLifecycleSpy) RevokeAll(
	_ context.Context,
	userID uuid.UUID,
	reason string,
) error {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	spy.userID = userID
	spy.reason = reason
	return nil
}

func TestPasswordResetIssuePersistsOnlyProtectedMaterialAndSchedulesEncryptedDeliveryInTransaction(t *testing.T) {
	controller := gomock.NewController(t)
	credentials := idmocks.NewMockCredentialRepository(controller)
	repository := &passwordResetMemoryRepository{}
	lifecycle := &passwordResetTokenLifecycleSpy{}
	manager := &lifecycleDeferredTx{}
	scheduler := &passwordResetScheduleCapture{manager: manager}
	service, codec := newPasswordResetTestService(t, repository, credentials, lifecycle, scheduler, manager)
	user := passwordResetTestUser()

	require.NoError(t, service.Issue(context.Background(), user))

	scheduler.mu.Lock()
	require.Equal(t, 1, scheduler.enqueueCalls)
	require.Len(t, scheduler.intents, 1)
	intent := scheduler.intents[0]
	scheduler.mu.Unlock()
	require.Equal(t, ports.SecurityEmailKindPasswordReset, intent.Kind)
	require.NotEqual(t, intent.ID, intent.PasswordResetTokenID)
	require.Equal(t, securityEmailProviderIdempotencyKey(intent.ID), intent.ProviderIdempotencyKey)
	require.Equal(t, "password-reset-test-v1", intent.KeyID)
	require.NotContains(t, string(intent.PayloadCiphertext), "recovery.user@example.com")
	message, err := codec.OpenPasswordReset(SecurityEmailPayloadMetadata{
		DeliveryID: intent.ID, CredentialID: intent.PasswordResetTokenID, Kind: intent.Kind,
		ProviderIdempotencyKey: intent.ProviderIdempotencyKey,
		ExpiresAt:              intent.PayloadExpiresAt,
	}, intent.KeyID, intent.PayloadCiphertext, intent.AvailableAt)
	require.NoError(t, err)
	require.Equal(t, "recovery.user@example.com", message.Recipient)
	require.True(t, bytes.HasPrefix([]byte(message.Token), []byte(passwordResetTokenPrefix)))

	repository.mu.Lock()
	issue := repository.issue
	repository.mu.Unlock()
	require.Equal(t, repository.contact.ID, issue.ContactID)
	require.Equal(t, repository.contact.Version, issue.ContactVersion)
	require.Equal(t, "password-reset-test-v1", issue.Token.KeyID)
	require.Len(t, issue.Token.Hash, 32)
	require.NotEqual(t, []byte(message.Token), issue.Token.Hash)
	require.Equal(t, intent.AvailableAt.Add(-15*time.Minute), issue.ShortWindowStart)
	require.Equal(t, intent.AvailableAt.Add(-24*time.Hour), issue.DailyWindowStart)
	require.Equal(t, 3, issue.MaximumPerShortWindow)
	require.Equal(t, 10, issue.MaximumPerDay)
}

func TestPasswordResetObscurePerformsBoundedCryptoWithoutPersistence(t *testing.T) {
	controller := gomock.NewController(t)
	repository := &passwordResetMemoryRepository{}
	scheduler := &passwordResetScheduleCapture{}
	service, _ := newPasswordResetTestService(
		t,
		repository,
		idmocks.NewMockCredentialRepository(controller),
		&passwordResetTokenLifecycleSpy{},
		scheduler,
		lifecycleTestTx{},
	)

	require.NoError(t, service.Obscure(context.Background()))
	repository.mu.Lock()
	require.Equal(t, uuid.Nil, repository.issue.ID)
	repository.mu.Unlock()
	scheduler.mu.Lock()
	require.Zero(t, scheduler.enqueueCalls)
	require.Empty(t, scheduler.intents)
	scheduler.mu.Unlock()
}

func TestPasswordResetDeliveryIntentRollsBackWithOuterTransaction(t *testing.T) {
	controller := gomock.NewController(t)
	credentials := idmocks.NewMockCredentialRepository(controller)
	repository := &passwordResetMemoryRepository{}
	manager := &lifecycleDeferredTx{}
	scheduler := &passwordResetScheduleCapture{manager: manager}
	service, _ := newPasswordResetTestService(
		t, repository, credentials, &passwordResetTokenLifecycleSpy{}, scheduler, manager,
	)
	rollbackErr := errors.New("later mutation failed")

	err := manager.DoInTx(context.Background(), func(txCtx context.Context) error {
		require.NoError(t, service.Issue(txCtx, passwordResetTestUser()))
		scheduler.mu.Lock()
		require.Equal(t, 1, scheduler.enqueueCalls)
		require.Empty(t, scheduler.intents)
		scheduler.mu.Unlock()
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)
	scheduler.mu.Lock()
	require.Empty(t, scheduler.intents)
	scheduler.mu.Unlock()
}

func TestPasswordResetIssueFailsWhenDeliveryIntentCannotBePersisted(t *testing.T) {
	controller := gomock.NewController(t)
	scheduleErr := errors.New("delivery storage unavailable")
	service, _ := newPasswordResetTestService(
		t,
		&passwordResetMemoryRepository{},
		idmocks.NewMockCredentialRepository(controller),
		&passwordResetTokenLifecycleSpy{},
		&passwordResetScheduleCapture{err: scheduleErr},
		lifecycleTestTx{},
	)

	err := service.Issue(context.Background(), passwordResetTestUser())
	require.Error(t, err)
	require.ErrorIs(t, err, scheduleErr)
}

func TestPasswordResetConsumeIsSingleUseAndRevokesAllSessions(t *testing.T) {
	controller := gomock.NewController(t)
	credentials := idmocks.NewMockCredentialRepository(controller)
	repository := &passwordResetMemoryRepository{}
	scheduler := &passwordResetScheduleCapture{}
	lifecycle := &passwordResetTokenLifecycleSpy{}
	service, codec := newPasswordResetTestService(
		t, repository, credentials, lifecycle, scheduler, lifecycleTestTx{},
	)
	user := passwordResetTestUser()
	require.NoError(t, service.Issue(context.Background(), user))
	scheduler.mu.Lock()
	intent := scheduler.intents[0]
	scheduler.mu.Unlock()
	message, err := codec.OpenPasswordReset(SecurityEmailPayloadMetadata{
		DeliveryID: intent.ID, CredentialID: intent.PasswordResetTokenID, Kind: intent.Kind,
		ProviderIdempotencyKey: intent.ProviderIdempotencyKey,
		ExpiresAt:              intent.PayloadExpiresAt,
	}, intent.KeyID, intent.PayloadCiphertext, intent.AvailableAt)
	require.NoError(t, err)
	rawToken := message.Token

	credential := &entity.Credential{
		ID: uuid.New(), UserID: user.ID, Type: credentialTypePassword,
		CredentialData: map[string]any{credentialKeyHash: "$2a$10$old"},
	}
	credentials.EXPECT().GetByUserID(gomock.Any(), user.ID, credentialTypePassword).Return(credential, nil)
	credentials.EXPECT().Update(gomock.Any(), credential).DoAndReturn(
		func(_ context.Context, updated *entity.Credential) error {
			hash, ok := updated.CredentialData[credentialKeyHash].(string)
			require.True(t, ok)
			verifier, verifierErr := passwordsecurity.New(passwordsecurity.DefaultPolicy())
			require.NoError(t, verifierErr)
			result, verifyErr := verifier.Verify(context.Background(), hash, []byte("new-password-123"))
			require.NoError(t, verifyErr)
			require.True(t, result.Valid)
			return nil
		},
	)

	require.NoError(t, service.Consume(context.Background(), rawToken, "new-password-123"))
	lifecycle.mu.Lock()
	require.Equal(t, user.ID, lifecycle.userID)
	require.Equal(t, "password_reset", lifecycle.reason)
	lifecycle.mu.Unlock()
	mutations := service.journal.(*securityMutationJournalCapture).snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationPasswordResetConsumed, mutations[0].Kind)
	require.Equal(t, user.ID, mutations[0].UserID)
	require.Equal(t, user.SecurityVersion+1, mutations[0].SecurityVersion)

	err = service.Consume(context.Background(), rawToken, "another-password-123")
	require.ErrorIs(t, err, ports.ErrPasswordResetRejected)
}

func TestPasswordResetCannotCreatePasswordCredentialForOAuthOnlyAccount(t *testing.T) {
	controller := gomock.NewController(t)
	credentials := idmocks.NewMockCredentialRepository(controller)
	repository := &passwordResetMemoryRepository{}
	scheduler := &passwordResetScheduleCapture{}
	lifecycle := &passwordResetTokenLifecycleSpy{}
	service, codec := newPasswordResetTestService(
		t, repository, credentials, lifecycle, scheduler, lifecycleTestTx{},
	)
	user := passwordResetTestUser()
	require.NoError(t, service.Issue(context.Background(), user))
	scheduler.mu.Lock()
	delivery := scheduler.intents[0]
	scheduler.mu.Unlock()
	message, err := codec.OpenPasswordReset(SecurityEmailPayloadMetadata{
		DeliveryID: delivery.ID, CredentialID: delivery.PasswordResetTokenID, Kind: delivery.Kind,
		ProviderIdempotencyKey: delivery.ProviderIdempotencyKey,
		ExpiresAt:              delivery.PayloadExpiresAt,
	}, delivery.KeyID, delivery.PayloadCiphertext, delivery.AvailableAt)
	require.NoError(t, err)

	credentials.EXPECT().GetByUserID(
		gomock.Any(), user.ID, credentialTypePassword,
	).Return(nil, apperr.New(apperr.CodeNotFound, "credential not found", nil))

	err = service.Consume(context.Background(), message.Token, "new-password-123")
	require.ErrorIs(t, err, ports.ErrPasswordResetRejected)
	repository.mu.Lock()
	require.False(t, repository.consumed)
	repository.mu.Unlock()
	lifecycle.mu.Lock()
	require.Equal(t, uuid.Nil, lifecycle.userID)
	lifecycle.mu.Unlock()
}

func TestPasswordResetRejectsMalformedCredentialBeforePersistence(t *testing.T) {
	controller := gomock.NewController(t)
	service, _ := newPasswordResetTestService(
		t,
		&passwordResetMemoryRepository{},
		idmocks.NewMockCredentialRepository(controller),
		&passwordResetTokenLifecycleSpy{},
		&passwordResetScheduleCapture{},
		lifecycleTestTx{},
	)
	for _, credential := range []string{
		"not-a-reset-token",
		"eyJhbGciOiJSUzI1NiJ9.eyJ0eXBlIjoicmVzZXQifQ.signature",
	} {
		err := service.Consume(context.Background(), credential, "new-password-123")
		require.ErrorIs(t, err, ports.ErrPasswordResetRejected)
	}
}

func newPasswordResetTestService(
	t *testing.T,
	repository ports.PasswordResetRepository,
	credentials ports.CredentialRepository,
	lifecycle ports.TokenLifecycle,
	scheduler ports.SecurityEmailScheduler,
	manager interface {
		DoInTx(context.Context, func(context.Context) error) error
	},
) (*PasswordResetService, *SecurityEmailPayloadCodec) {
	t.Helper()
	codec, err := NewSecurityEmailPayloadCodec(SecurityEmailPayloadCodecConfig{
		ActiveKeyID: "password-reset-test-v1",
		Keys: map[string][]byte{
			"password-reset-test-v1": bytes.Repeat([]byte{0xa1}, 32),
		},
		MaximumCiphertextBytes: 4096,
		Clock: func() time.Time {
			return time.Date(2026, 7, 19, 14, 0, 0, 0, time.UTC)
		},
		Random: bytes.NewReader(bytes.Repeat([]byte{0x61}, 4096)),
	})
	require.NoError(t, err)
	protector, err := NewRecoveryContactProtector(RecoveryContactProtectorConfig{
		ActiveKeyID: "password-reset-test-v1",
		Keys: map[string][]byte{
			"password-reset-test-v1": bytes.Repeat([]byte{0x91}, 32),
		},
		Random: bytes.NewReader(bytes.Repeat([]byte{0x72}, 4096)),
	})
	require.NoError(t, err)
	if memory, ok := repository.(*passwordResetMemoryRepository); ok && memory.contact == nil {
		user := passwordResetTestUser()
		contactID := uuid.MustParse("00000000-0000-7000-8000-000000000902")
		metadata := RecoveryContactMetadata{
			UserID: user.ID, ContactID: contactID, Kind: recoveryContactKindEmail, Version: 2,
		}
		ciphertext, keyID, protectErr := protector.Protect(metadata, "recovery.user@example.com")
		require.NoError(t, protectErr)
		memory.contact = &entity.RecoveryContact{
			ID: contactID, UserID: user.ID, Kind: recoveryContactKindEmail,
			Status: entity.RecoveryContactStatusVerified, Version: 2,
			AddressCiphertext: ciphertext, KeyID: keyID,
		}
	}
	service, err := NewPasswordResetService(
		repository, credentials, lifecycle, scheduler, codec, protector, manager,
		&securityMutationJournalCapture{},
		PasswordResetConfig{
			TTL: 15 * time.Minute, MaximumBytes: 256,
			Clock: func() time.Time { return time.Date(2026, 7, 19, 14, 0, 0, 0, time.UTC) },
		},
	)
	require.NoError(t, err)
	return service, codec
}

func passwordResetTestUser() *entity.User {
	return &entity.User{
		ID: uuid.MustParse("00000000-0000-7000-8000-000000000901"), Username: "reset-user",
		SecurityStatus: entity.UserSecurityStatusActive, SecurityVersion: 4,
	}
}
