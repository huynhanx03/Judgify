package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/stretchr/testify/require"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type sessionManagementMemoryRepository struct {
	mu              sync.Mutex
	sessions        map[uuid.UUID]*entity.Session
	securityVersion map[uuid.UUID]uint64
	refreshActive   map[uuid.UUID]bool
	receipts        map[string]ports.SessionManagementMutation
	lastList        ports.SessionListQuery
}

func (repository *sessionManagementMemoryRepository) List(
	_ context.Context,
	query ports.SessionListQuery,
) (ports.SessionListResult, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.lastList = query
	items := make([]entity.Session, 0)
	for _, session := range repository.sessions {
		if session.UserID == query.UserID {
			items = append(items, *cloneManagedSession(session))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if query.After != nil {
		filtered := items[:0]
		for _, item := range items {
			if item.CreatedAt.Before(query.After.CreatedAt) ||
				(item.CreatedAt.Equal(query.After.CreatedAt) && item.ID.String() < query.After.ID.String()) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	result := ports.SessionListResult{Sessions: items}
	if len(items) > query.Limit {
		result.Sessions = items[:query.Limit]
		last := result.Sessions[len(result.Sessions)-1]
		result.Next = &ports.SessionCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return result, nil
}

func (repository *sessionManagementMemoryRepository) RevokeOwned(
	_ context.Context,
	command ports.SessionRevokeCommand,
	now, _ time.Time,
) (ports.SessionManagementMutation, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	key := command.UserID.String() + "/" + command.IdempotencyKey.String()
	if receipt, ok := repository.receipts[key]; ok {
		if receipt.ActorSessionID != command.ActorSessionID || receipt.TargetSessionID == nil ||
			*receipt.TargetSessionID != command.TargetSessionID || receipt.Kind != ports.SessionCommandRevoke {
			return ports.SessionManagementMutation{}, ports.ErrSessionCommandIdempotencyConflict
		}
		receipt.Applied = false
		return receipt, nil
	}
	session, ok := repository.sessions[command.TargetSessionID]
	if !ok || session.UserID != command.UserID {
		return ports.SessionManagementMutation{}, ports.ErrOwnedSessionNotFound
	}
	target := command.TargetSessionID
	result := ports.SessionManagementMutation{
		Kind:           ports.SessionCommandRevoke,
		ActorSessionID: command.ActorSessionID, TargetSessionID: &target,
	}
	switch session.Status {
	case entity.SessionStatusRevoked, entity.SessionStatusExpired:
		return result, nil
	case entity.SessionStatusActive:
		result.MutationID = uuid.New()
		at := now.UTC()
		reason := "user_revoked"
		session.Status = entity.SessionStatusRevoked
		session.RevokedAt = &at
		session.RevokeReason = &reason
		repository.refreshActive[session.ID] = false
		result.Applied = true
	default:
		return ports.SessionManagementMutation{}, ports.ErrInvalidSessionManagementPersistence
	}
	repository.receipts[key] = result
	return result, nil
}

func (repository *sessionManagementMemoryRepository) LogoutAll(
	_ context.Context,
	command ports.SessionLogoutAllCommand,
	now, _ time.Time,
) (ports.SessionManagementMutation, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	key := command.UserID.String() + "/" + command.IdempotencyKey.String()
	if receipt, ok := repository.receipts[key]; ok {
		if receipt.ActorSessionID != command.ActorSessionID || receipt.TargetSessionID != nil ||
			receipt.Kind != ports.SessionCommandLogoutAll {
			return ports.SessionManagementMutation{}, ports.ErrSessionCommandIdempotencyConflict
		}
		receipt.Applied = false
		return receipt, nil
	}
	version := repository.securityVersion[command.UserID] + 1
	repository.securityVersion[command.UserID] = version
	for _, session := range repository.sessions {
		if session.UserID != command.UserID || session.Status != entity.SessionStatusActive {
			continue
		}
		at := now.UTC()
		reason := "user_logout_all"
		session.Status = entity.SessionStatusRevoked
		session.RevokedAt = &at
		session.RevokeReason = &reason
		repository.refreshActive[session.ID] = false
	}
	result := ports.SessionManagementMutation{
		MutationID: uuid.New(), Kind: ports.SessionCommandLogoutAll,
		ActorSessionID: command.ActorSessionID, SecurityVersion: version, Applied: true,
	}
	repository.receipts[key] = result
	return result, nil
}

type sessionManagementSnapshotTx struct {
	repository *sessionManagementMemoryRepository
}

func (manager sessionManagementSnapshotTx) DoInTx(ctx context.Context, fn func(context.Context) error) error {
	manager.repository.mu.Lock()
	sessions := make(map[uuid.UUID]*entity.Session, len(manager.repository.sessions))
	for id, session := range manager.repository.sessions {
		sessions[id] = cloneManagedSession(session)
	}
	versions := cloneManagedMap(manager.repository.securityVersion)
	refresh := cloneManagedMap(manager.repository.refreshActive)
	receipts := cloneManagedMap(manager.repository.receipts)
	manager.repository.mu.Unlock()
	if err := fn(ctx); err != nil {
		manager.repository.mu.Lock()
		manager.repository.sessions = sessions
		manager.repository.securityVersion = versions
		manager.repository.refreshActive = refresh
		manager.repository.receipts = receipts
		manager.repository.mu.Unlock()
		return err
	}
	return nil
}

type sessionInvalidationCapture struct {
	sessions []uuid.UUID
	users    []uuid.UUID
}

func (capture *sessionInvalidationCapture) InvalidateSession(_ context.Context, id uuid.UUID) error {
	capture.sessions = append(capture.sessions, id)
	return nil
}

func (capture *sessionInvalidationCapture) InvalidateUser(_ context.Context, id uuid.UUID) error {
	capture.users = append(capture.users, id)
	return nil
}

func TestSessionManagementListsBoundedCanonicalSafeViews(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	currentID := uuid.New()
	otherID := uuid.New()
	userAgent := "Mozilla/5.0 test browser"
	reason := "internal reason"
	repository := newSessionManagementMemoryRepository(userID, currentID, otherID, now)
	repository.sessions[otherID].UserAgent = &userAgent
	repository.sessions[otherID].IPHash = []byte("must-never-leave-the-service")
	repository.sessions[otherID].Status = entity.SessionStatusRevoked
	revokedAt := now.Add(-30 * time.Second)
	repository.sessions[otherID].RevokedAt = &revokedAt
	repository.sessions[otherID].RevokeReason = &reason
	service := newSessionManagementFixture(t, repository, now, &securityMutationJournalCapture{}).service

	page, err := service.List(context.Background(), userID, currentID, &dto.SessionListRequest{})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, 20, repository.lastList.Limit)
	require.True(t, page.Items[0].Current)
	require.Equal(t, currentID, page.Items[0].ID)

	encoded, err := json.Marshal(page)
	require.NoError(t, err)
	for _, forbidden := range []string{"ip_hash", "family_id", "revoke_reason", "must-never-leave"} {
		require.NotContains(t, string(encoded), forbidden)
	}

	_, err = service.List(context.Background(), userID, currentID, &dto.SessionListRequest{Limit: 51})
	requireAppErrorCode(t, err, apperr.CodeBadRequest)
	_, err = service.List(context.Background(), userID, currentID, &dto.SessionListRequest{Cursor: "not-a-cursor"})
	requireAppErrorCode(t, err, apperr.CodeBadRequest)
}

func TestSessionManagementCursorIsDeterministicAndBounded(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 123456789, time.UTC)
	cursor := ports.SessionCursor{CreatedAt: now, ID: uuid.New()}
	encoded, err := encodeSessionCursor(cursor)
	require.NoError(t, err)
	decoded, err := decodeSessionCursor(encoded)
	require.NoError(t, err)
	require.Equal(t, cursor, decoded)
	require.LessOrEqual(t, len(encoded), 64)

	for _, malformed := range []string{"", encoded + "=", encoded[:len(encoded)-1], "AAAA"} {
		_, err := decodeSessionCursor(malformed)
		require.Error(t, err, malformed)
	}
}

func TestSessionManagementRevokeIsOwnedIdempotentAndJournaled(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	actorID := uuid.New()
	targetID := uuid.New()
	repository := newSessionManagementMemoryRepository(userID, actorID, targetID, now)
	journal := &securityMutationJournalCapture{}
	fixture := newSessionManagementFixture(t, repository, now, journal)
	key := uuid.New()

	err := fixture.service.Revoke(context.Background(), userID, actorID, targetID, key)
	require.NoError(t, err)
	require.Equal(t, entity.SessionStatusRevoked, repository.sessions[targetID].Status)
	require.False(t, repository.refreshActive[targetID])
	require.Equal(t, []uuid.UUID{targetID}, fixture.invalidator.sessions)
	mutations := journal.snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationSessionRevoked, mutations[0].Kind)
	require.NotNil(t, mutations[0].SessionID)
	require.Equal(t, actorID, *mutations[0].SessionID)
	require.NotNil(t, mutations[0].TargetSessionID)
	require.Equal(t, targetID, *mutations[0].TargetSessionID)

	err = fixture.service.Revoke(context.Background(), userID, actorID, targetID, key)
	require.NoError(t, err)
	require.Len(t, journal.snapshot(), 1)
	require.Len(t, fixture.invalidator.sessions, 1)

	otherUserSession := uuid.New()
	repository.sessions[otherUserSession] = &entity.Session{ID: otherUserSession, UserID: uuid.New(), Status: entity.SessionStatusActive}
	err = fixture.service.Revoke(context.Background(), userID, actorID, otherUserSession, uuid.New())
	requireAppErrorCode(t, err, apperr.CodeNotFound)
	require.Equal(t, entity.SessionStatusActive, repository.sessions[otherUserSession].Status)
	err = fixture.service.Revoke(context.Background(), userID, actorID, uuid.New(), uuid.New())
	requireAppErrorCode(t, err, apperr.CodeNotFound)
}

func TestSessionManagementTerminalTargetIsAReceiptFreeNoOp(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	actorID := uuid.New()
	targetID := uuid.New()
	repository := newSessionManagementMemoryRepository(userID, actorID, targetID, now)
	revokedAt := now.Add(-time.Minute)
	reason := "already_revoked"
	repository.sessions[targetID].Status = entity.SessionStatusRevoked
	repository.sessions[targetID].RevokedAt = &revokedAt
	repository.sessions[targetID].RevokeReason = &reason
	repository.refreshActive[targetID] = false
	journal := &securityMutationJournalCapture{}
	fixture := newSessionManagementFixture(t, repository, now, journal)

	require.NoError(t, fixture.service.Revoke(
		context.Background(), userID, actorID, targetID, uuid.New(),
	))
	require.Empty(t, repository.receipts)
	require.Empty(t, journal.snapshot())
	require.Empty(t, fixture.invalidator.sessions)
}

func TestSessionManagementJournalFailureRollsBackSecurityMutation(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	actorID := uuid.New()
	targetID := uuid.New()
	repository := newSessionManagementMemoryRepository(userID, actorID, targetID, now)
	journal := &securityMutationJournalCapture{err: errors.New("audit unavailable")}
	fixture := newSessionManagementFixture(t, repository, now, journal)

	err := fixture.service.Revoke(context.Background(), userID, actorID, targetID, uuid.New())
	requireAppErrorCode(t, err, apperr.CodeInternalServer)
	require.Equal(t, entity.SessionStatusActive, repository.sessions[targetID].Status)
	require.True(t, repository.refreshActive[targetID])
	require.Empty(t, repository.receipts)
	require.Empty(t, fixture.invalidator.sessions)
}

func TestSessionManagementLogoutAllAdvancesSecurityFenceExactlyOnce(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()
	actorID := uuid.New()
	otherID := uuid.New()
	repository := newSessionManagementMemoryRepository(userID, actorID, otherID, now)
	repository.securityVersion[userID] = 8
	journal := &securityMutationJournalCapture{}
	fixture := newSessionManagementFixture(t, repository, now, journal)
	key := uuid.New()

	err := fixture.service.LogoutAll(context.Background(), userID, actorID, key)
	require.NoError(t, err)
	require.Equal(t, uint64(9), repository.securityVersion[userID])
	require.False(t, repository.refreshActive[actorID])
	require.False(t, repository.refreshActive[otherID])
	require.Equal(t, []uuid.UUID{userID}, fixture.invalidator.users)
	mutations := journal.snapshot()
	require.Len(t, mutations, 1)
	require.Equal(t, entity.SecurityMutationSessionsRevokedAll, mutations[0].Kind)
	require.Equal(t, uint64(9), mutations[0].SecurityVersion)
	require.Nil(t, mutations[0].TargetSessionID)

	err = fixture.service.LogoutAll(context.Background(), userID, actorID, key)
	require.NoError(t, err)
	require.Equal(t, uint64(9), repository.securityVersion[userID])
	require.Len(t, journal.snapshot(), 1)
	require.Len(t, fixture.invalidator.users, 1)
}

type sessionManagementFixture struct {
	service     *SessionManagementService
	invalidator *sessionInvalidationCapture
}

func newSessionManagementFixture(
	t *testing.T,
	repository *sessionManagementMemoryRepository,
	now time.Time,
	journal ports.SecurityMutationJournal,
) sessionManagementFixture {
	t.Helper()
	invalidator := &sessionInvalidationCapture{}
	service, err := NewSessionManagementService(
		repository,
		sessionManagementSnapshotTx{repository: repository},
		journal,
		invalidator,
		SessionManagementConfig{Clock: func() time.Time { return now }, ReceiptTTL: 24 * time.Hour},
	)
	require.NoError(t, err)
	return sessionManagementFixture{service: service, invalidator: invalidator}
}

func newSessionManagementMemoryRepository(
	userID, firstID, secondID uuid.UUID,
	now time.Time,
) *sessionManagementMemoryRepository {
	return &sessionManagementMemoryRepository{
		sessions: map[uuid.UUID]*entity.Session{
			firstID: {
				ID: firstID, UserID: userID, FamilyID: uuid.New(), Status: entity.SessionStatusActive,
				CreatedAt: now.Add(-time.Minute), LastSeenAt: now.Add(-time.Second),
				LastReauthenticatedAt: now.Add(-time.Minute), IdleExpiresAt: now.Add(time.Hour),
				AbsoluteExpiresAt: now.Add(24 * time.Hour),
			},
			secondID: {
				ID: secondID, UserID: userID, FamilyID: uuid.New(), Status: entity.SessionStatusActive,
				CreatedAt: now.Add(-2 * time.Minute), LastSeenAt: now.Add(-time.Minute),
				LastReauthenticatedAt: now.Add(-2 * time.Minute), IdleExpiresAt: now.Add(time.Hour),
				AbsoluteExpiresAt: now.Add(24 * time.Hour),
			},
		},
		securityVersion: map[uuid.UUID]uint64{userID: 1},
		refreshActive:   map[uuid.UUID]bool{firstID: true, secondID: true},
		receipts:        make(map[string]ports.SessionManagementMutation),
	}
}

func cloneManagedSession(session *entity.Session) *entity.Session {
	if session == nil {
		return nil
	}
	copy := *session
	copy.IPHash = append([]byte(nil), session.IPHash...)
	if session.UserAgent != nil {
		value := *session.UserAgent
		copy.UserAgent = &value
	}
	if session.RevokeReason != nil {
		value := *session.RevokeReason
		copy.RevokeReason = &value
	}
	if session.RevokedAt != nil {
		value := *session.RevokedAt
		copy.RevokedAt = &value
	}
	return &copy
}

func cloneManagedMap[K comparable, V any](source map[K]V) map[K]V {
	result := make(map[K]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
