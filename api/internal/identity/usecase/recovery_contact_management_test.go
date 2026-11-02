package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil/mocks"
)

type recoveryContactManagementMemoryRepository struct {
	*recoveryContactMemoryRepository
	state *ports.RecoveryContactState
}

type recoveryContactMutationMemoryRepository struct {
	*recoveryContactManagementMemoryRepository
	replacement ports.RecoveryContactReplacementCommand
	resend      ports.RecoveryContactResendCommand
	cancel      ports.RecoveryContactVersionCommand
	remove      ports.RecoveryContactVersionCommand
	replay      bool
}

func (repository *recoveryContactMutationMemoryRepository) StartReplacement(
	_ context.Context,
	command ports.RecoveryContactReplacementCommand,
) (bool, error) {
	repository.replacement = command
	if repository.replay {
		return false, nil
	}
	contact := command.Issue.Contact
	repository.state.Pending = &contact
	repository.state.PendingVerificationExpiresAt = command.Issue.ExpiresAt
	repository.state.PendingLastIssuedAt = command.Issue.IssuedAt
	repository.state.PendingDeliveryState = entity.RecoveryContactDeliveryQueued
	return true, nil
}

func (repository *recoveryContactMutationMemoryRepository) ResendPending(
	_ context.Context,
	command ports.RecoveryContactResendCommand,
) (bool, error) {
	repository.resend = command
	if repository.replay {
		return false, nil
	}
	repository.state.PendingVerificationExpiresAt = command.ExpiresAt
	repository.state.PendingLastIssuedAt = command.IssuedAt
	repository.state.PendingDeliveryState = entity.RecoveryContactDeliveryQueued
	return true, nil
}

func (repository *recoveryContactMutationMemoryRepository) CancelPending(
	_ context.Context,
	command ports.RecoveryContactVersionCommand,
) (bool, error) {
	repository.cancel = command
	if repository.replay {
		return false, nil
	}
	repository.state.Pending = nil
	return true, nil
}

func (repository *recoveryContactMutationMemoryRepository) RemoveCurrent(
	_ context.Context,
	command ports.RecoveryContactVersionCommand,
) (bool, error) {
	repository.remove = command
	if repository.replay {
		return false, nil
	}
	repository.state.Current = nil
	return true, nil
}

type recoveryContactSessionCapture struct {
	userID, sessionID uuid.UUID
	at                time.Time
	updated           bool
	calls             int
}

func (repository *recoveryContactSessionCapture) MarkReauthenticated(
	_ context.Context,
	sessionID, userID uuid.UUID,
	at time.Time,
) (bool, error) {
	repository.calls++
	repository.sessionID = sessionID
	repository.userID = userID
	repository.at = at
	return repository.updated, nil
}

func (repository *recoveryContactManagementMemoryRepository) GetState(
	_ context.Context,
	userID, sessionID uuid.UUID,
	_ time.Time,
) (*ports.RecoveryContactState, error) {
	if repository.state == nil || repository.state.UserID != userID ||
		repository.state.SessionID != sessionID {
		return nil, ports.ErrRecoveryContactRejected
	}
	result := *repository.state
	return &result, nil
}

func TestRecoveryContactProjectionDecryptsOnlyToReturnMaskedAddresses(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	current := entity.RecoveryContact{
		ID: uuid.New(), UserID: userID, Kind: recoveryContactKindEmail,
		Purpose: entity.RecoveryContactPurposeRegistration,
		Status:  entity.RecoveryContactStatusVerified, Version: 2,
	}
	pending := entity.RecoveryContact{
		ID: uuid.New(), UserID: userID, Kind: recoveryContactKindEmail,
		Purpose: entity.RecoveryContactPurposeReplacement,
		Status:  entity.RecoveryContactStatusPending, Version: 3,
	}
	repository := &recoveryContactManagementMemoryRepository{
		recoveryContactMemoryRepository: &recoveryContactMemoryRepository{},
		state: &ports.RecoveryContactState{
			UserID: userID, SessionID: sessionID,
			Current: &current, Pending: &pending,
			CanRemove:                    true,
			LastReauthenticatedAt:        now.Add(-2 * time.Minute),
			PendingVerificationExpiresAt: now.Add(20 * time.Hour),
			PendingLastIssuedAt:          now.Add(-30 * time.Second),
			PendingDeliveryState:         entity.RecoveryContactDeliveryQueued,
		},
	}
	manager := &lifecycleDeferredTx{}
	service, _, protector := newRecoveryContactTestService(t, repository, &passwordResetScheduleCapture{manager: manager}, manager, now)
	current.AddressCiphertext, current.KeyID = protectRecoveryAddress(t, protector, current, "owner@example.com")
	pending.AddressCiphertext, pending.KeyID = protectRecoveryAddress(t, protector, pending, "next@example.com")
	repository.state.Current = &current
	repository.state.Pending = &pending

	projection, err := service.Get(context.Background(), userID, sessionID)
	require.NoError(t, err)
	require.Equal(t, "o***@example.com", projection.Current.MaskedAddress)
	require.Equal(t, entity.RecoveryContactStatusVerified, projection.Current.Status)
	require.Equal(t, uint64(2), projection.Current.Version)
	require.Equal(t, "n***@example.com", projection.Pending.MaskedAddress)
	require.Equal(t, entity.RecoveryContactPurposeReplacement, projection.Pending.Purpose)
	require.Equal(t, entity.RecoveryContactDeliveryQueued, projection.Pending.DeliveryState)
	require.NotNil(t, projection.ReauthenticatedUntil)
	require.Equal(t, now.Add(8*time.Minute), *projection.ReauthenticatedUntil)
	require.False(t, projection.CanRemove, "a pending replacement must fail closed")

	repository.state.Pending = nil
	projection, err = service.Get(context.Background(), userID, sessionID)
	require.NoError(t, err)
	require.True(t, projection.CanRemove)
}

func TestRecoveryContactReauthenticationUpdatesOnlyTheCurrentLiveSession(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	hash := testHashPassword(t, "correct horse battery staple")

	controller := gomock.NewController(t)
	credentials := mocks.NewMockCredentialRepository(controller)
	sessions := &recoveryContactSessionCapture{updated: true}
	credentials.EXPECT().GetByUserID(gomock.Any(), userID, credentialTypePassword).Return(&entity.Credential{
		UserID: userID, Type: credentialTypePassword,
		CredentialData: map[string]any{credentialKeyHash: hash},
	}, nil)

	repository := &recoveryContactManagementMemoryRepository{recoveryContactMemoryRepository: &recoveryContactMemoryRepository{}}
	manager := &lifecycleDeferredTx{}
	service, _, _ := newRecoveryContactTestService(t, repository, &passwordResetScheduleCapture{manager: manager}, manager, now)
	require.NoError(t, service.ConfigureManagement(credentials, sessions))

	until, err := service.Reauthenticate(context.Background(), userID, sessionID, "correct horse battery staple")
	require.NoError(t, err)
	require.Equal(t, now.Add(10*time.Minute), until)
	require.Equal(t, 1, sessions.calls)
	require.Equal(t, userID, sessions.userID)
	require.Equal(t, sessionID, sessions.sessionID)
	require.Equal(t, now, sessions.at)
}

func TestRecoveryContactReauthenticationRejectsWrongPasswordWithoutTouchingSession(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	hash := testHashPassword(t, "correct horse battery staple")

	controller := gomock.NewController(t)
	credentials := mocks.NewMockCredentialRepository(controller)
	sessions := &recoveryContactSessionCapture{updated: true}
	credentials.EXPECT().GetByUserID(gomock.Any(), userID, credentialTypePassword).Return(&entity.Credential{
		UserID: userID, Type: credentialTypePassword,
		CredentialData: map[string]any{credentialKeyHash: hash},
	}, nil)

	repository := &recoveryContactManagementMemoryRepository{recoveryContactMemoryRepository: &recoveryContactMemoryRepository{}}
	manager := &lifecycleDeferredTx{}
	service, _, _ := newRecoveryContactTestService(t, repository, &passwordResetScheduleCapture{manager: manager}, manager, now)
	require.NoError(t, service.ConfigureManagement(credentials, sessions))

	_, err := service.Reauthenticate(context.Background(), userID, sessionID, "wrong password")
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, apperr.CodeUnauthorized, applicationError.Code)
	require.Zero(t, sessions.calls)
}

func TestRecoveryContactReplacementKeepsCurrentLiveAndStagesProtectedCandidate(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	current := entity.RecoveryContact{
		ID: uuid.New(), UserID: userID, Kind: recoveryContactKindEmail,
		Purpose: entity.RecoveryContactPurposeRegistration,
		Status:  entity.RecoveryContactStatusVerified, Version: 2,
	}
	base := &recoveryContactManagementMemoryRepository{
		recoveryContactMemoryRepository: &recoveryContactMemoryRepository{},
		state: &ports.RecoveryContactState{
			UserID: userID, SessionID: sessionID, Current: &current,
			LastReauthenticatedAt: now.Add(-time.Minute),
		},
	}
	repository := &recoveryContactMutationMemoryRepository{recoveryContactManagementMemoryRepository: base}
	manager := &lifecycleDeferredTx{}
	scheduler := &passwordResetScheduleCapture{manager: manager}
	service, codec, protector := newRecoveryContactTestService(t, repository, scheduler, manager, now)
	current.AddressCiphertext, current.KeyID = protectRecoveryAddress(t, protector, current, "owner@example.com")
	repository.state.Current = &current

	projection, err := service.Replace(
		context.Background(), userID, sessionID, uuid.New(), " next@example.com ", 2,
	)
	require.NoError(t, err)
	require.Equal(t, "o***@example.com", projection.Current.MaskedAddress)
	require.Equal(t, "n***@example.com", projection.Pending.MaskedAddress)
	require.Equal(t, entity.RecoveryContactPurposeReplacement, repository.replacement.Issue.Contact.Purpose)
	require.Equal(t, entity.RecoveryContactStatusPending, repository.replacement.Issue.Contact.Status)
	require.Equal(t, uint64(3), repository.replacement.Issue.Contact.Version)
	require.NotContains(t, string(repository.replacement.Issue.Contact.AddressCiphertext), "next@example.com")
	require.Len(t, repository.replacement.Command.RequestFingerprint, 32)
	require.Equal(t, uint64(2), repository.replacement.ExpectedCurrentVersion)

	scheduler.mu.Lock()
	require.Len(t, scheduler.intents, 1)
	delivery := scheduler.intents[0]
	scheduler.mu.Unlock()
	message, err := codec.OpenRecoveryContactVerification(SecurityEmailPayloadMetadata{
		DeliveryID: delivery.ID, CredentialID: delivery.VerificationTokenID, Kind: delivery.Kind,
		ProviderIdempotencyKey: delivery.ProviderIdempotencyKey, ExpiresAt: delivery.PayloadExpiresAt,
	}, delivery.KeyID, delivery.PayloadCiphertext, now)
	require.NoError(t, err)
	require.Equal(t, "next@example.com", message.Recipient)
	mutations := service.journal.(*securityMutationJournalCapture).snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationRecoveryContactReplacementStarted, mutations[0].Kind)
	require.Equal(t, repository.replacement.Command.ReceiptID, mutations[0].ID)
	require.Equal(t, repository.replacement.Issue.Contact.ID, *mutations[0].ContactID)
}

func TestRecoveryContactResendCancelAndRemoveUseExpectedVersions(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	current := entity.RecoveryContact{
		ID: uuid.New(), UserID: userID, Kind: recoveryContactKindEmail,
		Purpose: entity.RecoveryContactPurposeRegistration,
		Status:  entity.RecoveryContactStatusVerified, Version: 2,
	}
	pending := entity.RecoveryContact{
		ID: uuid.New(), UserID: userID, Kind: recoveryContactKindEmail,
		Purpose: entity.RecoveryContactPurposeReplacement,
		Status:  entity.RecoveryContactStatusPending, Version: 3,
	}
	base := &recoveryContactManagementMemoryRepository{
		recoveryContactMemoryRepository: &recoveryContactMemoryRepository{},
		state: &ports.RecoveryContactState{
			UserID: userID, SessionID: sessionID, Current: &current, Pending: &pending,
			LastReauthenticatedAt: now.Add(-time.Minute), PendingLastIssuedAt: now.Add(-2 * time.Minute),
			PendingVerificationExpiresAt: now.Add(20 * time.Hour),
			PendingDeliveryState:         entity.RecoveryContactDeliverySent,
		},
	}
	repository := &recoveryContactMutationMemoryRepository{recoveryContactManagementMemoryRepository: base}
	manager := &lifecycleDeferredTx{}
	scheduler := &passwordResetScheduleCapture{manager: manager}
	service, _, protector := newRecoveryContactTestService(t, repository, scheduler, manager, now)
	current.AddressCiphertext, current.KeyID = protectRecoveryAddress(t, protector, current, "owner@example.com")
	pending.AddressCiphertext, pending.KeyID = protectRecoveryAddress(t, protector, pending, "next@example.com")
	repository.state.Current = &current
	repository.state.Pending = &pending

	_, err := service.Resend(context.Background(), userID, sessionID, uuid.New(), 3)
	require.NoError(t, err)
	require.Equal(t, pending.ID, repository.resend.ContactID)
	require.Equal(t, uint64(3), repository.resend.ExpectedPendingVersion)

	_, err = service.CancelPending(context.Background(), userID, sessionID, uuid.New(), 3)
	require.NoError(t, err)
	require.Equal(t, uint64(3), repository.cancel.ExpectedVersion)

	_, err = service.Remove(context.Background(), userID, sessionID, uuid.New(), 2)
	require.NoError(t, err)
	require.Equal(t, uint64(2), repository.remove.ExpectedVersion)
	mutations := service.journal.(*securityMutationJournalCapture).snapshot()
	require.Len(t, mutations, 3)
	require.Equal(t, entity.SecurityMutationRecoveryContactVerificationResent, mutations[0].Kind)
	require.Equal(t, entity.SecurityMutationRecoveryContactPendingCancelled, mutations[1].Kind)
	require.Equal(t, entity.SecurityMutationRecoveryContactRemoved, mutations[2].Kind)
}

func TestRecoveryContactExactReplayDoesNotDuplicateDeliveryOrSecurityJournal(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	pending := entity.RecoveryContact{
		ID: uuid.New(), UserID: userID, Kind: recoveryContactKindEmail,
		Purpose: entity.RecoveryContactPurposeReplacement,
		Status:  entity.RecoveryContactStatusPending, Version: 3,
	}
	base := &recoveryContactManagementMemoryRepository{
		recoveryContactMemoryRepository: &recoveryContactMemoryRepository{},
		state: &ports.RecoveryContactState{
			UserID: userID, SessionID: sessionID, Pending: &pending,
			LastReauthenticatedAt:        now.Add(-time.Minute),
			PendingLastIssuedAt:          now.Add(-2 * time.Minute),
			PendingVerificationExpiresAt: now.Add(20 * time.Hour),
			PendingDeliveryState:         entity.RecoveryContactDeliverySent,
		},
	}
	repository := &recoveryContactMutationMemoryRepository{
		recoveryContactManagementMemoryRepository: base,
		replay: true,
	}
	manager := &lifecycleDeferredTx{}
	scheduler := &passwordResetScheduleCapture{manager: manager}
	service, _, protector := newRecoveryContactTestService(t, repository, scheduler, manager, now)
	pending.AddressCiphertext, pending.KeyID = protectRecoveryAddress(
		t, protector, pending, "next@example.com",
	)
	repository.state.Pending = &pending

	projection, err := service.Resend(
		context.Background(), userID, sessionID, uuid.New(), pending.Version,
	)

	require.NoError(t, err)
	require.NotNil(t, projection.Pending)
	require.Equal(t, pending.Version, projection.Pending.Version)
	scheduler.mu.Lock()
	require.Zero(t, scheduler.enqueueCalls)
	require.Empty(t, scheduler.intents)
	scheduler.mu.Unlock()
	require.Empty(t, service.journal.(*securityMutationJournalCapture).snapshot())
}

func protectRecoveryAddress(
	t *testing.T,
	protector *RecoveryContactProtector,
	contact entity.RecoveryContact,
	address string,
) ([]byte, string) {
	t.Helper()
	ciphertext, keyID, err := protector.Protect(RecoveryContactMetadata{
		UserID: contact.UserID, ContactID: contact.ID, Kind: contact.Kind,
		Version: contact.Version,
	}, address)
	require.NoError(t, err)
	return ciphertext, keyID
}
