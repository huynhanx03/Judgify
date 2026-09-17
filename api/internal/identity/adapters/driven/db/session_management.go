package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsessioncommandreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/predicate"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const sessionManagementRepoName = "session management"
const maximumSessionPageLimit = 50

// SessionManagementRepository is the transaction-bound adapter for the
// account-security UI. It intentionally does not alter SessionRepository's
// authentication contract. All state, including durable command receipts, is
// written through generated Ent queries in the caller's transaction.
type SessionManagementRepository struct {
	client *dbent.EntClient
}

func NewSessionManagementRepository(client *dbent.EntClient) ports.SessionManagementRepository {
	return &SessionManagementRepository{client: client}
}

func (repository *SessionManagementRepository) List(
	ctx context.Context,
	query ports.SessionListQuery,
) (ports.SessionListResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || query.UserID == uuid.Nil ||
		query.Limit < 1 || query.Limit > maximumSessionPageLimit {
		return ports.SessionListResult{}, ports.ErrInvalidSessionManagementPersistence
	}
	requestLimit := query.Limit + 1
	if requestLimit <= query.Limit { // defensive integer-overflow guard
		return ports.SessionListResult{}, ports.ErrInvalidSessionManagementPersistence
	}
	where := []predicate.AuthSession{
		authsession.UserID(query.UserID),
	}
	if query.After != nil {
		if query.After.ID == uuid.Nil || query.After.CreatedAt.IsZero() {
			return ports.SessionListResult{}, ports.ErrInvalidSessionManagementPersistence
		}
		where = append(where, authsession.Or(
			authsession.CreatedAtLT(query.After.CreatedAt.UTC()),
			authsession.And(
				authsession.CreatedAtEQ(query.After.CreatedAt.UTC()),
				authsession.IDLT(query.After.ID),
			),
		))
	}
	queryBuilder := repository.client.DB(ctx).AuthSession.Query().Where(where...).
		Order(generate.Desc(authsession.FieldCreatedAt), generate.Desc(authsession.FieldID)).
		Limit(requestLimit).
		Select(
			authsession.FieldID,
			authsession.FieldUserID,
			authsession.FieldStatus,
			authsession.FieldCreatedAt,
			authsession.FieldLastSeenAt,
			authsession.FieldLastReauthenticatedAt,
			authsession.FieldIdleExpiresAt,
			authsession.FieldAbsoluteExpiresAt,
			authsession.FieldRevokedAt,
			authsession.FieldRevokeReason,
			authsession.FieldUserAgent,
		)
	records, err := queryBuilder.All(ctx)
	if err != nil {
		return ports.SessionListResult{}, commonent.MapEntError(err, sessionManagementRepoName)
	}
	result := ports.SessionListResult{Sessions: make([]entity.Session, 0, min(len(records), query.Limit))}
	for index, record := range records {
		if index == query.Limit {
			break
		}
		if record == nil || record.ID == uuid.Nil || record.UserID != query.UserID {
			return ports.SessionListResult{}, ports.ErrInvalidSessionManagementPersistence
		}
		result.Sessions = append(result.Sessions, managedSessionEntity(record))
	}
	if len(records) > query.Limit {
		last := result.Sessions[len(result.Sessions)-1]
		result.Next = &ports.SessionCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return result, nil
}

func (repository *SessionManagementRepository) RevokeOwned(
	ctx context.Context,
	command ports.SessionRevokeCommand,
	now, receiptExpiresAt time.Time,
) (ports.SessionManagementMutation, error) {
	if err := repository.validateMutationContext(ctx); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	if !validSessionRevokeCommand(command, now, receiptExpiresAt) {
		return ports.SessionManagementMutation{}, ports.ErrInvalidSessionManagementPersistence
	}
	client := repository.client.DB(ctx)
	owner, err := lockSessionCommandOwner(ctx, client, command.UserID)
	if err != nil {
		return ports.SessionManagementMutation{}, err
	}
	if receipt, found, err := repository.findReceipt(ctx, command.UserID, command.IdempotencyKey, now); err != nil {
		return ports.SessionManagementMutation{}, err
	} else if found {
		return validateRevokeReceipt(receipt, command)
	}
	if err := validateLiveCommandActor(ctx, client, owner, command.ActorSessionID, now); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	target, err := client.AuthSession.Query().
		Where(authsession.ID(command.TargetSessionID), authsession.UserID(command.UserID)).
		Select(authsession.FieldID, authsession.FieldStatus).
		ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.SessionManagementMutation{}, ports.ErrOwnedSessionNotFound
		}
		return ports.SessionManagementMutation{}, commonent.MapEntError(err, sessionManagementRepoName)
	}
	targetID := command.TargetSessionID
	mutation := ports.SessionManagementMutation{
		Kind:           ports.SessionCommandRevoke,
		ActorSessionID: command.ActorSessionID, TargetSessionID: &targetID,
	}
	switch target.Status {
	case authsession.StatusRevoked, authsession.StatusExpired:
		// A new idempotency key cannot create a new side effect for an already
		// terminal session, so no durable receipt is needed. Exact retries of a
		// previously applied command are still recovered above from its receipt.
		return mutation, nil
	case authsession.StatusActive:
	default:
		return ports.SessionManagementMutation{}, ports.ErrInvalidSessionManagementPersistence
	}
	mutationID, err := uuid.NewV7()
	if err != nil {
		return ports.SessionManagementMutation{}, fmt.Errorf("%s: generate mutation id: %w", sessionManagementRepoName, err)
	}
	mutation.MutationID = mutationID
	if err := repository.revokeSessionState(ctx, client, command.TargetSessionID, now); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	mutation.Applied = true
	if err := repository.insertReceipt(ctx, mutation, command.UserID, command.IdempotencyKey, now, receiptExpiresAt); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	return mutation, nil
}

func (repository *SessionManagementRepository) LogoutAll(
	ctx context.Context,
	command ports.SessionLogoutAllCommand,
	now, receiptExpiresAt time.Time,
) (ports.SessionManagementMutation, error) {
	if err := repository.validateMutationContext(ctx); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	if !validSessionLogoutAllCommand(command, now, receiptExpiresAt) {
		return ports.SessionManagementMutation{}, ports.ErrInvalidSessionManagementPersistence
	}
	client := repository.client.DB(ctx)
	owner, err := lockSessionCommandOwner(ctx, client, command.UserID)
	if err != nil {
		return ports.SessionManagementMutation{}, err
	}
	if receipt, found, err := repository.findReceipt(ctx, command.UserID, command.IdempotencyKey, now); err != nil {
		return ports.SessionManagementMutation{}, err
	} else if found {
		return validateLogoutAllReceipt(receipt, command)
	}
	if err := validateLiveCommandActor(ctx, client, owner, command.ActorSessionID, now); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	if owner.SecurityVersion == math.MaxInt64 {
		return ports.SessionManagementMutation{}, fmt.Errorf("%s: security version exhausted", sessionManagementRepoName)
	}
	newVersion := owner.SecurityVersion + 1
	updatedUser, err := client.User.UpdateOneID(command.UserID).
		Where(user.SecurityVersionEQ(owner.SecurityVersion)).
		SetSecurityVersion(newVersion).
		Save(ctx)
	if err != nil {
		return ports.SessionManagementMutation{}, commonent.MapEntError(err, sessionManagementRepoName)
	}
	if updatedUser == nil || updatedUser.ID != command.UserID {
		return ports.SessionManagementMutation{}, ports.ErrSessionCommandAuthorityLost
	}
	updatedSessions, err := client.AuthSession.Update().
		Where(authsession.UserID(command.UserID), authsession.StatusEQ(authsession.StatusActive)).
		SetStatus(authsession.StatusRevoked).
		SetRevokedAt(now.UTC()).
		SetRevokeReason("user_logout_all").
		Save(ctx)
	if err != nil {
		return ports.SessionManagementMutation{}, commonent.MapEntError(err, sessionManagementRepoName)
	}
	if updatedSessions < 1 {
		return ports.SessionManagementMutation{}, ports.ErrSessionCommandAuthorityLost
	}
	if _, err := client.AuthRefreshToken.Update().
		Where(authrefreshtoken.StatusEQ(authrefreshtoken.StatusIssued), authrefreshtoken.HasSessionWith(authsession.UserID(command.UserID))).
		SetStatus(authrefreshtoken.StatusRevoked).
		SetRevokedAt(now.UTC()).
		Save(ctx); err != nil {
		return ports.SessionManagementMutation{}, commonent.MapEntError(err, sessionManagementRepoName)
	}
	mutationID, err := uuid.NewV7()
	if err != nil {
		return ports.SessionManagementMutation{}, fmt.Errorf("%s: generate mutation id: %w", sessionManagementRepoName, err)
	}
	mutation := ports.SessionManagementMutation{
		MutationID: mutationID, Kind: ports.SessionCommandLogoutAll,
		ActorSessionID: command.ActorSessionID, SecurityVersion: uint64(newVersion), Applied: true,
	}
	if err := repository.insertReceipt(ctx, mutation, command.UserID, command.IdempotencyKey, now, receiptExpiresAt); err != nil {
		return ports.SessionManagementMutation{}, err
	}
	return mutation, nil
}

type sessionCommandOwner struct {
	ID              uuid.UUID
	Status          user.Status
	SecurityVersion int64
	DeletedAt       *time.Time
}

func lockSessionCommandOwner(ctx context.Context, client *generate.Client, userID uuid.UUID) (*sessionCommandOwner, error) {
	record, err := client.User.Query().Where(user.ID(userID)).
		Select(user.FieldID, user.FieldStatus, user.FieldSecurityVersion, user.FieldDeletedAt).
		ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return nil, ports.ErrSessionCommandAuthorityLost
		}
		return nil, commonent.MapEntError(err, sessionManagementRepoName)
	}
	return &sessionCommandOwner{
		ID: record.ID, Status: record.Status, SecurityVersion: record.SecurityVersion, DeletedAt: record.DeletedAt,
	}, nil
}

func validateLiveCommandActor(
	ctx context.Context,
	client *generate.Client,
	owner *sessionCommandOwner,
	actorSessionID uuid.UUID,
	now time.Time,
) error {
	if owner == nil || owner.ID == uuid.Nil || owner.Status != user.StatusActive || owner.DeletedAt != nil || owner.SecurityVersion < 1 {
		return ports.ErrSessionCommandAuthorityLost
	}
	actor, err := client.AuthSession.Query().Where(
		authsession.ID(actorSessionID), authsession.UserID(owner.ID),
		authsession.StatusEQ(authsession.StatusActive),
		authsession.UserSecurityVersionEQ(owner.SecurityVersion),
		authsession.IdleExpiresAtGT(now.UTC()), authsession.AbsoluteExpiresAtGT(now.UTC()),
	).Select(authsession.FieldID).ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return ports.ErrSessionCommandAuthorityLost
		}
		return commonent.MapEntError(err, sessionManagementRepoName)
	}
	if actor == nil || actor.ID != actorSessionID {
		return ports.ErrSessionCommandAuthorityLost
	}
	return nil
}

func (repository *SessionManagementRepository) revokeSessionState(
	ctx context.Context,
	client *generate.Client,
	sessionID uuid.UUID,
	now time.Time,
) error {
	if _, err := client.AuthRefreshToken.Update().
		Where(authrefreshtoken.SessionID(sessionID), authrefreshtoken.StatusEQ(authrefreshtoken.StatusIssued)).
		SetStatus(authrefreshtoken.StatusRevoked).SetRevokedAt(now.UTC()).Save(ctx); err != nil {
		return commonent.MapEntError(err, sessionManagementRepoName)
	}
	updated, err := client.AuthSession.Update().
		Where(authsession.ID(sessionID), authsession.StatusEQ(authsession.StatusActive)).
		SetStatus(authsession.StatusRevoked).SetRevokedAt(now.UTC()).SetRevokeReason("user_revoked").Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, sessionManagementRepoName)
	}
	if updated != 1 {
		return ports.ErrOwnedSessionNotFound
	}
	return nil
}

type persistedSessionCommandReceipt struct {
	MutationID      uuid.UUID
	UserID          uuid.UUID
	ActorSessionID  uuid.UUID
	TargetSessionID *uuid.UUID
	IdempotencyKey  uuid.UUID
	Kind            ports.SessionCommandKind
	SecurityVersion uint64
	ExpiresAt       time.Time
}

func (repository *SessionManagementRepository) findReceipt(
	ctx context.Context,
	userID, idempotencyKey uuid.UUID,
	now time.Time,
) (persistedSessionCommandReceipt, bool, error) {
	record, err := repository.client.DB(ctx).AuthSessionCommandReceipt.Query().
		Where(authsessioncommandreceipt.UserID(userID), authsessioncommandreceipt.IdempotencyKey(idempotencyKey)).
		ForUpdate().Only(ctx)
	if err != nil {
		if generate.IsNotFound(err) {
			return persistedSessionCommandReceipt{}, false, nil
		}
		return persistedSessionCommandReceipt{}, false, commonent.MapEntError(err, sessionManagementRepoName)
	}
	if record == nil || record.ID == uuid.Nil || record.UserID != userID || record.IdempotencyKey != idempotencyKey ||
		record.ResultSecurityVersion < 0 || record.ExpiresAt.IsZero() {
		return persistedSessionCommandReceipt{}, false, ports.ErrInvalidSessionManagementPersistence
	}
	if !record.ExpiresAt.After(now.UTC()) {
		if err := repository.client.DB(ctx).AuthSessionCommandReceipt.DeleteOneID(record.ID).Exec(ctx); err != nil {
			return persistedSessionCommandReceipt{}, false, commonent.MapEntError(err, sessionManagementRepoName)
		}
		return persistedSessionCommandReceipt{}, false, nil
	}
	return persistedSessionCommandReceipt{
		MutationID: record.ID, UserID: record.UserID, ActorSessionID: record.ActorSessionID,
		TargetSessionID: copyUUIDPointer(record.TargetSessionID), IdempotencyKey: record.IdempotencyKey,
		Kind: ports.SessionCommandKind(record.Kind), SecurityVersion: uint64(record.ResultSecurityVersion),
		ExpiresAt: record.ExpiresAt.UTC(),
	}, true, nil
}

func (repository *SessionManagementRepository) insertReceipt(
	ctx context.Context,
	mutation ports.SessionManagementMutation,
	userID, idempotencyKey uuid.UUID,
	createdAt,
	expiresAt time.Time,
) error {
	if mutation.MutationID == uuid.Nil || userID == uuid.Nil || idempotencyKey == uuid.Nil || createdAt.IsZero() || expiresAt.IsZero() || !expiresAt.After(createdAt) ||
		(mutation.Kind == ports.SessionCommandRevoke && (mutation.TargetSessionID == nil || mutation.SecurityVersion != 0)) ||
		(mutation.Kind == ports.SessionCommandLogoutAll && (mutation.TargetSessionID != nil || mutation.SecurityVersion == 0)) {
		return ports.ErrInvalidSessionManagementPersistence
	}
	_, err := repository.client.DB(ctx).AuthSessionCommandReceipt.Create().
		SetID(mutation.MutationID).
		SetUserID(userID).
		SetActorSessionID(mutation.ActorSessionID).
		SetNillableTargetSessionID(mutation.TargetSessionID).
		SetIdempotencyKey(idempotencyKey).
		SetKind(authsessioncommandreceipt.Kind(mutation.Kind)).
		SetResultSecurityVersion(int64(mutation.SecurityVersion)).
		SetExpiresAt(expiresAt.UTC()).
		SetCreatedAt(createdAt.UTC()).
		SetUpdatedAt(createdAt.UTC()).
		Save(ctx)
	if err != nil {
		if sqlgraph.IsUniqueConstraintError(err) {
			return ports.ErrSessionCommandIdempotencyConflict
		}
		return fmt.Errorf("%s: persist command receipt: %w", sessionManagementRepoName, err)
	}
	return nil
}

func validateRevokeReceipt(
	receipt persistedSessionCommandReceipt,
	command ports.SessionRevokeCommand,
) (ports.SessionManagementMutation, error) {
	if receipt.Kind != ports.SessionCommandRevoke || receipt.UserID != command.UserID ||
		receipt.ActorSessionID != command.ActorSessionID || receipt.IdempotencyKey != command.IdempotencyKey ||
		receipt.TargetSessionID == nil || *receipt.TargetSessionID != command.TargetSessionID || receipt.SecurityVersion != 0 {
		return ports.SessionManagementMutation{}, ports.ErrSessionCommandIdempotencyConflict
	}
	return ports.SessionManagementMutation{
		MutationID: receipt.MutationID, Kind: receipt.Kind, ActorSessionID: receipt.ActorSessionID,
		TargetSessionID: uuidPointer(command.TargetSessionID), SecurityVersion: 0, Applied: false,
	}, nil
}

func validateLogoutAllReceipt(
	receipt persistedSessionCommandReceipt,
	command ports.SessionLogoutAllCommand,
) (ports.SessionManagementMutation, error) {
	if receipt.Kind != ports.SessionCommandLogoutAll || receipt.UserID != command.UserID ||
		receipt.ActorSessionID != command.ActorSessionID || receipt.IdempotencyKey != command.IdempotencyKey ||
		receipt.TargetSessionID != nil || receipt.SecurityVersion == 0 {
		return ports.SessionManagementMutation{}, ports.ErrSessionCommandIdempotencyConflict
	}
	return ports.SessionManagementMutation{
		MutationID: receipt.MutationID, Kind: receipt.Kind, ActorSessionID: receipt.ActorSessionID,
		SecurityVersion: receipt.SecurityVersion, Applied: false,
	}, nil
}

func (repository *SessionManagementRepository) validateMutationContext(ctx context.Context) error {
	if repository == nil || repository.client == nil || ctx == nil || ctx.Err() != nil {
		return ports.ErrInvalidSessionManagementPersistence
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return errors.Join(ports.ErrInvalidSessionManagementPersistence, platformtx.ErrTransactionRequired)
	}
	return nil
}

func validSessionRevokeCommand(command ports.SessionRevokeCommand, now, expiresAt time.Time) bool {
	return command.UserID != uuid.Nil && command.ActorSessionID != uuid.Nil && command.TargetSessionID != uuid.Nil &&
		command.IdempotencyKey != uuid.Nil && !now.IsZero() && !expiresAt.IsZero() && expiresAt.After(now)
}

func validSessionLogoutAllCommand(command ports.SessionLogoutAllCommand, now, expiresAt time.Time) bool {
	return command.UserID != uuid.Nil && command.ActorSessionID != uuid.Nil && command.IdempotencyKey != uuid.Nil &&
		!now.IsZero() && !expiresAt.IsZero() && expiresAt.After(now)
}

func managedSessionEntity(record *generate.AuthSession) entity.Session {
	result := entity.Session{
		ID: record.ID, UserID: record.UserID, Status: entity.SessionStatus(record.Status),
		CreatedAt: record.CreatedAt.UTC(), LastSeenAt: record.LastSeenAt.UTC(),
		LastReauthenticatedAt: record.LastReauthenticatedAt.UTC(), IdleExpiresAt: record.IdleExpiresAt.UTC(),
		AbsoluteExpiresAt: record.AbsoluteExpiresAt.UTC(), RevokedAt: utcTimePointer(record.RevokedAt),
	}
	result.RevokeReason = copyStringPointer(record.RevokeReason)
	result.UserAgent = copyStringPointer(record.UserAgent)
	return result
}

func uuidPointer(value uuid.UUID) *uuid.UUID {
	copy := value
	return &copy
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

var _ ports.SessionManagementRepository = (*SessionManagementRepository)(nil)
