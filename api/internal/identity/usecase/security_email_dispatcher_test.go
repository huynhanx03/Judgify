package usecase

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/judgify/internal/ports"
)

type securityEmailRepositoryStub struct {
	mu sync.Mutex

	claim        func(ports.SecurityEmailClaim) ([]ports.SecurityEmailLease, error)
	prepare      func(ports.SecurityEmailPrepareCommand) (bool, error)
	claimCalls   int
	prepareCalls int
	sent         []ports.SecurityEmailLeaseOutcome
	retried      []ports.SecurityEmailLeaseOutcome
	dead         []ports.SecurityEmailLeaseOutcome
	terminalHit  chan struct{}
	notifyOnce   sync.Once
}

func (repository *securityEmailRepositoryStub) PrepareForSend(
	_ context.Context,
	command ports.SecurityEmailPrepareCommand,
) (bool, error) {
	repository.mu.Lock()
	repository.prepareCalls++
	callback := repository.prepare
	repository.mu.Unlock()
	if callback == nil {
		return true, nil
	}
	return callback(command)
}

func (*securityEmailRepositoryStub) Enqueue(context.Context, ports.SecurityEmailDeliveryIntent) error {
	return nil
}

func (repository *securityEmailRepositoryStub) Claim(
	_ context.Context,
	claim ports.SecurityEmailClaim,
) ([]ports.SecurityEmailLease, error) {
	repository.mu.Lock()
	repository.claimCalls++
	callback := repository.claim
	repository.mu.Unlock()
	if callback == nil {
		return nil, nil
	}
	return callback(claim)
}

func (repository *securityEmailRepositoryStub) MarkSent(
	_ context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
) error {
	repository.mu.Lock()
	repository.sent = append(repository.sent, outcome)
	repository.mu.Unlock()
	repository.notify()
	return nil
}

func (repository *securityEmailRepositoryStub) MarkRetry(
	_ context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
) error {
	repository.mu.Lock()
	repository.retried = append(repository.retried, outcome)
	repository.mu.Unlock()
	repository.notify()
	return nil
}

func (repository *securityEmailRepositoryStub) MarkDead(
	_ context.Context,
	outcome ports.SecurityEmailLeaseOutcome,
) error {
	repository.mu.Lock()
	repository.dead = append(repository.dead, outcome)
	repository.mu.Unlock()
	repository.notify()
	return nil
}

func (repository *securityEmailRepositoryStub) notify() {
	if repository.terminalHit != nil {
		repository.notifyOnce.Do(func() { close(repository.terminalHit) })
	}
}

type securityEmailDeliveryStub struct {
	mu                   sync.Mutex
	messages             []ports.PasswordResetDeliveryMessage
	verificationMessages []ports.RecoveryContactVerificationDeliveryMessage
	invitationMessages   []ports.AccountInvitationDeliveryMessage
	err                  error
	deliver              func(context.Context, ports.PasswordResetDeliveryMessage) error
}

func (delivery *securityEmailDeliveryStub) DeliverVerification(
	_ context.Context,
	message ports.RecoveryContactVerificationDeliveryMessage,
) error {
	delivery.mu.Lock()
	defer delivery.mu.Unlock()
	delivery.verificationMessages = append(delivery.verificationMessages, message)
	return delivery.err
}

func (delivery *securityEmailDeliveryStub) DeliverInvitation(
	_ context.Context,
	message ports.AccountInvitationDeliveryMessage,
) error {
	delivery.mu.Lock()
	defer delivery.mu.Unlock()
	delivery.invitationMessages = append(delivery.invitationMessages, message)
	return delivery.err
}

func TestSecurityEmailDispatcherDeliversRecoveryContactVerification(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailVerificationLeaseForTest(t, now, now.Add(24*time.Hour))
	repository := &securityEmailRepositoryStub{}
	delivery := &securityEmailDeliveryStub{}
	dispatcher := newSecurityEmailDispatcherForTest(
		t, repository, delivery, codec, func() time.Time { return now },
	)

	require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
	delivery.mu.Lock()
	require.Len(t, delivery.verificationMessages, 1)
	require.Empty(t, delivery.messages)
	require.Equal(t, lease.ProviderIdempotencyKey, delivery.verificationMessages[0].ProviderIdempotencyKey)
	delivery.mu.Unlock()
	repository.mu.Lock()
	require.Len(t, repository.sent, 1)
	require.Empty(t, repository.dead)
	repository.mu.Unlock()
}

func TestSecurityEmailDispatcherDeliversAccountInvitation(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailInvitationLeaseForTest(t, now, now.Add(24*time.Hour))
	repository := &securityEmailRepositoryStub{}
	delivery := &securityEmailDeliveryStub{}
	dispatcher := newSecurityEmailDispatcherForTest(
		t, repository, delivery, codec, func() time.Time { return now },
	)

	require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
	delivery.mu.Lock()
	require.Len(t, delivery.invitationMessages, 1)
	require.Empty(t, delivery.messages)
	require.Empty(t, delivery.verificationMessages)
	require.Equal(
		t,
		lease.ProviderIdempotencyKey,
		delivery.invitationMessages[0].ProviderIdempotencyKey,
	)
	delivery.mu.Unlock()
	repository.mu.Lock()
	require.Len(t, repository.sent, 1)
	require.Empty(t, repository.dead)
	repository.mu.Unlock()
}

func TestSecurityEmailDispatcherCancelsStaleIntentBeforeDecryptionOrProviderIO(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailVerificationLeaseForTest(t, now, now.Add(24*time.Hour))
	lease.PayloadCiphertext[len(lease.PayloadCiphertext)-1] ^= 0xff
	repository := &securityEmailRepositoryStub{prepare: func(
		command ports.SecurityEmailPrepareCommand,
	) (bool, error) {
		require.Equal(t, lease.ID, command.ID)
		require.Equal(t, lease.VerificationTokenID, command.CredentialID)
		return false, nil
	}}
	delivery := &securityEmailDeliveryStub{}
	dispatcher := newSecurityEmailDispatcherForTest(
		t, repository, delivery, codec, func() time.Time { return now },
	)

	require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
	repository.mu.Lock()
	require.Equal(t, 1, repository.prepareCalls)
	require.Empty(t, repository.sent)
	require.Empty(t, repository.retried)
	require.Empty(t, repository.dead)
	repository.mu.Unlock()
	delivery.mu.Lock()
	require.Empty(t, delivery.verificationMessages)
	require.Empty(t, delivery.messages)
	delivery.mu.Unlock()
}

func (delivery *securityEmailDeliveryStub) Deliver(
	ctx context.Context,
	message ports.PasswordResetDeliveryMessage,
) error {
	delivery.mu.Lock()
	delivery.messages = append(delivery.messages, message)
	callback := delivery.deliver
	err := delivery.err
	delivery.mu.Unlock()
	if callback != nil {
		return callback(ctx, message)
	}
	return err
}

type securityEmailTx struct{}

func (securityEmailTx) DoInTx(
	ctx context.Context,
	callback func(context.Context) error,
) error {
	return callback(ctx)
}

type retryableSecurityEmailFailure struct {
	retryable bool
	class     string
}

func (failure retryableSecurityEmailFailure) Error() string      { return "provider failure" }
func (failure retryableSecurityEmailFailure) Retryable() bool    { return failure.retryable }
func (failure retryableSecurityEmailFailure) ErrorClass() string { return failure.class }

func TestSecurityEmailDispatcherImmediatelyDeliversAndReportsHealthy(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailLeaseForTest(t, now, now.Add(15*time.Minute), 1)
	terminalHit := make(chan struct{})
	repository := &securityEmailRepositoryStub{terminalHit: terminalHit}
	repository.claim = func(ports.SecurityEmailClaim) ([]ports.SecurityEmailLease, error) {
		repository.mu.Lock()
		calls := repository.claimCalls
		repository.mu.Unlock()
		if calls == 1 {
			return []ports.SecurityEmailLease{lease}, nil
		}
		return nil, nil
	}
	delivery := &securityEmailDeliveryStub{}
	dispatcher := newSecurityEmailDispatcherForTest(t, repository, delivery, codec, func() time.Time { return now })
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- dispatcher.Run(runCtx) }()

	select {
	case <-terminalHit:
	case <-time.After(time.Second):
		t.Fatal("immediate security email dispatch did not finish")
	}
	require.Eventually(t, func() bool {
		stats := dispatcher.Stats()
		return stats.Sent == 1 && !stats.LastSuccessAt.IsZero() &&
			dispatcher.Check(context.Background()) == nil
	}, time.Second, time.Millisecond)
	require.NoError(t, dispatcher.Check(context.Background()))
	delivery.mu.Lock()
	require.Len(t, delivery.messages, 1)
	require.Equal(t, lease.ProviderIdempotencyKey, delivery.messages[0].ProviderIdempotencyKey)
	delivery.mu.Unlock()

	cancel()
	require.NoError(t, <-runDone)
	require.ErrorIs(t, dispatcher.Check(context.Background()), ErrSecurityEmailDispatcherHealth)
}

func TestSecurityEmailDispatcherRetriesWithBoundedExponentialBackoff(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailLeaseForTest(t, now, now.Add(15*time.Minute), 2)
	repository := &securityEmailRepositoryStub{}
	delivery := &securityEmailDeliveryStub{err: retryableSecurityEmailFailure{
		retryable: true, class: "provider_unavailable",
	}}
	dispatcher := newSecurityEmailDispatcherForTest(t, repository, delivery, codec, func() time.Time { return now })

	require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
	repository.mu.Lock()
	require.Empty(t, repository.sent)
	require.Empty(t, repository.dead)
	require.Len(t, repository.retried, 1)
	require.Equal(t, now.Add(2*time.Second), repository.retried[0].NextAttemptAt)
	require.Equal(t, "provider_unavailable", repository.retried[0].ErrorClass)
	repository.mu.Unlock()
}

func TestSecurityEmailDispatcherSchedulesRetryFromProviderCompletion(t *testing.T) {
	issueTime := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	currentTime := issueTime
	codec, lease := securityEmailLeaseForTest(t, issueTime, issueTime.Add(15*time.Minute), 1)
	repository := &securityEmailRepositoryStub{}
	delivery := &securityEmailDeliveryStub{deliver: func(
		context.Context,
		ports.PasswordResetDeliveryMessage,
	) error {
		currentTime = issueTime.Add(5 * time.Second)
		return retryableSecurityEmailFailure{retryable: true, class: "provider_unavailable"}
	}}
	dispatcher := newSecurityEmailDispatcherForTest(
		t, repository, delivery, codec, func() time.Time { return currentTime },
	)

	require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
	repository.mu.Lock()
	require.Len(t, repository.retried, 1)
	require.Equal(t, issueTime.Add(6*time.Second), repository.retried[0].NextAttemptAt)
	repository.mu.Unlock()
}

func TestSecurityEmailDispatcherDeadLettersExpiredTamperedOrExhaustedPayload(t *testing.T) {
	issueTime := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		dispatchAt time.Time
		attempt    uint32
		tamper     bool
		wantClass  string
	}{
		{name: "expired", dispatchAt: issueTime.Add(16 * time.Minute), attempt: 1, wantClass: "payload_expired"},
		{name: "tampered", dispatchAt: issueTime, attempt: 1, tamper: true, wantClass: "payload_authentication"},
		{name: "exhausted", dispatchAt: issueTime, attempt: 5, wantClass: "attempts_exhausted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			codec, lease := securityEmailLeaseForTest(t, issueTime, issueTime.Add(15*time.Minute), test.attempt)
			if test.tamper {
				lease.PayloadCiphertext[len(lease.PayloadCiphertext)-1] ^= 0xff
			}
			repository := &securityEmailRepositoryStub{}
			delivery := &securityEmailDeliveryStub{}
			dispatcher := newSecurityEmailDispatcherForTest(
				t, repository, delivery, codec, func() time.Time { return test.dispatchAt },
			)

			require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
			repository.mu.Lock()
			require.Len(t, repository.dead, 1)
			require.Equal(t, test.wantClass, repository.dead[0].ErrorClass)
			repository.mu.Unlock()
			delivery.mu.Lock()
			require.Empty(t, delivery.messages)
			delivery.mu.Unlock()
		})
	}
}

func TestSecurityEmailDispatcherPermanentProviderFailureIsDeadLettered(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailLeaseForTest(t, now, now.Add(15*time.Minute), 1)
	repository := &securityEmailRepositoryStub{}
	delivery := &securityEmailDeliveryStub{err: retryableSecurityEmailFailure{
		retryable: false, class: "provider_rejected",
	}}
	dispatcher := newSecurityEmailDispatcherForTest(t, repository, delivery, codec, func() time.Time { return now })

	require.NoError(t, dispatcher.dispatchLease(context.Background(), lease))
	repository.mu.Lock()
	require.Len(t, repository.dead, 1)
	require.Equal(t, "provider_rejected", repository.dead[0].ErrorClass)
	repository.mu.Unlock()
}

func TestSecurityEmailDispatcherStorageFailureDegradesHealthWithoutLeakingError(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repository := &securityEmailRepositoryStub{claim: func(ports.SecurityEmailClaim) ([]ports.SecurityEmailLease, error) {
		return nil, errors.New("database included raw-user@example.com and jpr1.secret")
	}}
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x55)
	dispatcher := newSecurityEmailDispatcherForTest(
		t, repository, &securityEmailDeliveryStub{}, codec, func() time.Time { return now },
	)

	dispatcher.dispatchCycle(context.Background())
	stats := dispatcher.Stats()
	require.Equal(t, uint64(1), stats.ConsecutiveFailures)
	require.Equal(t, "storage", stats.LastFailureClass)
	require.NotContains(t, stats.LastFailureClass, "raw-user@example.com")
	require.NotContains(t, stats.LastFailureClass, "jpr1.secret")
}

func TestSecurityEmailDispatcherStorageFailureLogKeepsSafeCorrelationFields(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	codec, lease := securityEmailLeaseForTest(t, now, now.Add(15*time.Minute), 1)
	storageErr := errors.New("database included raw-user@example.com and jpr1.secret")
	repository := &securityEmailRepositoryStub{prepare: func(
		ports.SecurityEmailPrepareCommand,
	) (bool, error) {
		return false, storageErr
	}}
	dispatcher := newSecurityEmailDispatcherForTest(
		t, repository, &securityEmailDeliveryStub{}, codec, func() time.Time { return now },
	)

	core, observed := observer.New(zap.DebugLevel)
	ctx := correlation.WithContext(
		context.Background(),
		"018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
	)
	ctx = commonlogger.WithContext(ctx, zap.New(core))
	require.ErrorIs(t, dispatcher.dispatchLease(ctx, lease), storageErr)

	entries := observed.FilterMessage("security email dispatch failed").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17", fields["correlation_id"])
	require.Equal(t, lease.ID.String(), fields["delivery_id"])
	require.Equal(t, string(lease.Kind), fields["security_email_kind"])
	require.Equal(t, "1", fields["attempt"])
	require.Equal(t, "storage", fields["error_class"])
	require.NotContains(t, entries[0].Message, "raw-user@example.com")
	require.NotContains(t, entries[0].Message, "jpr1.secret")
	require.NotContains(t, fields, "error")
}

func TestSecurityEmailDispatcherShutdownBeforeRunIsIdempotent(t *testing.T) {
	now := time.Now().UTC()
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x56)
	dispatcher := newSecurityEmailDispatcherForTest(
		t, &securityEmailRepositoryStub{}, &securityEmailDeliveryStub{}, codec, time.Now,
	)
	require.NoError(t, dispatcher.Shutdown(context.Background()))
	require.NoError(t, dispatcher.Shutdown(context.Background()))
	require.ErrorIs(t, dispatcher.Run(context.Background()), ErrInvalidSecurityEmailDispatcher)
}

func TestSecurityEmailDispatcherRequiresLeaseSafetyMargin(t *testing.T) {
	now := time.Now().UTC()
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, now, 0x58)
	_, err := NewSecurityEmailDispatcher(
		&securityEmailRepositoryStub{},
		&securityEmailDeliveryStub{},
		codec,
		securityEmailTx{},
		SecurityEmailDispatcherConfig{
			Owner: "api-email-test", PollInterval: time.Minute,
			ClaimTimeout: time.Second, DeliveryTimeout: time.Second,
			FinalizeTimeout: time.Second, LeaseDuration: 3 * time.Second,
			BatchSize: 1, Concurrency: 1, MaximumAttempts: 3,
			RetryInitial: time.Second, RetryMaximum: time.Minute, RetryMultiplier: 2,
			Clock: time.Now,
		},
	)
	require.ErrorIs(t, err, ErrInvalidSecurityEmailDispatcher)
}

func securityEmailLeaseForTest(
	t *testing.T,
	issueTime time.Time,
	expiresAt time.Time,
	attempt uint32,
) (*SecurityEmailPayloadCodec, ports.SecurityEmailLease) {
	t.Helper()
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, issueTime, 0x57)
	metadata := securityEmailPayloadMetadataForTest(expiresAt)
	message := ports.PasswordResetDeliveryMessage{
		Recipient: "security.user@example.com",
		Token:     "jpr1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: expiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	ciphertext, keyID, err := codec.SealPasswordReset(metadata, message)
	require.NoError(t, err)
	return codec, ports.SecurityEmailLease{
		ID: metadata.DeliveryID, Kind: metadata.Kind,
		PasswordResetTokenID:   metadata.CredentialID,
		ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
		PayloadCiphertext:      ciphertext, KeyID: keyID, PayloadExpiresAt: expiresAt,
		Attempt: attempt, Owner: "api-email-test", LeaseToken: uuid.New(),
		LeasedUntil: issueTime.Add(time.Minute),
	}
}

func securityEmailVerificationLeaseForTest(
	t *testing.T,
	issueTime time.Time,
	expiresAt time.Time,
) (*SecurityEmailPayloadCodec, ports.SecurityEmailLease) {
	t.Helper()
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, issueTime, 0x59)
	metadata := SecurityEmailPayloadMetadata{
		DeliveryID: uuid.New(), CredentialID: uuid.New(),
		Kind:                   ports.SecurityEmailKindRecoveryContactVerification,
		ProviderIdempotencyKey: "security-email/" + uuid.NewString(),
		ExpiresAt:              expiresAt,
	}
	message := ports.RecoveryContactVerificationDeliveryMessage{
		Recipient: "verify.user@example.com",
		Token:     "jcv1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: expiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	ciphertext, keyID, err := codec.SealRecoveryContactVerification(metadata, message)
	require.NoError(t, err)
	return codec, ports.SecurityEmailLease{
		ID: metadata.DeliveryID, Kind: metadata.Kind,
		VerificationTokenID:    metadata.CredentialID,
		ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
		PayloadCiphertext:      ciphertext, KeyID: keyID, PayloadExpiresAt: expiresAt,
		Attempt: 1, Owner: "api-email-test", LeaseToken: uuid.New(),
		LeasedUntil: issueTime.Add(time.Minute),
	}
}

func securityEmailInvitationLeaseForTest(
	t *testing.T,
	issueTime time.Time,
	expiresAt time.Time,
) (*SecurityEmailPayloadCodec, ports.SecurityEmailLease) {
	t.Helper()
	codec := newSecurityEmailPayloadCodecForTest(t, "active-v1", map[string][]byte{
		"active-v1": bytes.Repeat([]byte{0x31}, 32),
	}, issueTime, 0x5a)
	metadata := SecurityEmailPayloadMetadata{
		DeliveryID: uuid.New(), CredentialID: uuid.New(),
		Kind:                   ports.SecurityEmailKindAccountInvitation,
		ProviderIdempotencyKey: "security-email/" + uuid.NewString(),
		ExpiresAt:              expiresAt,
	}
	message := ports.AccountInvitationDeliveryMessage{
		Recipient: "invited.user@example.com",
		Token:     "jai1." + string(bytes.Repeat([]byte{'A'}, 43)),
		ExpiresAt: expiresAt, ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	ciphertext, keyID, err := codec.SealAccountInvitation(metadata, message)
	require.NoError(t, err)
	return codec, ports.SecurityEmailLease{
		ID: metadata.DeliveryID, Kind: metadata.Kind,
		VerificationTokenID:    metadata.CredentialID,
		ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
		PayloadCiphertext:      ciphertext, KeyID: keyID, PayloadExpiresAt: expiresAt,
		Attempt: 1, Owner: "api-email-test", LeaseToken: uuid.New(),
		LeasedUntil: issueTime.Add(time.Minute),
	}
}

func newSecurityEmailDispatcherForTest(
	t *testing.T,
	repository ports.SecurityEmailDeliveryRepository,
	delivery ports.SecurityEmailDelivery,
	codec *SecurityEmailPayloadCodec,
	clock func() time.Time,
) *SecurityEmailDispatcher {
	t.Helper()
	dispatcher, err := NewSecurityEmailDispatcher(
		repository,
		delivery,
		codec,
		securityEmailTx{},
		SecurityEmailDispatcherConfig{
			Owner: "api-email-test", PollInterval: time.Hour,
			ClaimTimeout: time.Second, DeliveryTimeout: time.Second,
			FinalizeTimeout: time.Second, LeaseDuration: time.Minute,
			BatchSize: 2, Concurrency: 2, MaximumAttempts: 4,
			RetryInitial: time.Second, RetryMaximum: time.Minute, RetryMultiplier: 2,
			Clock: clock,
		},
	)
	require.NoError(t, err)
	return dispatcher
}
