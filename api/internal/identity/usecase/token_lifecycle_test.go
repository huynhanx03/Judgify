package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type lifecycleTestIssuer struct{}

func (lifecycleTestIssuer) Issue(
	ctx context.Context,
	request authentication.IssueRequest,
) (authentication.IssuedToken, error) {
	if err := ctx.Err(); err != nil {
		return authentication.IssuedToken{}, err
	}
	return authentication.IssuedToken{
		Raw:       "access." + request.TokenID,
		TokenID:   request.TokenID,
		ExpiresAt: request.ExpiresAt.UTC(),
		KeyID:     "test-signing-key",
	}, nil
}

type lifecycleTestTx struct{}

func (lifecycleTestTx) DoInTx(ctx context.Context, operation func(context.Context) error) error {
	if ctx == nil || operation == nil {
		return errors.New("invalid test transaction")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return operation(ctx)
}

type lifecycleDeferredTxKey struct{}

type lifecycleDeferredTx struct {
	mu        sync.Mutex
	callbacks []func(context.Context) error
}

func (manager *lifecycleDeferredTx) DoInTx(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if ctx == nil || operation == nil {
		return errors.New("invalid deferred transaction")
	}
	if active, _ := ctx.Value(lifecycleDeferredTxKey{}).(*lifecycleDeferredTx); active == manager {
		return operation(ctx)
	}
	txCtx := context.WithValue(ctx, lifecycleDeferredTxKey{}, manager)
	if err := operation(txCtx); err != nil {
		manager.mu.Lock()
		manager.callbacks = nil
		manager.mu.Unlock()
		return err
	}
	manager.mu.Lock()
	callbacks := append([]func(context.Context) error(nil), manager.callbacks...)
	manager.callbacks = nil
	manager.mu.Unlock()
	for _, callback := range callbacks {
		if err := callback(context.WithoutCancel(ctx)); err != nil {
			return err
		}
	}
	return nil
}

func (manager *lifecycleDeferredTx) RegisterAfterCommit(
	ctx context.Context,
	callback func(context.Context) error,
) error {
	if manager == nil || callback == nil {
		return errors.New("invalid post-commit callback")
	}
	active, _ := ctx.Value(lifecycleDeferredTxKey{}).(*lifecycleDeferredTx)
	if active != manager {
		return errors.New("transaction required")
	}
	manager.mu.Lock()
	manager.callbacks = append(manager.callbacks, callback)
	manager.mu.Unlock()
	return nil
}

type lifecycleTestUser struct {
	id              uuid.UUID
	username        string
	securityVersion uint64
}

type lifecycleMemoryStore struct {
	mu sync.Mutex

	sessions map[uuid.UUID]*entity.Session
	refresh  map[uuid.UUID]*entity.RefreshToken
	issuance map[string]*entity.SessionIssuanceReceipt
	receipts map[string]*entity.RefreshReceipt
	users    map[uuid.UUID]lifecycleTestUser

	invalidated      []uuid.UUID
	invalidatedUsers []uuid.UUID
	revocationOps    []string
}

func newLifecycleMemoryStore(user *entity.User) *lifecycleMemoryStore {
	store := &lifecycleMemoryStore{
		sessions: make(map[uuid.UUID]*entity.Session),
		refresh:  make(map[uuid.UUID]*entity.RefreshToken),
		issuance: make(map[string]*entity.SessionIssuanceReceipt),
		receipts: make(map[string]*entity.RefreshReceipt),
		users:    make(map[uuid.UUID]lifecycleTestUser),
	}
	store.users[user.ID] = lifecycleTestUser{
		id: user.ID, username: user.Username, securityVersion: user.SecurityVersion,
	}
	return store
}

func (store *lifecycleMemoryStore) createValue(value any) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	switch record := value.(type) {
	case *entity.Session:
		if _, exists := store.sessions[record.ID]; exists {
			return errors.New("duplicate session")
		}
		store.sessions[record.ID] = cloneLifecycleSession(record)
		return nil
	case *entity.RefreshToken:
		if _, exists := store.refresh[record.ID]; exists {
			return errors.New("duplicate refresh token")
		}
		store.refresh[record.ID] = cloneLifecycleRefresh(record)
		return nil
	case *entity.SessionIssuanceReceipt:
		selector := ports.IssuanceReceiptSelector{
			Purpose: record.Purpose, IdempotencyKey: record.IdempotencyKey,
		}
		if record.Purpose == entity.SessionIssuancePasswordLogin {
			selector.UserID = valueOrNilUUID(record.UserID)
			selector.PreSessionID = valueOrNilUUID(record.PreSessionID)
		} else {
			selector.RequestID = valueOrNilUUID(record.RequestOrTicketID)
		}
		key := lifecycleIssuanceKey(selector)
		if _, exists := store.issuance[key]; exists {
			return errors.New("duplicate issuance receipt")
		}
		store.issuance[key] = cloneLifecycleIssuance(record)
		return nil
	default:
		return fmt.Errorf("unsupported create type %T", value)
	}
}

func (store *lifecycleMemoryStore) getIssuance(
	selector ports.IssuanceReceiptSelector,
) (*entity.SessionIssuanceReceipt, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.issuance[lifecycleIssuanceKey(selector)]
	if record == nil {
		return nil, apperr.New(apperr.CodeNotFound, "issuance receipt not found", nil)
	}
	return cloneLifecycleIssuance(record), nil
}

func (store *lifecycleMemoryStore) markIssuanceRecovered(id uuid.UUID, recoveredAt time.Time) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, receipt := range store.issuance {
		if receipt.ID == id {
			at := recoveredAt.UTC()
			receipt.RecoveredAt = &at
			return nil
		}
	}
	return errors.New("issuance receipt not found")
}

func (store *lifecycleMemoryStore) GetActiveForAuthentication(
	_ context.Context,
	sessionID, userID uuid.UUID,
) (entity.SessionSecurityState, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session := store.sessions[sessionID]
	owner, exists := store.users[userID]
	if session == nil || !exists || session.UserID != userID {
		return entity.SessionSecurityState{}, errors.New("session not found")
	}
	return entity.SessionSecurityState{
		SessionID: session.ID, UserID: session.UserID,
		SessionStatus: session.Status, SessionSecurityVersion: session.UserSecurityVersion,
		UserStatus: entity.UserSecurityStatusActive, UserSecurityVersion: owner.securityVersion,
		Username: owner.username, LastSeenAt: session.LastSeenAt,
		LastReauthenticatedAt: session.LastReauthenticatedAt,
		IdleExpiresAt:         session.IdleExpiresAt, AbsoluteExpiresAt: session.AbsoluteExpiresAt,
	}, nil
}

func (store *lifecycleMemoryStore) TouchLastSeen(
	_ context.Context,
	sessionID uuid.UUID,
	observedAt, idleExpiresAt time.Time,
) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session := store.sessions[sessionID]
	if session == nil || session.Status != entity.SessionStatusActive {
		return false, nil
	}
	session.LastSeenAt = observedAt.UTC()
	session.IdleExpiresAt = idleExpiresAt.UTC()
	return true, nil
}

func (store *lifecycleMemoryStore) Revoke(
	_ context.Context,
	sessionID uuid.UUID,
	reason string,
	at time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revocationOps = append(store.revocationOps, "session")
	session := store.sessions[sessionID]
	if session == nil {
		return errors.New("session not found")
	}
	if session.Status == entity.SessionStatusActive {
		revokedAt := at.UTC()
		session.Status = entity.SessionStatusRevoked
		session.RevokedAt = &revokedAt
		session.RevokeReason = &reason
	}
	return nil
}

func (store *lifecycleMemoryStore) RevokeAll(
	_ context.Context,
	userID uuid.UUID,
	reason string,
	at time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revocationOps = append(store.revocationOps, "sessions")
	for _, session := range store.sessions {
		if session.UserID == userID && session.Status == entity.SessionStatusActive {
			revokedAt := at.UTC()
			session.Status = entity.SessionStatusRevoked
			session.RevokedAt = &revokedAt
			session.RevokeReason = &reason
		}
	}
	return nil
}

func (store *lifecycleMemoryStore) GetRefresh(_ context.Context, id uuid.UUID) (*entity.RefreshToken, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.refresh[id]
	if record == nil {
		return nil, errors.New("refresh token not found")
	}
	return cloneLifecycleRefresh(record), nil
}

func (store *lifecycleMemoryStore) GetByHash(_ context.Context, tokenHash []byte) (*entity.RefreshToken, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	record := store.refreshByHashLocked(tokenHash)
	if record == nil {
		return nil, errors.New("refresh token not found")
	}
	return cloneLifecycleRefresh(record), nil
}

func (store *lifecycleMemoryStore) RevokeIssuedForSession(
	_ context.Context,
	sessionID uuid.UUID,
	at time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revocationOps = append(store.revocationOps, "refresh")
	store.revokeIssuedLocked(sessionID, at)
	return nil
}

func (store *lifecycleMemoryStore) RevokeIssuedForUser(
	_ context.Context,
	userID uuid.UUID,
	at time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.revocationOps = append(store.revocationOps, "refreshes")
	for sessionID, session := range store.sessions {
		if session.UserID == userID {
			store.revokeIssuedLocked(sessionID, at)
		}
	}
	return nil
}

func (store *lifecycleMemoryStore) ContextByHash(
	_ context.Context,
	tokenHash []byte,
	now time.Time,
) (ports.RefreshContext, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	presented := store.refreshByHashLocked(tokenHash)
	if presented == nil || presented.Status == entity.RefreshTokenStatusRevoked || !presented.ExpiresAt.After(now) {
		return ports.RefreshContext{}, errors.New("refresh token is inactive")
	}
	session := store.sessions[presented.SessionID]
	if session == nil || session.Status != entity.SessionStatusActive ||
		!session.IdleExpiresAt.After(now) || !session.AbsoluteExpiresAt.After(now) {
		return ports.RefreshContext{}, errors.New("session is inactive")
	}
	owner := store.users[session.UserID]
	return ports.RefreshContext{
		TokenID: presented.ID, SessionID: session.ID, UserID: owner.id,
		Username: owner.username, SecurityVersion: owner.securityVersion,
		AuthenticatedAt:   session.LastReauthenticatedAt,
		AbsoluteExpiresAt: session.AbsoluteExpiresAt,
		IdleExpiresAt:     session.IdleExpiresAt, RefreshExpiresAt: presented.ExpiresAt,
	}, nil
}

func (store *lifecycleMemoryStore) Rotate(
	_ context.Context,
	mutation ports.RefreshRotationMutation,
	now time.Time,
) (entity.RefreshRotationResult, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	presented := store.refreshByHashLocked(mutation.PresentedHash)
	if presented == nil {
		return entity.RefreshRotationResult{}, errors.New("refresh token not found")
	}
	if presented.Status == entity.RefreshTokenStatusConsumed {
		key := lifecycleRefreshReceiptKey(presented.ID, mutation.Receipt.IdempotencyKey)
		if receipt := store.receipts[key]; receipt != nil && receipt.ExpiresAt.After(now) && receipt.ErasedAt == nil {
			replacement := store.refresh[receipt.ReplacementTokenID]
			return store.rotationResultLocked(
				entity.RefreshRotationRecovered,
				receipt,
				replacement,
			), nil
		}
		store.revokeIssuedLocked(presented.SessionID, now)
		session := store.sessions[presented.SessionID]
		reason := "refresh_replay"
		revokedAt := now.UTC()
		session.Status = entity.SessionStatusRevoked
		session.RevokedAt = &revokedAt
		session.RevokeReason = &reason
		return entity.RefreshRotationResult{
			Outcome: entity.RefreshRotationReplayContained, SessionID: presented.SessionID,
		}, nil
	}
	if presented.Status != entity.RefreshTokenStatusIssued {
		return entity.RefreshRotationResult{}, errors.New("refresh token is inactive")
	}
	replacement := cloneLifecycleRefresh(mutation.Replacement)
	receipt := cloneLifecycleRefreshReceipt(mutation.Receipt)
	store.refresh[replacement.ID] = replacement
	consumedAt := now.UTC()
	presented.Status = entity.RefreshTokenStatusConsumed
	presented.ConsumedAt = &consumedAt
	presented.ReplacedByID = &replacement.ID
	store.receipts[lifecycleRefreshReceiptKey(presented.ID, receipt.IdempotencyKey)] = receipt
	return store.rotationResultLocked(entity.RefreshRotationIssued, receipt, replacement), nil
}

func (store *lifecycleMemoryStore) MarkReceiptRecovered(
	_ context.Context,
	receiptID uuid.UUID,
	recoveredAt time.Time,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, receipt := range store.receipts {
		if receipt.ID == receiptID {
			at := recoveredAt.UTC()
			receipt.RecoveredAt = &at
			return nil
		}
	}
	return errors.New("refresh receipt not found")
}

func (store *lifecycleMemoryStore) Verify(
	context.Context,
	string,
	authentication.TokenPurpose,
) (authentication.Principal, error) {
	return authentication.Anonymous(), authentication.ErrInvalidToken
}

func (store *lifecycleMemoryStore) InvalidateSession(_ context.Context, sessionID uuid.UUID) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.invalidated = append(store.invalidated, sessionID)
	return nil
}

func (store *lifecycleMemoryStore) InvalidateUserSessions(_ context.Context, userID uuid.UUID) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.invalidatedUsers = append(store.invalidatedUsers, userID)
	return nil
}

func (store *lifecycleMemoryStore) refreshByHashLocked(tokenHash []byte) *entity.RefreshToken {
	for _, record := range store.refresh {
		if hmac.Equal(record.TokenHash, tokenHash) {
			return record
		}
	}
	return nil
}

func (store *lifecycleMemoryStore) revokeIssuedLocked(sessionID uuid.UUID, at time.Time) {
	for _, record := range store.refresh {
		if record.SessionID == sessionID && record.Status == entity.RefreshTokenStatusIssued {
			revokedAt := at.UTC()
			record.Status = entity.RefreshTokenStatusRevoked
			record.RevokedAt = &revokedAt
		}
	}
}

func (store *lifecycleMemoryStore) rotationResultLocked(
	outcome entity.RefreshRotationOutcome,
	receipt *entity.RefreshReceipt,
	replacement *entity.RefreshToken,
) entity.RefreshRotationResult {
	session := store.sessions[receipt.SessionID]
	owner := store.users[session.UserID]
	return entity.RefreshRotationResult{
		Outcome: outcome, ReceiptID: receipt.ID,
		ReplacementTokenID: replacement.ID, SessionID: session.ID,
		UserID: owner.id, Username: owner.username,
		SecurityVersion:    owner.securityVersion,
		AuthenticatedAt:    session.LastReauthenticatedAt,
		AbsoluteExpiresAt:  session.AbsoluteExpiresAt,
		IdleExpiresAt:      session.IdleExpiresAt,
		RefreshExpiresAt:   replacement.ExpiresAt,
		RecoveryCiphertext: append([]byte(nil), receipt.RecoveryCiphertext...),
	}
}

func TestTokenLifecycleEstablishmentIsHashedAtomicAndRecoverable(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	command := fixture.establishment()
	credentials, err := fixture.service.Establish(fixture.ctx, command)
	require.NoError(t, err)
	require.NotEmpty(t, credentials.AccessToken)
	require.NotEmpty(t, credentials.RefreshToken)

	fixture.store.mu.Lock()
	require.Len(t, fixture.store.sessions, 1)
	require.Len(t, fixture.store.refresh, 1)
	require.Len(t, fixture.store.issuance, 1)
	var persistedRefresh *entity.RefreshToken
	for _, record := range fixture.store.refresh {
		persistedRefresh = cloneLifecycleRefresh(record)
	}
	var persistedReceipt *entity.SessionIssuanceReceipt
	for _, record := range fixture.store.issuance {
		persistedReceipt = cloneLifecycleIssuance(record)
	}
	fixture.store.mu.Unlock()

	require.Len(t, persistedRefresh.TokenHash, 32)
	require.NotEqual(t, credentials.RefreshToken, string(persistedRefresh.TokenHash))
	require.NotContains(t, string(persistedReceipt.RecoveryCiphertext), credentials.RefreshToken)
	require.NotContains(t, string(persistedReceipt.RecoveryCiphertext), credentials.AccessToken)

	recovered, err := fixture.service.Establish(fixture.ctx, command)
	require.NoError(t, err)
	require.Equal(t, credentials, recovered)
	fixture.store.mu.Lock()
	require.Len(t, fixture.store.sessions, 1)
	require.Len(t, fixture.store.refresh, 1)
	fixture.store.mu.Unlock()

	conflict := command
	conflict.RequestFingerprint = []byte("different authenticated command")
	_, err = fixture.service.Establish(fixture.ctx, conflict)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}

func TestRegistrationReceiptRequiresAccountProofBeforeCredentialRecovery(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	command := fixture.establishment()
	command.Purpose = entity.SessionIssuancePasswordRegistration
	command.RequestOrTicketID = command.PreSessionID
	command.TraitOfferID = uuid.New()
	command.OfferChecksum = bytes.Repeat([]byte{0x5a}, 32)
	credentials, err := fixture.service.Establish(fixture.ctx, command)
	require.NoError(t, err)
	proofRejected := errors.New("password proof rejected")

	_, found, err := fixture.service.RecoverRegistration(
		fixture.ctx,
		command.RequestOrTicketID,
		command.IdempotencyKey,
		command.RequestFingerprint,
		func(_ context.Context, userID uuid.UUID) error {
			require.Equal(t, fixture.user.ID, userID)
			return proofRejected
		},
	)
	require.ErrorIs(t, err, proofRejected)
	require.False(t, found)
	fixture.store.mu.Lock()
	for _, receipt := range fixture.store.issuance {
		require.Nil(t, receipt.RecoveredAt)
	}
	fixture.store.mu.Unlock()

	recovered, found, err := fixture.service.RecoverRegistration(
		fixture.ctx,
		command.RequestOrTicketID,
		command.IdempotencyKey,
		command.RequestFingerprint,
		func(_ context.Context, userID uuid.UUID) error {
			require.Equal(t, fixture.user.ID, userID)
			return nil
		},
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, credentials, recovered)
}

func TestTokenLifecycleRotationRecoversAndContainsReplay(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	initial, err := fixture.service.Establish(fixture.ctx, fixture.establishment())
	require.NoError(t, err)

	idempotencyKey := uuid.New()
	rotated, err := fixture.service.Rotate(fixture.ctx, ports.RefreshRotation{
		RawRefreshToken: initial.RefreshToken, IdempotencyKey: idempotencyKey,
	})
	require.NoError(t, err)
	require.NotEqual(t, initial.RefreshToken, rotated.RefreshToken)
	require.Equal(t, initial.SessionID, rotated.SessionID)

	recovered, err := fixture.service.Rotate(fixture.ctx, ports.RefreshRotation{
		RawRefreshToken: initial.RefreshToken, IdempotencyKey: idempotencyKey,
	})
	require.NoError(t, err)
	require.Equal(t, rotated, recovered)

	_, err = fixture.service.Rotate(fixture.ctx, ports.RefreshRotation{
		RawRefreshToken: initial.RefreshToken, IdempotencyKey: uuid.New(),
	})
	require.ErrorIs(t, err, ErrRefreshReplay)
	fixture.store.mu.Lock()
	require.Equal(t, entity.SessionStatusRevoked, fixture.store.sessions[initial.SessionID].Status)
	require.Equal(t, []uuid.UUID{initial.SessionID}, fixture.store.invalidated)
	replacement := fixture.store.refreshByHashLocked(fixture.service.protector.hashRefresh(rotated.RefreshToken))
	require.NotNil(t, replacement)
	require.Equal(t, entity.RefreshTokenStatusRevoked, replacement.Status)
	fixture.store.mu.Unlock()

	_, err = fixture.service.Rotate(fixture.ctx, ports.RefreshRotation{
		RawRefreshToken: rotated.RefreshToken, IdempotencyKey: uuid.New(),
	})
	require.Error(t, err)
}

func TestTokenLifecycleConcurrentRetryConvergesOnOneReplacement(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	initial, err := fixture.service.Establish(fixture.ctx, fixture.establishment())
	require.NoError(t, err)
	idempotencyKey := uuid.New()

	results := make(chan entity.SessionCredentials, 2)
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, rotateErr := fixture.service.Rotate(fixture.ctx, ports.RefreshRotation{
				RawRefreshToken: initial.RefreshToken, IdempotencyKey: idempotencyKey,
			})
			results <- result
			errorsChannel <- rotateErr
		}()
	}
	workers.Wait()
	close(results)
	close(errorsChannel)
	for rotateErr := range errorsChannel {
		require.NoError(t, rotateErr)
	}
	var expected *entity.SessionCredentials
	for result := range results {
		if expected == nil {
			copied := result
			expected = &copied
			continue
		}
		require.Equal(t, *expected, result)
	}
	fixture.store.mu.Lock()
	require.Len(t, fixture.store.refresh, 2)
	require.Len(t, fixture.store.receipts, 1)
	fixture.store.mu.Unlock()
}

func TestTokenLifecycleRevocationUsesSessionThenCredentialLockOrder(t *testing.T) {
	t.Run("single session", func(t *testing.T) {
		fixture := newTokenLifecycleFixture(t)
		credentials, err := fixture.service.Establish(fixture.ctx, fixture.establishment())
		require.NoError(t, err)

		require.NoError(t, fixture.service.Logout(fixture.ctx, credentials.SessionID))
		fixture.store.mu.Lock()
		operations := append([]string(nil), fixture.store.revocationOps...)
		fixture.store.mu.Unlock()
		require.Equal(t, []string{"session", "refresh"}, operations)
	})

	t.Run("all user sessions", func(t *testing.T) {
		fixture := newTokenLifecycleFixture(t)
		_, err := fixture.service.Establish(fixture.ctx, fixture.establishment())
		require.NoError(t, err)

		require.NoError(t, fixture.service.RevokeAll(fixture.ctx, fixture.user.ID, "security_fence"))
		fixture.store.mu.Lock()
		operations := append([]string(nil), fixture.store.revocationOps...)
		fixture.store.mu.Unlock()
		require.Equal(t, []string{"sessions", "refreshes"}, operations)
	})
}

func TestTokenLifecycleRejectsUnsafeConfigurationAndCancellation(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	invalid := fixture.config
	invalid.RecoveryReceiptTTL = invalid.AccessTTL
	_, err := NewTokenLifecycleService(
		lifecycleTestIssuer{}, lifecycleTestTx{}, fixture.sessionRepository(), fixture.refreshRepository(),
		fixture.issuanceRepository(), fixture.store, fixture.store, invalid,
	)
	require.ErrorIs(t, err, ErrInvalidTokenLifecycle)
	placeholder := fixture.config
	placeholder.Secret = []byte("${JWT_SECRET}" + string(bytes.Repeat([]byte{'x'}, 32)))
	_, err = NewTokenLifecycleService(
		lifecycleTestIssuer{}, lifecycleTestTx{}, fixture.sessionRepository(), fixture.refreshRepository(),
		fixture.issuanceRepository(), fixture.store, fixture.store, placeholder,
	)
	require.ErrorIs(t, err, ErrInvalidTokenLifecycle)

	cancelled, cancel := context.WithCancel(fixture.ctx)
	cancel()
	_, err = fixture.service.Rotate(cancelled, ports.RefreshRotation{
		RawRefreshToken: "jrf1.invalid", IdempotencyKey: uuid.New(),
	})
	require.ErrorIs(t, err, ErrRefreshRejected)
}

func TestTokenLifecycleDefersUserCacheFenceUntilOutermostCommit(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	manager := &lifecycleDeferredTx{}
	service, err := NewTokenLifecycleService(
		lifecycleTestIssuer{}, manager, fixture.sessionRepository(), fixture.refreshRepository(),
		fixture.issuanceRepository(), fixture.store, fixture.store, fixture.config,
	)
	require.NoError(t, err)

	_, err = service.Establish(fixture.ctx, fixture.establishment())
	require.NoError(t, err)

	err = manager.DoInTx(fixture.ctx, func(txCtx context.Context) error {
		require.NoError(t, service.RevokeAll(txCtx, fixture.user.ID, "password_changed"))

		const concurrentReaders = 64
		var wait sync.WaitGroup
		var prematureFences atomic.Int64
		for range concurrentReaders {
			wait.Add(1)
			go func() {
				defer wait.Done()
				fixture.store.mu.Lock()
				if len(fixture.store.invalidatedUsers) != 0 {
					prematureFences.Add(1)
				}
				fixture.store.mu.Unlock()
			}()
		}
		wait.Wait()
		require.Zero(t, prematureFences.Load(), "cache fence ran before outer commit")
		return nil
	})
	require.NoError(t, err)

	fixture.store.mu.Lock()
	require.Equal(t, []uuid.UUID{fixture.user.ID}, fixture.store.invalidatedUsers)
	fixture.store.mu.Unlock()
}

func TestTokenLifecycleDropsUserCacheFenceOnOuterRollback(t *testing.T) {
	fixture := newTokenLifecycleFixture(t)
	manager := &lifecycleDeferredTx{}
	service, err := NewTokenLifecycleService(
		lifecycleTestIssuer{}, manager, fixture.sessionRepository(), fixture.refreshRepository(),
		fixture.issuanceRepository(), fixture.store, fixture.store, fixture.config,
	)
	require.NoError(t, err)

	rollbackErr := errors.New("later account mutation failed")
	err = manager.DoInTx(fixture.ctx, func(txCtx context.Context) error {
		require.NoError(t, service.RevokeAll(txCtx, fixture.user.ID, "account_deleted"))
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	fixture.store.mu.Lock()
	require.Empty(t, fixture.store.invalidatedUsers)
	fixture.store.mu.Unlock()
}

type tokenLifecycleFixture struct {
	ctx     context.Context
	user    *entity.User
	store   *lifecycleMemoryStore
	service *TokenLifecycleService
	config  TokenLifecycleConfig
}

func newTokenLifecycleFixture(t *testing.T) tokenLifecycleFixture {
	t.Helper()
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	user := &entity.User{
		ID: uuid.New(), Username: "session-user",
		SecurityStatus: entity.UserSecurityStatusActive, SecurityVersion: 7,
	}
	store := newLifecycleMemoryStore(user)
	config := TokenLifecycleConfig{
		AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour,
		SessionIdleTTL: 24 * time.Hour, SessionAbsoluteTTL: 30 * 24 * time.Hour,
		RecoveryReceiptTTL: 5 * time.Minute,
		Secret:             bytes.Repeat([]byte{0x42}, 32), SecretKeyID: "test-identity-v1",
		MaximumCredentialBytes: 4096, MaximumUserAgentBytes: 256,
		Clock: func() time.Time { return now },
	}
	service, err := NewTokenLifecycleService(
		lifecycleTestIssuer{}, lifecycleTestTx{}, lifecycleSessionRepository{store: store},
		lifecycleRefreshRepository{store: store}, lifecycleIssuanceRepository{store: store}, store, store, config,
	)
	require.NoError(t, err)
	return tokenLifecycleFixture{
		ctx:  correlation.WithContext(context.Background(), "identity-test-cid"),
		user: user, store: store, service: service, config: config,
	}
}

func (fixture tokenLifecycleFixture) establishment() ports.SessionEstablishment {
	return ports.SessionEstablishment{
		Purpose:        entity.SessionIssuancePasswordLogin,
		PreSessionID:   uuid.New(),
		IdempotencyKey: uuid.New(), User: fixture.user,
		RequestFingerprint: []byte("authenticated login command"),
		UserAgent:          "test-browser/1.0", IPHash: bytes.Repeat([]byte{0x24}, 32),
	}
}

func (fixture tokenLifecycleFixture) refreshRepository() ports.RefreshTokenRepository {
	return lifecycleRefreshRepository{store: fixture.store}
}

func (fixture tokenLifecycleFixture) sessionRepository() ports.SessionRepository {
	return lifecycleSessionRepository{store: fixture.store}
}

func (fixture tokenLifecycleFixture) issuanceRepository() ports.SessionIssuanceReceiptRepository {
	return lifecycleIssuanceRepository{store: fixture.store}
}

// Create has the same name on the session, refresh-token, and issuance ports.
// A narrow adapter keeps Go's method set unambiguous while all state remains
// in the one synchronized in-memory store.
type lifecycleRefreshRepository struct{ store *lifecycleMemoryStore }

func (repository lifecycleRefreshRepository) Create(ctx context.Context, token *entity.RefreshToken) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return repository.store.createValue(token)
}

type lifecycleSessionRepository struct{ store *lifecycleMemoryStore }

func (repository lifecycleSessionRepository) Create(ctx context.Context, session *entity.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return repository.store.createValue(session)
}
func (repository lifecycleSessionRepository) GetActiveForAuthentication(ctx context.Context, sessionID, userID uuid.UUID) (entity.SessionSecurityState, error) {
	return repository.store.GetActiveForAuthentication(ctx, sessionID, userID)
}
func (repository lifecycleSessionRepository) TouchLastSeen(ctx context.Context, sessionID uuid.UUID, observedAt, idleExpiresAt time.Time) (bool, error) {
	return repository.store.TouchLastSeen(ctx, sessionID, observedAt, idleExpiresAt)
}
func (repository lifecycleSessionRepository) Revoke(ctx context.Context, sessionID uuid.UUID, reason string, at time.Time) error {
	return repository.store.Revoke(ctx, sessionID, reason, at)
}
func (repository lifecycleSessionRepository) RevokeAll(ctx context.Context, userID uuid.UUID, reason string, at time.Time) error {
	return repository.store.RevokeAll(ctx, userID, reason, at)
}

type lifecycleIssuanceRepository struct{ store *lifecycleMemoryStore }

func (repository lifecycleIssuanceRepository) Get(ctx context.Context, selector ports.IssuanceReceiptSelector) (*entity.SessionIssuanceReceipt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return repository.store.getIssuance(selector)
}
func (repository lifecycleIssuanceRepository) Create(ctx context.Context, receipt *entity.SessionIssuanceReceipt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return repository.store.createValue(receipt)
}
func (repository lifecycleIssuanceRepository) MarkRecovered(ctx context.Context, id uuid.UUID, recoveredAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return repository.store.markIssuanceRecovered(id, recoveredAt)
}
func (repository lifecycleRefreshRepository) Get(ctx context.Context, id uuid.UUID) (*entity.RefreshToken, error) {
	return repository.store.GetRefresh(ctx, id)
}
func (repository lifecycleRefreshRepository) GetByHash(ctx context.Context, hash []byte) (*entity.RefreshToken, error) {
	return repository.store.GetByHash(ctx, hash)
}
func (repository lifecycleRefreshRepository) RevokeIssuedForSession(ctx context.Context, id uuid.UUID, at time.Time) error {
	return repository.store.RevokeIssuedForSession(ctx, id, at)
}
func (repository lifecycleRefreshRepository) RevokeIssuedForUser(ctx context.Context, id uuid.UUID, at time.Time) error {
	return repository.store.RevokeIssuedForUser(ctx, id, at)
}

func lifecycleIssuanceKey(selector ports.IssuanceReceiptSelector) string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s",
		selector.Purpose,
		selector.RequestID,
		selector.UserID,
		selector.PreSessionID,
		selector.IdempotencyKey,
	)
}

func lifecycleRefreshReceiptKey(tokenID, idempotencyKey uuid.UUID) string {
	return tokenID.String() + "|" + idempotencyKey.String()
}

func valueOrNilUUID(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}
	return *value
}

func cloneLifecycleSession(source *entity.Session) *entity.Session {
	if source == nil {
		return nil
	}
	result := *source
	result.IPHash = append([]byte(nil), source.IPHash...)
	result.UserAgent = copyLifecycleString(source.UserAgent)
	result.RevokedAt = copyLifecycleTime(source.RevokedAt)
	result.RevokeReason = copyLifecycleString(source.RevokeReason)
	return &result
}

func cloneLifecycleRefresh(source *entity.RefreshToken) *entity.RefreshToken {
	if source == nil {
		return nil
	}
	result := *source
	result.TokenHash = append([]byte(nil), source.TokenHash...)
	result.ParentID = copyLifecycleUUID(source.ParentID)
	result.ReplacedByID = copyLifecycleUUID(source.ReplacedByID)
	result.ConsumedAt = copyLifecycleTime(source.ConsumedAt)
	result.RevokedAt = copyLifecycleTime(source.RevokedAt)
	return &result
}

func cloneLifecycleIssuance(source *entity.SessionIssuanceReceipt) *entity.SessionIssuanceReceipt {
	if source == nil {
		return nil
	}
	result := *source
	result.RequestOrTicketID = copyLifecycleUUID(source.RequestOrTicketID)
	result.UserID = copyLifecycleUUID(source.UserID)
	result.PreSessionID = copyLifecycleUUID(source.PreSessionID)
	result.RequestFingerprint = append([]byte(nil), source.RequestFingerprint...)
	result.FingerprintKeyID = copyLifecycleString(source.FingerprintKeyID)
	result.RecoveryCiphertext = append([]byte(nil), source.RecoveryCiphertext...)
	result.RecoveredAt = copyLifecycleTime(source.RecoveredAt)
	result.ErasedAt = copyLifecycleTime(source.ErasedAt)
	return &result
}

func cloneLifecycleRefreshReceipt(source *entity.RefreshReceipt) *entity.RefreshReceipt {
	if source == nil {
		return nil
	}
	result := *source
	result.RecoveryCiphertext = append([]byte(nil), source.RecoveryCiphertext...)
	result.RecoveredAt = copyLifecycleTime(source.RecoveredAt)
	result.ErasedAt = copyLifecycleTime(source.ErasedAt)
	return &result
}

func copyLifecycleUUID(source *uuid.UUID) *uuid.UUID {
	if source == nil {
		return nil
	}
	result := *source
	return &result
}

func copyLifecycleString(source *string) *string {
	if source == nil {
		return nil
	}
	result := *source
	return &result
}

func copyLifecycleTime(source *time.Time) *time.Time {
	if source == nil {
		return nil
	}
	result := source.UTC()
	return &result
}

var (
	_ ports.RefreshRotationRepository        = (*lifecycleMemoryStore)(nil)
	_ ports.SessionAuthenticator             = (*lifecycleMemoryStore)(nil)
	_ ports.RefreshTokenRepository           = lifecycleRefreshRepository{}
	_ ports.SessionRepository                = lifecycleSessionRepository{}
	_ ports.SessionIssuanceReceiptRepository = lifecycleIssuanceRepository{}
)
