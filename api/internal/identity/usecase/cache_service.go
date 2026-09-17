package usecase

import (
	"container/list"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/cache"
	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	defaultPermissionConfigVersion = 1
	defaultSessionCacheMaximumTTL  = 30 * time.Second
	defaultSessionCacheEntries     = 20_000
	maximumSessionCacheEntries     = 100_000
	defaultSessionCacheLoadTimeout = 2 * time.Second
	maximumSessionCacheLoadTimeout = 10 * time.Second
	maximumSessionTokenIDBytes     = 256
	maximumResolveAttempts         = 2
)

var (
	errInvalidSessionCache     = errors.New("identity: invalid session cache configuration")
	errSessionCacheInvalidated = errors.New("identity: session cache invalidated during load")
)

type cacheService struct {
	cache cache.LocalCache[string, any]
}

// SessionStateCacheConfig configures the defensive process-local session
// cache. Zero values select production defaults.
type SessionStateCacheConfig struct {
	MaximumTTL     time.Duration
	MaximumEntries int
	LoadTimeout    time.Duration
	Clock          func() time.Time
}

type sessionCacheEntry struct {
	state           entity.SessionSecurityState
	accessExpiresAt time.Time
	validUntil      time.Time
	element         *list.Element
}

// sessionStateCache is a memory- and time-bounded LRU. It is intentionally
// separate from general entity caching because authentication invalidation
// needs exact session/user indexes and no stale-while-revalidate behavior.
type sessionStateCache struct {
	mu             sync.Mutex
	entries        map[ports.SessionCacheKey]*sessionCacheEntry
	bySession      map[uuid.UUID]map[ports.SessionCacheKey]struct{}
	byUser         map[uuid.UUID]map[ports.SessionCacheKey]struct{}
	recency        *list.List
	maximumTTL     time.Duration
	maximumEntries int
	loadTimeout    time.Duration
	clock          func() time.Time
	generation     uint64
	loads          singleflight.Group
}

// NewCacheService creates a new CacheService instance.
func NewCacheService(cache cache.LocalCache[string, any]) ports.CacheService {
	return &cacheService{cache: cache}
}

// NewSessionStateCache builds the session-specific revalidation cache.
func NewSessionStateCache(config SessionStateCacheConfig) (ports.SessionStateCache, error) {
	if config.MaximumTTL == 0 {
		config.MaximumTTL = defaultSessionCacheMaximumTTL
	}
	if config.MaximumEntries == 0 {
		config.MaximumEntries = defaultSessionCacheEntries
	}
	if config.LoadTimeout == 0 {
		config.LoadTimeout = defaultSessionCacheLoadTimeout
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.MaximumTTL < 0 || config.MaximumTTL > defaultSessionCacheMaximumTTL ||
		config.MaximumEntries < 1 || config.MaximumEntries > maximumSessionCacheEntries ||
		config.LoadTimeout < 0 ||
		config.LoadTimeout > maximumSessionCacheLoadTimeout {
		return nil, errInvalidSessionCache
	}
	return &sessionStateCache{
		entries:        make(map[ports.SessionCacheKey]*sessionCacheEntry),
		bySession:      make(map[uuid.UUID]map[ports.SessionCacheKey]struct{}),
		byUser:         make(map[uuid.UUID]map[ports.SessionCacheKey]struct{}),
		recency:        list.New(),
		maximumTTL:     config.MaximumTTL,
		maximumEntries: config.MaximumEntries,
		loadTimeout:    config.LoadTimeout,
		clock:          config.Clock,
	}, nil
}

// InvalidatePermissionConfig increments the permission config version.
func (s *cacheService) InvalidatePermissionConfig(ctx context.Context) error {
	version, _ := s.GetPermissionConfigVersion(ctx)
	newVersion := version + 1
	// No TTL: this is the invalidation anchor for permission caches, not a
	// cached entity — it must persist so a bump is never silently forgotten.
	cache.Set(s.cache, constant.CacheKeyPermissionConfigVersion, newVersion)
	return nil
}

// GetPermissionConfigVersion retrieves the current permission config version.
func (s *cacheService) GetPermissionConfigVersion(ctx context.Context) (int64, error) {
	if v, found := cache.Get[int64](s.cache, constant.CacheKeyPermissionConfigVersion); found {
		return v, nil
	}

	return defaultPermissionConfigVersion, nil
}

func (sessionCache *sessionStateCache) Resolve(
	ctx context.Context,
	key ports.SessionCacheKey,
	accessExpiresAt time.Time,
	loader ports.SessionStateLoader,
) (entity.SessionSecurityState, error) {
	if err := validateSessionCacheRequest(ctx, key, accessExpiresAt, loader); err != nil {
		return entity.SessionSecurityState{}, err
	}
	accessExpiresAt = accessExpiresAt.UTC()
	for attempt := 0; attempt < maximumResolveAttempts; attempt++ {
		if state, found := sessionCache.lookup(key, accessExpiresAt); found {
			return state, nil
		}

		result := sessionCache.loads.DoChan(sessionCacheLoadKey(key), func() (any, error) {
			if state, found := sessionCache.lookup(key, accessExpiresAt); found {
				return state, nil
			}
			// The database read is shared by every waiter for this credential.
			// Detach only cancellation/deadline from the singleflight leader so
			// one disconnected request cannot fail healthy peers; values such as
			// CID remain attached, and this internal operation is still bounded.
			loadContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionCache.loadTimeout)
			defer cancel()
			return sessionCache.load(loadContext, key, accessExpiresAt, loader)
		})
		select {
		case <-ctx.Done():
			return entity.SessionSecurityState{}, ctx.Err()
		case outcome := <-result:
			if errors.Is(outcome.Err, errSessionCacheInvalidated) {
				sessionCache.loads.Forget(sessionCacheLoadKey(key))
				continue
			}
			if outcome.Err != nil {
				return entity.SessionSecurityState{}, outcome.Err
			}
			state, ok := outcome.Val.(entity.SessionSecurityState)
			if !ok {
				return entity.SessionSecurityState{}, errInvalidSessionCache
			}
			return state, nil
		}
	}
	return entity.SessionSecurityState{}, errSessionCacheInvalidated
}

func (sessionCache *sessionStateCache) lookup(
	key ports.SessionCacheKey,
	accessExpiresAt time.Time,
) (entity.SessionSecurityState, bool) {
	now := sessionCache.clock().UTC()
	sessionCache.mu.Lock()
	defer sessionCache.mu.Unlock()
	entry, found := sessionCache.entries[key]
	if !found {
		return entity.SessionSecurityState{}, false
	}
	// A JTI's expiry is immutable. If a caller presents contradictory verified
	// metadata, discard rather than widening or narrowing an existing entry.
	if !entry.accessExpiresAt.Equal(accessExpiresAt) || !entry.validUntil.After(now) {
		sessionCache.removeLocked(key)
		return entity.SessionSecurityState{}, false
	}
	sessionCache.recency.MoveToFront(entry.element)
	return entry.state, true
}

func (sessionCache *sessionStateCache) load(
	ctx context.Context,
	key ports.SessionCacheKey,
	accessExpiresAt time.Time,
	loader ports.SessionStateLoader,
) (entity.SessionSecurityState, error) {
	sessionCache.mu.Lock()
	generation := sessionCache.generation
	sessionCache.mu.Unlock()

	state, err := loader(ctx)
	if err != nil {
		return entity.SessionSecurityState{}, err
	}
	now := sessionCache.clock().UTC()
	validUntil := earliestTime(
		now.Add(sessionCache.maximumTTL),
		accessExpiresAt,
		state.IdleExpiresAt.UTC(),
		state.AbsoluteExpiresAt.UTC(),
	)

	sessionCache.mu.Lock()
	defer sessionCache.mu.Unlock()
	if generation != sessionCache.generation {
		return entity.SessionSecurityState{}, errSessionCacheInvalidated
	}
	if validUntil.After(now) {
		sessionCache.storeLocked(key, state, accessExpiresAt, validUntil)
	}
	return state, nil
}

func (sessionCache *sessionStateCache) InvalidateSession(_ context.Context, sessionID uuid.UUID) error {
	if sessionCache == nil || sessionID == uuid.Nil {
		return errInvalidSessionCache
	}
	sessionCache.mu.Lock()
	defer sessionCache.mu.Unlock()
	sessionCache.generation++
	for key := range sessionCache.bySession[sessionID] {
		sessionCache.removeLocked(key)
	}
	return nil
}

func (sessionCache *sessionStateCache) InvalidateUser(_ context.Context, userID uuid.UUID) error {
	if sessionCache == nil || userID == uuid.Nil {
		return errInvalidSessionCache
	}
	sessionCache.mu.Lock()
	defer sessionCache.mu.Unlock()
	sessionCache.generation++
	for key := range sessionCache.byUser[userID] {
		sessionCache.removeLocked(key)
	}
	return nil
}

// MarkTouched mirrors the durable last-seen/sliding-idle update. A failed or
// conditional no-op touch never calls this method, so cache state cannot run
// ahead of PostgreSQL.
func (sessionCache *sessionStateCache) MarkTouched(
	sessionID uuid.UUID,
	observedAt, idleExpiresAt time.Time,
) {
	if sessionCache == nil || sessionID == uuid.Nil || observedAt.IsZero() ||
		idleExpiresAt.IsZero() || !idleExpiresAt.After(observedAt) {
		return
	}
	observedAt = observedAt.UTC()
	idleExpiresAt = idleExpiresAt.UTC()
	sessionCache.mu.Lock()
	defer sessionCache.mu.Unlock()
	for key := range sessionCache.bySession[sessionID] {
		entry := sessionCache.entries[key]
		if entry != nil && observedAt.After(entry.state.LastSeenAt) &&
			!idleExpiresAt.After(entry.state.AbsoluteExpiresAt) {
			entry.state.LastSeenAt = observedAt
			entry.state.IdleExpiresAt = idleExpiresAt
		}
	}
}

func (sessionCache *sessionStateCache) storeLocked(
	key ports.SessionCacheKey,
	state entity.SessionSecurityState,
	accessExpiresAt, validUntil time.Time,
) {
	if existing := sessionCache.entries[key]; existing != nil {
		existing.state = state
		existing.accessExpiresAt = accessExpiresAt
		existing.validUntil = validUntil
		sessionCache.recency.MoveToFront(existing.element)
		return
	}
	for len(sessionCache.entries) >= sessionCache.maximumEntries {
		oldest := sessionCache.recency.Back()
		if oldest == nil {
			break
		}
		sessionCache.removeLocked(oldest.Value.(ports.SessionCacheKey))
	}
	element := sessionCache.recency.PushFront(key)
	sessionCache.entries[key] = &sessionCacheEntry{
		state:           state,
		accessExpiresAt: accessExpiresAt,
		validUntil:      validUntil,
		element:         element,
	}
	addCacheIndex(sessionCache.bySession, key.SessionID, key)
	addCacheIndex(sessionCache.byUser, key.UserID, key)
}

func (sessionCache *sessionStateCache) removeLocked(key ports.SessionCacheKey) {
	entry := sessionCache.entries[key]
	if entry == nil {
		return
	}
	delete(sessionCache.entries, key)
	sessionCache.recency.Remove(entry.element)
	removeCacheIndex(sessionCache.bySession, key.SessionID, key)
	removeCacheIndex(sessionCache.byUser, key.UserID, key)
}

func validateSessionCacheRequest(
	ctx context.Context,
	key ports.SessionCacheKey,
	accessExpiresAt time.Time,
	loader ports.SessionStateLoader,
) error {
	if ctx == nil || key.SessionID == uuid.Nil || key.UserID == uuid.Nil ||
		key.TokenID == "" || len(key.TokenID) > maximumSessionTokenIDBytes ||
		key.SecurityVersion == 0 || accessExpiresAt.IsZero() || loader == nil {
		return errInvalidSessionCache
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func sessionCacheLoadKey(key ports.SessionCacheKey) string {
	tokenIDHash := sha256.Sum256([]byte(key.TokenID))
	return fmt.Sprintf(
		"%s:%s:%d:%x",
		key.SessionID.String(),
		key.UserID.String(),
		key.SecurityVersion,
		tokenIDHash,
	)
}

func earliestTime(values ...time.Time) time.Time {
	var earliest time.Time
	for _, value := range values {
		if value.IsZero() {
			continue
		}
		if earliest.IsZero() || value.Before(earliest) {
			earliest = value
		}
	}
	return earliest
}

func addCacheIndex(
	index map[uuid.UUID]map[ports.SessionCacheKey]struct{},
	id uuid.UUID,
	key ports.SessionCacheKey,
) {
	keys := index[id]
	if keys == nil {
		keys = make(map[ports.SessionCacheKey]struct{})
		index[id] = keys
	}
	keys[key] = struct{}{}
}

func removeCacheIndex(
	index map[uuid.UUID]map[ports.SessionCacheKey]struct{},
	id uuid.UUID,
	key ports.SessionCacheKey,
) {
	keys := index[id]
	delete(keys, key)
	if len(keys) == 0 {
		delete(index, id)
	}
}

var _ ports.SessionStateCache = (*sessionStateCache)(nil)
