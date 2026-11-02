package usecase

import (
	"container/list"
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
)

const (
	authorizationMembershipCacheCapacity = 4096
	authorizationResourceCacheCapacity   = 1024
)

type authorizationRelationKey struct {
	revision   int64
	scope      string
	userID     uuid.UUID
	resourceID string
}

type authorizationRelationEntry struct {
	key       authorizationRelationKey
	relations []authcasbin.PolicyRule
}

// authorizationRelationCache keeps hot memberships and object relations apart
// from tenant policy runtimes. Membership entries are shared across resource
// IDs; all entries are immutable after insert.
type authorizationRelationCache struct {
	memberships *authorizationRelationLRU
	resources   *authorizationRelationLRU
}

type authorizationRelationLRU struct {
	mu       sync.Mutex
	capacity int
	order    *list.List
	entries  map[authorizationRelationKey]*list.Element
}

func newAuthorizationRelationCache(membershipCapacity, resourceCapacity int) *authorizationRelationCache {
	return &authorizationRelationCache{
		memberships: newAuthorizationRelationLRU(membershipCapacity),
		resources:   newAuthorizationRelationLRU(resourceCapacity),
	}
}

func newAuthorizationRelationLRU(capacity int) *authorizationRelationLRU {
	return &authorizationRelationLRU{
		capacity: capacity,
		order:    list.New(),
		entries:  make(map[authorizationRelationKey]*list.Element, capacity),
	}
}

func (cache *authorizationRelationLRU) get(key authorizationRelationKey) ([]authcasbin.PolicyRule, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	element := cache.entries[key]
	if element == nil {
		return nil, false
	}
	cache.order.MoveToFront(element)
	return element.Value.(*authorizationRelationEntry).relations, true
}

func (cache *authorizationRelationLRU) store(
	key authorizationRelationKey,
	relations []authcasbin.PolicyRule,
) {
	entry := &authorizationRelationEntry{
		key:       key,
		relations: append([]authcasbin.PolicyRule(nil), relations...),
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if element := cache.entries[key]; element != nil {
		element.Value = entry
		cache.order.MoveToFront(element)
		return
	}
	cache.entries[key] = cache.order.PushFront(entry)
	if cache.order.Len() > cache.capacity {
		cache.remove(cache.order.Back())
	}
}

func (cache *authorizationRelationLRU) invalidateAll() {
	cache.mu.Lock()
	cache.entries = make(map[authorizationRelationKey]*list.Element, cache.capacity)
	cache.order.Init()
	cache.mu.Unlock()
}

func (cache *authorizationRelationLRU) remove(element *list.Element) {
	if element == nil {
		return
	}
	entry := element.Value.(*authorizationRelationEntry)
	delete(cache.entries, entry.key)
	cache.order.Remove(element)
}

func (cache *authorizationRelationCache) invalidateAll() {
	cache.memberships.invalidateAll()
	cache.resources.invalidateAll()
}

func (service *authorizationService) subjectRelations(
	ctx context.Context,
	scope authorization.Scope,
	userID uuid.UUID,
	resourceID string,
	revision int64,
) ([]authcasbin.PolicyRule, error) {
	if service == nil || service.runtime == nil || service.repository == nil || service.relationCache == nil ||
		ctx == nil || ctx.Err() != nil || scope.Validate() != nil || userID == uuid.Nil || revision <= 0 {
		return nil, authorization.ErrInvalidRequest
	}
	if service.runtime.AppliedRevision() != revision || service.runtime.Healthy() != nil {
		return nil, authcasbin.ErrUnhealthy
	}
	membershipKey := authorizationRelationKey{
		revision: revision,
		scope:    scope.Key(),
		userID:   userID,
	}
	memberships, found := service.relationCache.memberships.get(membershipKey)
	if !found {
		var err error
		memberships, err = service.repository.LoadSubjectRelations(
			ctx,
			revision,
			scope,
			userSubject(userID.String()),
		)
		if err != nil {
			return nil, err
		}
		if err := service.relationLoadCurrent(ctx, revision); err != nil {
			return nil, err
		}
		service.relationCache.memberships.store(membershipKey, memberships)
	}
	if resourceID == "" {
		if err := service.relationSnapshotCurrent(revision); err != nil {
			return nil, err
		}
		return memberships, nil
	}
	resourceKey := membershipKey
	resourceKey.resourceID = resourceID
	resourceRelations, found := service.relationCache.resources.get(resourceKey)
	if !found {
		var err error
		resourceRelations, err = service.repository.LoadResourceRelations(
			ctx,
			revision,
			scope,
			userSubject(userID.String()),
			resourceID,
		)
		if err != nil {
			return nil, err
		}
		if err := service.relationLoadCurrent(ctx, revision); err != nil {
			return nil, err
		}
		service.relationCache.resources.store(resourceKey, resourceRelations)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := service.relationSnapshotCurrent(revision); err != nil {
		return nil, err
	}
	combined := make([]authcasbin.PolicyRule, 0, len(memberships)+len(resourceRelations))
	combined = append(combined, memberships...)
	combined = append(combined, resourceRelations...)
	return combined, nil
}

func (service *authorizationService) relationSnapshotCurrent(revision int64) error {
	if service.runtime.AppliedRevision() != revision || service.runtime.Healthy() != nil {
		return authcasbin.ErrUnhealthy
	}
	return nil
}

func (service *authorizationService) relationLoadCurrent(ctx context.Context, revision int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return service.relationSnapshotCurrent(revision)
}
