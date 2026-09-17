package db

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const sessionRepoName = "authentication session"

const sessionIPHashBytes = 32

// SessionRepository is the Ent-backed canonical session store.
type SessionRepository struct {
	client *dbent.EntClient
}

func NewSessionRepository(client *dbent.EntClient) ports.RecoveryContactSessionStore {
	return &SessionRepository{client: client}
}

func (repository *SessionRepository) Create(ctx context.Context, session *entity.Session) error {
	if repository == nil || repository.client == nil || ctx == nil || session == nil {
		return fmt.Errorf("%s: invalid create input", sessionRepoName)
	}
	if err := validateSessionForCreate(session); err != nil {
		return err
	}
	securityVersion, err := persistedSecurityVersion(session.UserSecurityVersion)
	if err != nil {
		return err
	}
	status := session.Status
	if status == "" {
		status = entity.SessionStatusActive
	}
	revokeReason := session.RevokeReason
	if revokeReason != nil {
		normalized, normalizeErr := normalizeRevokeReason(*revokeReason)
		if normalizeErr != nil {
			return normalizeErr
		}
		revokeReason = &normalized
	}

	create := repository.client.DB(ctx).AuthSession.Create().
		SetID(session.ID).
		SetUserID(session.UserID).
		SetFamilyID(session.FamilyID).
		SetStatus(authsession.Status(status)).
		SetUserSecurityVersion(securityVersion).
		SetLastSeenAt(session.LastSeenAt.UTC()).
		SetLastReauthenticatedAt(session.LastReauthenticatedAt.UTC()).
		SetIdleExpiresAt(session.IdleExpiresAt.UTC()).
		SetAbsoluteExpiresAt(session.AbsoluteExpiresAt.UTC()).
		SetNillableRevokedAt(utcTimePointer(session.RevokedAt)).
		SetNillableRevokeReason(revokeReason).
		SetNillableUserAgent(session.UserAgent).
		SetCreatedCid(session.CreatedCID)
	if len(session.IPHash) > 0 {
		create.SetIPHash(append([]byte(nil), session.IPHash...))
	}
	if !session.CreatedAt.IsZero() {
		create.SetCreatedAt(session.CreatedAt.UTC())
	}
	if !session.UpdatedAt.IsZero() {
		create.SetUpdatedAt(session.UpdatedAt.UTC())
	}

	record, err := create.Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, sessionRepoName)
	}
	mapSession(record, session)
	return nil
}

func (repository *SessionRepository) GetActiveForAuthentication(
	ctx context.Context,
	sessionID, userID uuid.UUID,
) (entity.SessionSecurityState, error) {
	if repository == nil || repository.client == nil || ctx == nil || sessionID == uuid.Nil || userID == uuid.Nil {
		return entity.SessionSecurityState{}, fmt.Errorf("%s: invalid authentication lookup", sessionRepoName)
	}

	record, err := repository.client.DB(ctx).AuthSession.Query().
		Where(authsession.ID(sessionID), authsession.UserID(userID)).
		Select(
			authsession.FieldID,
			authsession.FieldUserID,
			authsession.FieldStatus,
			authsession.FieldUserSecurityVersion,
			authsession.FieldLastSeenAt,
			authsession.FieldLastReauthenticatedAt,
			authsession.FieldIdleExpiresAt,
			authsession.FieldAbsoluteExpiresAt,
		).
		WithUser(func(query *generate.UserQuery) {
			query.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldStatus,
				user.FieldSecurityVersion,
				user.FieldDeletedAt,
			)
		}).
		Only(ctx)
	if err != nil {
		return entity.SessionSecurityState{}, commonent.MapEntError(err, sessionRepoName)
	}
	owner, err := record.Edges.UserOrErr()
	if err != nil {
		return entity.SessionSecurityState{}, commonent.MapEntError(err, sessionRepoName)
	}
	if record.UserSecurityVersion < 1 || owner.SecurityVersion < 1 {
		return entity.SessionSecurityState{}, fmt.Errorf("%s: invalid persisted security version", sessionRepoName)
	}

	return entity.SessionSecurityState{
		SessionID:              record.ID,
		UserID:                 record.UserID,
		SessionStatus:          entity.SessionStatus(record.Status),
		SessionSecurityVersion: uint64(record.UserSecurityVersion),
		UserStatus:             entity.UserSecurityStatus(owner.Status),
		UserSecurityVersion:    uint64(owner.SecurityVersion),
		UserDeleted:            owner.DeletedAt != nil,
		Username:               owner.Username,
		LastSeenAt:             record.LastSeenAt.UTC(),
		LastReauthenticatedAt:  record.LastReauthenticatedAt.UTC(),
		IdleExpiresAt:          record.IdleExpiresAt.UTC(),
		AbsoluteExpiresAt:      record.AbsoluteExpiresAt.UTC(),
	}, nil
}

func (repository *SessionRepository) TouchLastSeen(
	ctx context.Context,
	sessionID uuid.UUID,
	observedAt, idleExpiresAt time.Time,
) (bool, error) {
	if repository == nil || repository.client == nil || ctx == nil || sessionID == uuid.Nil ||
		observedAt.IsZero() || idleExpiresAt.IsZero() {
		return false, fmt.Errorf("%s: invalid last-seen touch", sessionRepoName)
	}
	observedAt = observedAt.UTC()
	idleExpiresAt = idleExpiresAt.UTC()
	if !idleExpiresAt.After(observedAt) {
		return false, fmt.Errorf("%s: invalid sliding idle expiry", sessionRepoName)
	}
	updated, err := repository.client.DB(ctx).AuthSession.Update().
		Where(
			authsession.ID(sessionID),
			authsession.StatusEQ(authsession.StatusActive),
			authsession.LastSeenAtLT(observedAt),
			authsession.IdleExpiresAtGT(observedAt),
			authsession.IdleExpiresAtLTE(idleExpiresAt),
			authsession.AbsoluteExpiresAtGT(observedAt),
			authsession.AbsoluteExpiresAtGTE(idleExpiresAt),
		).
		SetLastSeenAt(observedAt).
		SetIdleExpiresAt(idleExpiresAt).
		Save(ctx)
	if err != nil {
		return false, commonent.MapEntError(err, sessionRepoName)
	}
	return updated == 1, nil
}

func (repository *SessionRepository) MarkReauthenticated(
	ctx context.Context,
	sessionID, userID uuid.UUID,
	at time.Time,
) (bool, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		sessionID == uuid.Nil || userID == uuid.Nil || at.IsZero() {
		return false, fmt.Errorf("%s: invalid reauthentication", sessionRepoName)
	}
	at = at.UTC()
	updated, err := repository.client.DB(ctx).AuthSession.Update().
		Where(
			authsession.ID(sessionID),
			authsession.UserID(userID),
			authsession.StatusEQ(authsession.StatusActive),
			authsession.IdleExpiresAtGT(at),
			authsession.AbsoluteExpiresAtGT(at),
			authsession.LastReauthenticatedAtLTE(at),
		).
		SetLastReauthenticatedAt(at).
		Save(ctx)
	if err != nil {
		return false, commonent.MapEntError(err, sessionRepoName)
	}
	return updated == 1, nil
}

func (repository *SessionRepository) Revoke(
	ctx context.Context,
	sessionID uuid.UUID,
	reason string,
	at time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil || sessionID == uuid.Nil {
		return fmt.Errorf("%s: invalid revoke", sessionRepoName)
	}
	reason, err := normalizeRevokeReason(reason)
	if err != nil {
		return err
	}
	at = at.UTC()
	if at.IsZero() {
		return fmt.Errorf("%s: invalid revoke time", sessionRepoName)
	}
	_, err = repository.client.DB(ctx).AuthSession.Update().
		Where(authsession.ID(sessionID), authsession.StatusEQ(authsession.StatusActive)).
		SetStatus(authsession.StatusRevoked).
		SetRevokedAt(at).
		SetRevokeReason(reason).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, sessionRepoName)
	}
	return nil
}

func (repository *SessionRepository) RevokeAll(
	ctx context.Context,
	userID uuid.UUID,
	reason string,
	at time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return fmt.Errorf("%s: invalid revoke-all", sessionRepoName)
	}
	reason, err := normalizeRevokeReason(reason)
	if err != nil {
		return err
	}
	at = at.UTC()
	if at.IsZero() {
		return fmt.Errorf("%s: invalid revoke time", sessionRepoName)
	}
	client := repository.client.DB(ctx)
	// Lock the physical account row, including an already soft-deleted row. A
	// concurrent delete may have passed its active-row existence check before
	// another transaction commits the soft delete. Keeping this security fence
	// idempotent lets the final UserRepository.Delete map that race to NotFound
	// while preserving the canonical user -> sessions -> tokens lock order.
	lockContext := commonent.SkipSoftDelete(ctx)
	if _, err = client.User.Query().
		Where(user.ID(userID)).
		Select(user.FieldID).
		ForUpdate().
		Only(lockContext); err != nil {
		return commonent.MapEntError(err, sessionRepoName)
	}
	_, err = client.AuthSession.Update().
		Where(authsession.UserID(userID), authsession.StatusEQ(authsession.StatusActive)).
		SetStatus(authsession.StatusRevoked).
		SetRevokedAt(at).
		SetRevokeReason(reason).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, sessionRepoName)
	}
	return nil
}

func mapSession(record *generate.AuthSession, destination *entity.Session) {
	if record == nil || destination == nil {
		return
	}
	destination.ID = record.ID
	destination.UserID = record.UserID
	destination.FamilyID = record.FamilyID
	destination.Status = entity.SessionStatus(record.Status)
	destination.UserSecurityVersion = uint64(record.UserSecurityVersion)
	destination.CreatedAt = record.CreatedAt.UTC()
	destination.UpdatedAt = record.UpdatedAt.UTC()
	destination.LastSeenAt = record.LastSeenAt.UTC()
	destination.LastReauthenticatedAt = record.LastReauthenticatedAt.UTC()
	destination.IdleExpiresAt = record.IdleExpiresAt.UTC()
	destination.AbsoluteExpiresAt = record.AbsoluteExpiresAt.UTC()
	destination.RevokedAt = utcTimePointer(record.RevokedAt)
	destination.RevokeReason = copyStringPointer(record.RevokeReason)
	if record.IPHash != nil {
		destination.IPHash = append([]byte(nil), (*record.IPHash)...)
	}
	destination.UserAgent = copyStringPointer(record.UserAgent)
	destination.CreatedCID = record.CreatedCid
}

func persistedSecurityVersion(version uint64) (int64, error) {
	if version == 0 || version > math.MaxInt64 {
		return 0, fmt.Errorf("%s: invalid security version", sessionRepoName)
	}
	return int64(version), nil
}

func normalizeRevokeReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 256 || containsUnsafeDisplayCharacter(reason) {
		return "", fmt.Errorf("%s: invalid revoke reason", sessionRepoName)
	}
	return reason, nil
}

func validateSessionForCreate(session *entity.Session) error {
	if session.ID == uuid.Nil || session.UserID == uuid.Nil || session.FamilyID == uuid.Nil ||
		session.UserSecurityVersion == 0 || session.LastSeenAt.IsZero() ||
		session.LastReauthenticatedAt.IsZero() || session.IdleExpiresAt.IsZero() ||
		session.AbsoluteExpiresAt.IsZero() || correlation.Validate(session.CreatedCID) != nil ||
		(len(session.IPHash) != 0 && len(session.IPHash) != sessionIPHashBytes) ||
		(session.UserAgent != nil && (len(*session.UserAgent) > 512 || containsUnsafeDisplayCharacter(*session.UserAgent))) ||
		session.IdleExpiresAt.After(session.AbsoluteExpiresAt) {
		return fmt.Errorf("%s: invalid create input", sessionRepoName)
	}
	status := session.Status
	if status == "" {
		status = entity.SessionStatusActive
	}
	switch status {
	case entity.SessionStatusActive, entity.SessionStatusExpired:
		if session.RevokedAt != nil || session.RevokeReason != nil {
			return fmt.Errorf("%s: inconsistent lifecycle state", sessionRepoName)
		}
	case entity.SessionStatusRevoked:
		if session.RevokedAt == nil || session.RevokeReason == nil {
			return fmt.Errorf("%s: incomplete revocation state", sessionRepoName)
		}
		if _, err := normalizeRevokeReason(*session.RevokeReason); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: invalid status", sessionRepoName)
	}
	return nil
}

func containsUnsafeDisplayCharacter(value string) bool {
	if !utf8.ValidString(value) {
		return true
	}
	return strings.IndexFunc(value, func(character rune) bool {
		return unicode.IsControl(character) || unicode.Is(unicode.Cf, character)
	}) >= 0
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

var _ ports.SessionRepository = (*SessionRepository)(nil)
var _ ports.RecoveryContactSessionRepository = (*SessionRepository)(nil)
var _ ports.RecoveryContactSessionStore = (*SessionRepository)(nil)
