package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type sessionTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *sessionTestClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *sessionTestClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

type sessionTestVerifier struct {
	mu        sync.Mutex
	principal func(raw string) (authentication.Principal, error)
	calls     int
	purposes  []authentication.TokenPurpose
}

func (verifier *sessionTestVerifier) Verify(
	_ context.Context,
	raw string,
	purpose authentication.TokenPurpose,
) (authentication.Principal, error) {
	verifier.mu.Lock()
	verifier.calls++
	verifier.purposes = append(verifier.purposes, purpose)
	fn := verifier.principal
	verifier.mu.Unlock()
	return fn(raw)
}

func (verifier *sessionTestVerifier) CallCount() int {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()
	return verifier.calls
}

type sessionTestRepository struct {
	mu          sync.Mutex
	state       entity.SessionSecurityState
	loadErr     error
	loads       int
	touches     int
	touchErr    error
	touchNoop   bool
	touchSignal chan lastSeenTouch
	blockFirst  chan struct{}
	firstLoaded chan struct{}
}

func (*sessionTestRepository) Create(context.Context, *entity.Session) error { return nil }

func (repository *sessionTestRepository) GetActiveForAuthentication(
	ctx context.Context,
	_, _ uuid.UUID,
) (entity.SessionSecurityState, error) {
	repository.mu.Lock()
	repository.loads++
	loadNumber := repository.loads
	state := repository.state
	err := repository.loadErr
	block := repository.blockFirst
	started := repository.firstLoaded
	repository.mu.Unlock()
	if loadNumber == 1 && block != nil {
		if started != nil {
			close(started)
		}
		select {
		case <-block:
		case <-ctx.Done():
			return entity.SessionSecurityState{}, ctx.Err()
		}
	}
	return state, err
}

func (repository *sessionTestRepository) TouchLastSeen(
	ctx context.Context,
	sessionID uuid.UUID,
	observedAt, idleExpiresAt time.Time,
) (bool, error) {
	repository.mu.Lock()
	repository.touches++
	err := repository.touchErr
	noop := repository.touchNoop
	signal := repository.touchSignal
	repository.mu.Unlock()
	if signal != nil {
		select {
		case signal <- lastSeenTouch{
			sessionID:     sessionID,
			observedAt:    observedAt,
			idleExpiresAt: idleExpiresAt,
			cid:           correlation.FromContext(ctx),
		}:
		default:
		}
	}
	return err == nil && !noop, err
}

func (*sessionTestRepository) Revoke(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}

func (*sessionTestRepository) RevokeAll(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}

func (repository *sessionTestRepository) LoadCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.loads
}

func (repository *sessionTestRepository) TouchCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.touches
}

type sessionFixture struct {
	now       time.Time
	clock     *sessionTestClock
	userID    uuid.UUID
	sessionID uuid.UUID
	principal authentication.Principal
	state     entity.SessionSecurityState
}

func newSessionFixture() sessionFixture {
	now := time.Date(2026, 7, 19, 4, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()
	return sessionFixture{
		now:       now,
		clock:     &sessionTestClock{now: now},
		userID:    userID,
		sessionID: sessionID,
		principal: authentication.Principal{
			Authenticated:      true,
			Subject:            userID.String(),
			UserID:             userID.String(),
			SessionID:          sessionID.String(),
			Username:           "stale-display-name",
			TokenID:            uuid.NewString(),
			AuthenticatedAt:    now.Add(-time.Hour),
			SecurityVersion:    7,
			AuthenticatedUntil: now.Add(15 * time.Minute),
		},
		state: entity.SessionSecurityState{
			SessionID:              sessionID,
			UserID:                 userID,
			SessionStatus:          entity.SessionStatusActive,
			SessionSecurityVersion: 7,
			UserStatus:             entity.UserSecurityStatusActive,
			UserSecurityVersion:    7,
			Username:               "canonical-name",
			LastSeenAt:             now,
			LastReauthenticatedAt:  now.Add(-time.Hour),
			IdleExpiresAt:          now.Add(30 * 24 * time.Hour),
			AbsoluteExpiresAt:      now.Add(90 * 24 * time.Hour),
		},
	}
}

func newSessionAuthenticatorFixture(
	t *testing.T,
	fixture sessionFixture,
	repository *sessionTestRepository,
	config SessionAuthenticatorConfig,
) (*SessionAuthenticator, *sessionStateCache, *sessionTestVerifier) {
	t.Helper()
	stateCachePort, err := NewSessionStateCache(SessionStateCacheConfig{
		MaximumTTL:     30 * time.Second,
		MaximumEntries: 32,
		Clock:          fixture.clock.Now,
	})
	require.NoError(t, err)
	stateCache := stateCachePort.(*sessionStateCache)
	verifier := &sessionTestVerifier{principal: func(string) (authentication.Principal, error) {
		return fixture.principal, nil
	}}
	config.Clock = fixture.clock.Now
	if config.TouchMinimumInterval == 0 {
		config.TouchMinimumInterval = time.Hour
	}
	authenticator, err := NewSessionAuthenticator(verifier, repository, stateCache, config)
	require.NoError(t, err)
	return authenticator, stateCache, verifier
}

func TestSessionAuthenticatorActiveUsesEarliestBoundary(t *testing.T) {
	fixture := newSessionFixture()
	fixture.state.IdleExpiresAt = fixture.now.Add(10 * time.Minute)
	fixture.state.AbsoluteExpiresAt = fixture.now.Add(12 * time.Minute)
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

	principal, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.NoError(t, err)
	require.True(t, principal.Authenticated)
	require.Equal(t, fixture.state.IdleExpiresAt, principal.AuthenticatedUntil)
	require.Equal(t, fixture.state.Username, principal.Username)
	require.Equal(t, fixture.userID.String(), principal.Subject)
	require.Equal(t, fixture.userID.String(), principal.UserID)
	require.Equal(t, fixture.sessionID.String(), principal.SessionID)
	require.Equal(t, fixture.state.LastReauthenticatedAt, principal.AuthenticatedAt)
}

func TestSessionAuthenticatorPreservesCredentialAuthenticationTimeBelowCanonicalState(t *testing.T) {
	fixture := newSessionFixture()
	fixture.principal.AuthenticatedAt = fixture.now.Add(-2 * time.Hour)
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

	principal, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.NoError(t, err)
	require.Equal(t, fixture.principal.AuthenticatedAt, principal.AuthenticatedAt)
}

func TestSessionAuthenticatorRejectsAuthenticationTimeAheadOfCanonicalSession(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*sessionFixture)
	}{
		{
			name: "token claims a later authentication",
			mutate: func(fixture *sessionFixture) {
				fixture.principal.AuthenticatedAt = fixture.state.LastReauthenticatedAt.Add(time.Second)
			},
		},
		{
			name: "canonical session authentication time is in the future",
			mutate: func(fixture *sessionFixture) {
				fixture.state.LastReauthenticatedAt = fixture.now.Add(time.Second)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSessionFixture()
			test.mutate(&fixture)
			repository := &sessionTestRepository{state: fixture.state}
			authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

			principal, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
			require.Equal(t, authentication.Anonymous(), principal)
			require.ErrorIs(t, err, ErrAuthenticationTimeMismatch)
		})
	}
}

func TestSessionAuthenticatorRejectsInactiveExpiredAndVersionDrift(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*sessionFixture)
		wantErr error
	}{
		{"idle expired", func(f *sessionFixture) { f.state.IdleExpiresAt = f.now }, ErrSessionExpired},
		{"absolute expired", func(f *sessionFixture) { f.state.AbsoluteExpiresAt = f.now.Add(-time.Second) }, ErrSessionExpired},
		{"invalid expiry order", func(f *sessionFixture) { f.state.IdleExpiresAt = f.state.AbsoluteExpiresAt.Add(time.Second) }, ErrSessionExpired},
		{"revoked", func(f *sessionFixture) { f.state.SessionStatus = entity.SessionStatusRevoked }, ErrSessionInactive},
		{"expired status", func(f *sessionFixture) { f.state.SessionStatus = entity.SessionStatusExpired }, ErrSessionInactive},
		{"suspended user", func(f *sessionFixture) { f.state.UserStatus = entity.UserSecurityStatusSuspended }, ErrUserInactive},
		{"deleted user status", func(f *sessionFixture) { f.state.UserStatus = entity.UserSecurityStatusDeleted }, ErrUserInactive},
		{"soft deleted user", func(f *sessionFixture) { f.state.UserDeleted = true }, ErrUserInactive},
		{"session version", func(f *sessionFixture) { f.state.SessionSecurityVersion++ }, ErrSecurityVersionMismatch},
		{"user version", func(f *sessionFixture) { f.state.UserSecurityVersion++ }, ErrSecurityVersionMismatch},
		{"session owner", func(f *sessionFixture) { f.state.UserID = uuid.New() }, ErrSessionInactive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSessionFixture()
			test.mutate(&fixture)
			repository := &sessionTestRepository{state: fixture.state}
			authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
			principal, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
			require.Equal(t, authentication.Anonymous(), principal)
			require.ErrorIs(t, err, authentication.ErrInvalidToken)
			require.ErrorIs(t, err, test.wantErr)
		})
	}
}

func TestSessionAuthenticatorCryptographicVerificationPrecedesUUIDParsing(t *testing.T) {
	fixture := newSessionFixture()
	fixture.principal.Subject = "not-a-uuid"
	fixture.principal.UserID = "not-a-uuid"
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, verifier := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

	_, err := authenticator.Verify(context.Background(), "signed-but-invalid-identity", authentication.PurposeAccess)
	require.ErrorIs(t, err, authentication.ErrInvalidToken)
	require.Equal(t, 1, verifier.CallCount(), "cryptographic verifier must run before UUID parsing")
	require.Zero(t, repository.LoadCount())
}

func TestSessionAuthenticatorRequiresExactCanonicalSubjectUserUUID(t *testing.T) {
	fixture := newSessionFixture()
	fixture.principal.UserID = uuid.NewString()
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	_, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.ErrorIs(t, err, authentication.ErrInvalidToken)

	fixture = newSessionFixture()
	fixture.principal.Subject = "{" + fixture.userID.String() + "}"
	fixture.principal.UserID = fixture.principal.Subject
	repository = &sessionTestRepository{state: fixture.state}
	authenticator, _, _ = newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	_, err = authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.ErrorIs(t, err, authentication.ErrInvalidToken)
}

func TestSessionAuthenticatorRequiresCanonicalUUIDTokenID(t *testing.T) {
	fixture := newSessionFixture()
	fixture.principal.TokenID = "not-a-jti"
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, verifier := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	_, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.ErrorIs(t, err, authentication.ErrInvalidToken)
	require.Equal(t, 1, verifier.CallCount())
	require.Zero(t, repository.LoadCount())

	fixture = newSessionFixture()
	fixture.principal.TokenID = "{" + fixture.principal.TokenID + "}"
	repository = &sessionTestRepository{state: fixture.state}
	authenticator, _, _ = newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	_, err = authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.ErrorIs(t, err, authentication.ErrInvalidToken)
}

func TestSessionAuthenticatorSeparatesAccessAndRefreshPurposes(t *testing.T) {
	fixture := newSessionFixture()
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, verifier := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

	_, err := authenticator.Verify(context.Background(), "access", authentication.PurposeRefresh)
	require.ErrorIs(t, err, authentication.ErrPurposeMismatch)
	require.Zero(t, verifier.CallCount(), "refresh purpose must not enter access cryptographic verification")

	verifier.mu.Lock()
	verifier.principal = func(string) (authentication.Principal, error) {
		return authentication.Anonymous(), authentication.ErrPurposeMismatch
	}
	verifier.mu.Unlock()
	_, err = authenticator.Verify(context.Background(), "refresh-token", authentication.PurposeAccess)
	require.ErrorIs(t, err, authentication.ErrPurposeMismatch)
	require.Zero(t, repository.LoadCount())
}

func TestSessionAuthenticatorCacheMissHitTTLAndCredentialKey(t *testing.T) {
	fixture := newSessionFixture()
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, stateCache, verifier := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

	_, err := authenticator.Verify(context.Background(), "first", authentication.PurposeAccess)
	require.NoError(t, err)
	_, err = authenticator.Verify(context.Background(), "first", authentication.PurposeAccess)
	require.NoError(t, err)
	require.Equal(t, 1, repository.LoadCount())

	stateCache.mu.Lock()
	require.Len(t, stateCache.entries, 1)
	for _, entry := range stateCache.entries {
		require.False(t, entry.validUntil.After(fixture.now.Add(30*time.Second)))
		require.False(t, entry.validUntil.After(fixture.principal.AuthenticatedUntil))
	}
	stateCache.mu.Unlock()

	fixture.clock.Advance(31 * time.Second)
	_, err = authenticator.Verify(context.Background(), "first", authentication.PurposeAccess)
	require.NoError(t, err)
	require.Equal(t, 2, repository.LoadCount())

	verifier.mu.Lock()
	second := fixture.principal
	second.TokenID = uuid.NewString()
	verifier.principal = func(raw string) (authentication.Principal, error) {
		if raw == "second" {
			return second, nil
		}
		return fixture.principal, nil
	}
	verifier.mu.Unlock()
	_, err = authenticator.Verify(context.Background(), "second", authentication.PurposeAccess)
	require.NoError(t, err)
	require.Equal(t, 3, repository.LoadCount(), "different JTI must have a distinct bounded entry")
}

func TestSessionAuthenticatorCacheNeverOutlivesAccessToken(t *testing.T) {
	fixture := newSessionFixture()
	fixture.principal.AuthenticatedUntil = fixture.now.Add(5 * time.Second)
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, stateCache, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	require.NoError(t, verifySession(authenticator))

	stateCache.mu.Lock()
	require.Len(t, stateCache.entries, 1)
	for _, entry := range stateCache.entries {
		require.Equal(t, fixture.principal.AuthenticatedUntil, entry.validUntil)
	}
	stateCache.mu.Unlock()
}

func TestSessionAuthenticatorSynchronousInvalidation(t *testing.T) {
	fixture := newSessionFixture()
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	require.NoError(t, verifySession(authenticator))
	require.NoError(t, verifySession(authenticator))
	require.Equal(t, 1, repository.LoadCount())

	// A failed durable mutation does not call the post-commit hook, so the
	// existing entry remains available.
	require.NoError(t, verifySession(authenticator))
	require.Equal(t, 1, repository.LoadCount())

	require.NoError(t, authenticator.InvalidateSession(context.Background(), fixture.sessionID))
	require.NoError(t, verifySession(authenticator))
	require.Equal(t, 2, repository.LoadCount())

	require.NoError(t, authenticator.InvalidateUserSessions(context.Background(), fixture.userID))
	require.NoError(t, verifySession(authenticator))
	require.Equal(t, 3, repository.LoadCount())
}

func TestSessionAuthenticatorInvalidationWinsAgainstInflightLoad(t *testing.T) {
	fixture := newSessionFixture()
	repository := &sessionTestRepository{
		state:       fixture.state,
		blockFirst:  make(chan struct{}),
		firstLoaded: make(chan struct{}),
	}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	done := make(chan error, 1)
	go func() { done <- verifySession(authenticator) }()
	<-repository.firstLoaded
	require.NoError(t, authenticator.InvalidateSession(context.Background(), fixture.sessionID))
	close(repository.blockFirst)
	require.NoError(t, <-done)
	require.Equal(t, 2, repository.LoadCount(), "post-invalidation state must be reloaded")
}

func TestSessionAuthenticatorPostgresOutageFailsClosedOnMiss(t *testing.T) {
	fixture := newSessionFixture()
	databaseErr := errors.New("postgres unavailable")
	repository := &sessionTestRepository{state: fixture.state, loadErr: databaseErr}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})

	for range 2 {
		principal, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
		require.Equal(t, authentication.Anonymous(), principal)
		require.ErrorIs(t, err, authentication.ErrInvalidToken)
		require.ErrorIs(t, err, ErrSessionStateUnavailable)
		require.ErrorIs(t, err, databaseErr)
	}
	require.Equal(t, 2, repository.LoadCount(), "database failures must never become positive cache entries")
}

func TestSessionAuthenticatorPostgresOutageCannotExtendCachedWindow(t *testing.T) {
	fixture := newSessionFixture()
	repository := &sessionTestRepository{state: fixture.state}
	authenticator, _, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{})
	require.NoError(t, verifySession(authenticator))

	repository.mu.Lock()
	repository.loadErr = errors.New("postgres unavailable")
	repository.mu.Unlock()
	require.NoError(t, verifySession(authenticator), "a still-valid defensive cache hit may authenticate")
	require.Equal(t, 1, repository.LoadCount())

	fixture.clock.Advance(31 * time.Second)
	principal, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	require.Equal(t, authentication.Anonymous(), principal)
	require.ErrorIs(t, err, ErrSessionStateUnavailable)
	require.Equal(t, 2, repository.LoadCount())
}

func TestSessionAuthenticatorBoundedLastSeenTouchLifecycle(t *testing.T) {
	fixture := newSessionFixture()
	fixture.state.LastSeenAt = fixture.now.Add(-2 * time.Minute)
	fixture.state.IdleExpiresAt = fixture.state.LastSeenAt.Add(30 * 24 * time.Hour)
	repository := &sessionTestRepository{
		state:       fixture.state,
		touchSignal: make(chan lastSeenTouch, 2),
	}
	authenticator, stateCache, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{
		TouchWorkers:         1,
		TouchQueueCapacity:   1,
		TouchTimeout:         time.Second,
		TouchMinimumInterval: time.Minute,
		TouchGateEntries:     8,
	})
	runContext, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- authenticator.Run(runContext) }()
	requireAuthenticatorRunning(t, authenticator)

	requestContext := correlation.WithContext(context.Background(), "touch-test-cid")
	_, err := authenticator.Verify(requestContext, "access", authentication.PurposeAccess)
	require.NoError(t, err)
	select {
	case touch := <-repository.touchSignal:
		require.Equal(t, fixture.sessionID, touch.sessionID)
		require.Equal(t, fixture.now, touch.observedAt)
		require.Equal(t, fixture.now.Add(30*24*time.Hour), touch.idleExpiresAt)
		require.Equal(t, "touch-test-cid", touch.cid)
	case <-time.After(time.Second):
		t.Fatal("last-seen touch was not processed")
	}
	require.Equal(t, 1, repository.TouchCount())

	stateCache.mu.Lock()
	for _, entry := range stateCache.entries {
		require.Equal(t, fixture.now, entry.state.LastSeenAt)
		require.Equal(t, fixture.now.Add(30*24*time.Hour), entry.state.IdleExpiresAt)
	}
	stateCache.mu.Unlock()

	cancelRun()
	require.NoError(t, <-runDone)
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
	defer cancelShutdown()
	require.NoError(t, authenticator.Shutdown(shutdownContext))
	require.NoError(t, authenticator.Shutdown(shutdownContext))
}

func TestSessionAuthenticatorFailedTouchDoesNotMutateCachedState(t *testing.T) {
	fixture := newSessionFixture()
	originalLastSeen := fixture.now.Add(-2 * time.Minute)
	fixture.state.LastSeenAt = originalLastSeen
	fixture.state.IdleExpiresAt = originalLastSeen.Add(30 * 24 * time.Hour)
	repository := &sessionTestRepository{
		state:       fixture.state,
		touchErr:    errors.New("touch failed"),
		touchSignal: make(chan lastSeenTouch, 1),
	}
	authenticator, stateCache, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{
		TouchWorkers:         1,
		TouchQueueCapacity:   1,
		TouchMinimumInterval: time.Minute,
	})
	runContext, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- authenticator.Run(runContext) }()
	requireAuthenticatorRunning(t, authenticator)
	require.NoError(t, verifySession(authenticator))
	select {
	case <-repository.touchSignal:
	case <-time.After(time.Second):
		t.Fatal("failed touch was not attempted")
	}

	stateCache.mu.Lock()
	for _, entry := range stateCache.entries {
		require.Equal(t, originalLastSeen, entry.state.LastSeenAt)
		require.Equal(t, fixture.state.IdleExpiresAt, entry.state.IdleExpiresAt)
	}
	stateCache.mu.Unlock()
	cancelRun()
	require.NoError(t, <-runDone)
}

func TestSessionAuthenticatorNoopTouchInvalidatesCachedState(t *testing.T) {
	fixture := newSessionFixture()
	originalLastSeen := fixture.now.Add(-2 * time.Minute)
	fixture.state.LastSeenAt = originalLastSeen
	fixture.state.IdleExpiresAt = originalLastSeen.Add(30 * 24 * time.Hour)
	repository := &sessionTestRepository{
		state:       fixture.state,
		touchNoop:   true,
		touchSignal: make(chan lastSeenTouch, 1),
	}
	authenticator, stateCache, _ := newSessionAuthenticatorFixture(t, fixture, repository, SessionAuthenticatorConfig{
		TouchWorkers:         1,
		TouchQueueCapacity:   1,
		TouchMinimumInterval: time.Minute,
	})
	runContext, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- authenticator.Run(runContext) }()
	requireAuthenticatorRunning(t, authenticator)
	require.NoError(t, verifySession(authenticator))
	select {
	case <-repository.touchSignal:
	case <-time.After(time.Second):
		t.Fatal("no-op touch was not attempted")
	}

	require.Eventually(t, func() bool {
		stateCache.mu.Lock()
		defer stateCache.mu.Unlock()
		return len(stateCache.entries) == 0
	}, time.Second, time.Millisecond)
	cancelRun()
	require.NoError(t, <-runDone)
}

func TestSessionAuthenticatorCacheCapacityIsBounded(t *testing.T) {
	fixture := newSessionFixture()
	stateCachePort, err := NewSessionStateCache(SessionStateCacheConfig{
		MaximumTTL:     30 * time.Second,
		MaximumEntries: 2,
		Clock:          fixture.clock.Now,
	})
	require.NoError(t, err)
	stateCache := stateCachePort.(*sessionStateCache)
	for index := 0; index < 3; index++ {
		key := ports.SessionCacheKey{
			SessionID:       uuid.New(),
			UserID:          uuid.New(),
			TokenID:         uuid.NewString(),
			SecurityVersion: 1,
		}
		state := fixture.state
		state.SessionID = key.SessionID
		state.UserID = key.UserID
		state.SessionSecurityVersion = 1
		state.UserSecurityVersion = 1
		_, err := stateCache.Resolve(
			context.Background(),
			key,
			fixture.now.Add(time.Minute),
			func(context.Context) (entity.SessionSecurityState, error) { return state, nil },
		)
		require.NoError(t, err)
	}
	stateCache.mu.Lock()
	require.Len(t, stateCache.entries, 2)
	require.LessOrEqual(t, len(stateCache.bySession), 2)
	require.LessOrEqual(t, len(stateCache.byUser), 2)
	stateCache.mu.Unlock()
}

func TestSessionAuthenticatorRejectsInvalidConstruction(t *testing.T) {
	_, err := NewSessionStateCache(SessionStateCacheConfig{MaximumTTL: 31 * time.Second})
	require.ErrorIs(t, err, errInvalidSessionCache)
	_, err = NewSessionStateCache(SessionStateCacheConfig{LoadTimeout: 11 * time.Second})
	require.ErrorIs(t, err, errInvalidSessionCache)
	_, err = NewSessionStateCache(SessionStateCacheConfig{MaximumEntries: maximumSessionCacheEntries + 1})
	require.ErrorIs(t, err, errInvalidSessionCache)
	_, err = NewSessionAuthenticator(nil, nil, nil, SessionAuthenticatorConfig{})
	require.ErrorIs(t, err, ErrInvalidSessionAuthenticator)
}

func verifySession(authenticator *SessionAuthenticator) error {
	_, err := authenticator.Verify(context.Background(), "access", authentication.PurposeAccess)
	return err
}

func requireAuthenticatorRunning(t *testing.T, authenticator *SessionAuthenticator) {
	t.Helper()
	require.Eventually(t, func() bool {
		authenticator.lifecycleMu.Lock()
		defer authenticator.lifecycleMu.Unlock()
		return authenticator.state == authenticatorRunning
	}, time.Second, time.Millisecond)
}

var (
	_ ports.AccessTokenVerifier = (*sessionTestVerifier)(nil)
	_ ports.SessionRepository   = (*sessionTestRepository)(nil)
)
