package usecase

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/logger"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	defaultSessionPageLimit  = 20
	maximumSessionPageLimit  = 50
	sessionCursorBytes       = 24
	defaultSessionReceiptTTL = 24 * time.Hour
	maximumSessionReceiptTTL = 30 * 24 * time.Hour
)

var ErrInvalidSessionManagement = errors.New("identity: invalid session management")

type SessionManagementConfig struct {
	ReceiptTTL time.Duration
	Clock      func() time.Time
}

type SessionManagementService struct {
	repository  ports.SessionManagementRepository
	tx          tx.Manager
	journal     ports.SecurityMutationJournal
	invalidator ports.SessionManagementInvalidator
	config      SessionManagementConfig
}

func NewSessionManagementService(
	repository ports.SessionManagementRepository,
	txManager tx.Manager,
	journal ports.SecurityMutationJournal,
	invalidator ports.SessionManagementInvalidator,
	config SessionManagementConfig,
) (*SessionManagementService, error) {
	if config.ReceiptTTL == 0 {
		config.ReceiptTTL = defaultSessionReceiptTTL
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if nilSessionManagementDependency(repository) || nilSessionManagementDependency(txManager) ||
		nilSessionManagementDependency(journal) || nilSessionManagementDependency(invalidator) ||
		config.ReceiptTTL <= 0 || config.ReceiptTTL > maximumSessionReceiptTTL {
		return nil, ErrInvalidSessionManagement
	}
	return &SessionManagementService{
		repository: repository, tx: txManager, journal: journal,
		invalidator: invalidator, config: config,
	}, nil
}

func (service *SessionManagementService) List(
	ctx context.Context,
	userID, currentSessionID uuid.UUID,
	request *dto.SessionListRequest,
) (*dto.SessionListResponse, error) {
	if !service.validContext(ctx) || userID == uuid.Nil || currentSessionID == uuid.Nil || request == nil {
		return nil, invalidSessionManagementRequest()
	}
	limit := request.Limit
	if limit == 0 {
		limit = defaultSessionPageLimit
	}
	if limit < 1 || limit > maximumSessionPageLimit {
		return nil, invalidSessionManagementRequest()
	}
	var after *ports.SessionCursor
	if request.Cursor != "" {
		decoded, err := decodeSessionCursor(request.Cursor)
		if err != nil {
			return nil, invalidSessionManagementRequest()
		}
		after = &decoded
	}
	page, err := service.repository.List(ctx, ports.SessionListQuery{
		UserID: userID, After: after, Limit: limit,
	})
	if err != nil {
		return nil, sessionManagementInternal("list authentication sessions", err)
	}
	if len(page.Sessions) > limit {
		return nil, sessionManagementInternal("validate authentication session page", ErrInvalidSessionManagement)
	}
	if page.Next != nil {
		if len(page.Sessions) == 0 || page.Next.ID == uuid.Nil || page.Next.CreatedAt.IsZero() {
			return nil, sessionManagementInternal("validate authentication session cursor", ErrInvalidSessionManagement)
		}
		last := page.Sessions[len(page.Sessions)-1]
		if page.Next.ID != last.ID || !page.Next.CreatedAt.Equal(last.CreatedAt) {
			return nil, sessionManagementInternal("validate authentication session cursor", ErrInvalidSessionManagement)
		}
	}
	now := service.config.Clock().UTC()
	response := &dto.SessionListResponse{Items: make([]dto.SessionResponse, 0, len(page.Sessions))}
	for index := range page.Sessions {
		session := page.Sessions[index]
		if !validSessionManagementProjection(session) || session.UserID != userID {
			return nil, sessionManagementInternal("validate authentication session projection", ErrInvalidSessionManagement)
		}
		status := session.Status
		if status == entity.SessionStatusActive &&
			(!session.IdleExpiresAt.After(now) || !session.AbsoluteExpiresAt.After(now)) {
			status = entity.SessionStatusExpired
		}
		view := dto.SessionResponse{
			ID: session.ID, Current: session.ID == currentSessionID && status == entity.SessionStatusActive,
			Status: string(status), CreatedAt: session.CreatedAt.UTC(),
			LastSeenAt:            session.LastSeenAt.UTC(),
			LastReauthenticatedAt: session.LastReauthenticatedAt.UTC(),
			IdleExpiresAt:         session.IdleExpiresAt.UTC(), AbsoluteExpiresAt: session.AbsoluteExpiresAt.UTC(),
			RevokedAt: utcSessionManagementTime(session.RevokedAt),
		}
		if session.UserAgent != nil && validSessionManagementUserAgent(*session.UserAgent) {
			view.UserAgent = *session.UserAgent
		}
		response.Items = append(response.Items, view)
	}
	if page.Next != nil {
		response.NextCursor, err = encodeSessionCursor(*page.Next)
		if err != nil {
			return nil, sessionManagementInternal("encode authentication session cursor", err)
		}
	}
	logger.FromContext(ctx).Debug(
		"authentication sessions listed",
		zap.Int("count", len(response.Items)),
		zap.Bool("has_next", response.NextCursor != ""),
	)
	return response, nil
}

func validSessionManagementProjection(session entity.Session) bool {
	if session.ID == uuid.Nil || session.UserID == uuid.Nil || session.CreatedAt.IsZero() ||
		session.LastSeenAt.IsZero() || session.LastReauthenticatedAt.IsZero() ||
		session.IdleExpiresAt.IsZero() || session.AbsoluteExpiresAt.IsZero() ||
		session.IdleExpiresAt.After(session.AbsoluteExpiresAt) {
		return false
	}
	switch session.Status {
	case entity.SessionStatusActive:
		return session.RevokedAt == nil && session.RevokeReason == nil
	case entity.SessionStatusRevoked:
		return session.RevokedAt != nil && session.RevokeReason != nil
	case entity.SessionStatusExpired:
		return session.RevokedAt == nil && session.RevokeReason == nil
	default:
		return false
	}
}

func (service *SessionManagementService) Revoke(
	ctx context.Context,
	userID, actorSessionID, targetSessionID, idempotencyKey uuid.UUID,
) error {
	if !service.validContext(ctx) || userID == uuid.Nil || actorSessionID == uuid.Nil ||
		targetSessionID == uuid.Nil || idempotencyKey == uuid.Nil {
		return invalidSessionManagementRequest()
	}
	now := service.config.Clock().UTC()
	var mutation ports.SessionManagementMutation
	invalidationScheduled := false
	err := service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var mutateErr error
		mutation, mutateErr = service.repository.RevokeOwned(
			transactionContext,
			ports.SessionRevokeCommand{
				UserID: userID, ActorSessionID: actorSessionID,
				TargetSessionID: targetSessionID, IdempotencyKey: idempotencyKey,
			},
			now,
			now.Add(service.config.ReceiptTTL),
		)
		if mutateErr != nil || !mutation.Applied {
			return mutateErr
		}
		if !validSessionManagementMutation(mutation, ports.SessionCommandRevoke, actorSessionID, &targetSessionID) {
			return ErrInvalidSessionManagement
		}
		if err := service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: mutation.MutationID, Kind: entity.SecurityMutationSessionRevoked,
			UserID: userID, SessionID: uuidPointer(actorSessionID),
			TargetSessionID: uuidPointer(targetSessionID), OccurredAt: now,
		}); err != nil {
			return err
		}
		invalidationScheduled, mutateErr = registerAfterCommit(
			service.tx,
			transactionContext,
			service.sessionInvalidation(targetSessionID),
		)
		return mutateErr
	})
	if err != nil {
		return classifySessionManagementError("revoke authentication session", err)
	}
	if mutation.Applied && !invalidationScheduled {
		if err := invokeAfterCommitFallback(ctx, service.sessionInvalidation(targetSessionID)); err != nil {
			return sessionManagementInternal("invalidate authentication session", err)
		}
	}
	return nil
}

func (service *SessionManagementService) LogoutAll(
	ctx context.Context,
	userID, actorSessionID, idempotencyKey uuid.UUID,
) error {
	if !service.validContext(ctx) || userID == uuid.Nil || actorSessionID == uuid.Nil || idempotencyKey == uuid.Nil {
		return invalidSessionManagementRequest()
	}
	now := service.config.Clock().UTC()
	var mutation ports.SessionManagementMutation
	invalidationScheduled := false
	err := service.tx.DoInTx(ctx, func(transactionContext context.Context) error {
		var mutateErr error
		mutation, mutateErr = service.repository.LogoutAll(
			transactionContext,
			ports.SessionLogoutAllCommand{
				UserID: userID, ActorSessionID: actorSessionID, IdempotencyKey: idempotencyKey,
			},
			now,
			now.Add(service.config.ReceiptTTL),
		)
		if mutateErr != nil || !mutation.Applied {
			return mutateErr
		}
		if !validSessionManagementMutation(mutation, ports.SessionCommandLogoutAll, actorSessionID, nil) ||
			mutation.SecurityVersion == 0 {
			return ErrInvalidSessionManagement
		}
		if err := service.journal.Append(transactionContext, entity.SecurityMutation{
			ID: mutation.MutationID, Kind: entity.SecurityMutationSessionsRevokedAll,
			UserID: userID, SessionID: uuidPointer(actorSessionID),
			SecurityVersion: mutation.SecurityVersion, OccurredAt: now,
		}); err != nil {
			return err
		}
		invalidationScheduled, mutateErr = registerAfterCommit(
			service.tx,
			transactionContext,
			service.userInvalidation(userID),
		)
		return mutateErr
	})
	if err != nil {
		return classifySessionManagementError("revoke all authentication sessions", err)
	}
	if mutation.Applied && !invalidationScheduled {
		if err := invokeAfterCommitFallback(ctx, service.userInvalidation(userID)); err != nil {
			return sessionManagementInternal("invalidate user authentication sessions", err)
		}
	}
	return nil
}

func (service *SessionManagementService) sessionInvalidation(sessionID uuid.UUID) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := service.invalidator.InvalidateSession(ctx, sessionID); err != nil {
			return err
		}
		logger.FromContext(ctx).Info(
			"authentication session revoked by owner",
		)
		return nil
	}
}

func (service *SessionManagementService) userInvalidation(userID uuid.UUID) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := service.invalidator.InvalidateUser(ctx, userID); err != nil {
			return err
		}
		logger.FromContext(ctx).Info(
			"all authentication sessions revoked by owner",
		)
		return nil
	}
}

func (service *SessionManagementService) validContext(ctx context.Context) bool {
	return service != nil && !nilSessionManagementDependency(service.repository) &&
		!nilSessionManagementDependency(service.tx) && !nilSessionManagementDependency(service.journal) &&
		!nilSessionManagementDependency(service.invalidator) && ctx != nil && ctx.Err() == nil
}

func encodeSessionCursor(cursor ports.SessionCursor) (string, error) {
	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() || cursor.CreatedAt.UnixNano() <= 0 {
		return "", ErrInvalidSessionManagement
	}
	bytes := make([]byte, sessionCursorBytes)
	binary.BigEndian.PutUint64(bytes[:8], uint64(cursor.CreatedAt.UTC().UnixNano()))
	copy(bytes[8:], cursor.ID[:])
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func decodeSessionCursor(raw string) (ports.SessionCursor, error) {
	if raw == "" || len(raw) > base64.RawURLEncoding.EncodedLen(sessionCursorBytes) {
		return ports.SessionCursor{}, ErrInvalidSessionManagement
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(decoded) != sessionCursorBytes {
		return ports.SessionCursor{}, ErrInvalidSessionManagement
	}
	nanos := binary.BigEndian.Uint64(decoded[:8])
	if nanos == 0 || nanos > math.MaxInt64 {
		return ports.SessionCursor{}, ErrInvalidSessionManagement
	}
	id, err := uuid.FromBytes(decoded[8:])
	if err != nil || id == uuid.Nil {
		return ports.SessionCursor{}, ErrInvalidSessionManagement
	}
	return ports.SessionCursor{CreatedAt: time.Unix(0, int64(nanos)).UTC(), ID: id}, nil
}

func validSessionManagementMutation(
	mutation ports.SessionManagementMutation,
	kind ports.SessionCommandKind,
	actorSessionID uuid.UUID,
	targetSessionID *uuid.UUID,
) bool {
	if mutation.MutationID == uuid.Nil || mutation.Kind != kind ||
		mutation.ActorSessionID != actorSessionID ||
		(mutation.TargetSessionID == nil) != (targetSessionID == nil) {
		return false
	}
	return targetSessionID == nil || *mutation.TargetSessionID == *targetSessionID
}

func classifySessionManagementError(operation string, err error) error {
	switch {
	case errors.Is(err, ports.ErrOwnedSessionNotFound):
		return apperr.New(apperr.CodeNotFound, "session not found", err)
	case errors.Is(err, ports.ErrSessionCommandIdempotencyConflict):
		return apperr.New(apperr.CodeConflict, "idempotency key conflict", err)
	case errors.Is(err, ports.ErrSessionCommandAuthorityLost):
		return apperr.New(apperr.CodeUnauthorized, "authentication session is no longer active", err)
	case transactionWasCommitted(err):
		return err
	default:
		return sessionManagementInternal(operation, err)
	}
}

func invalidSessionManagementRequest() error {
	return apperr.New(apperr.CodeBadRequest, "invalid session request", ErrInvalidSessionManagement)
}

func sessionManagementInternal(operation string, err error) error {
	return apperr.New(
		apperr.CodeInternalServer,
		"session management unavailable",
		fmt.Errorf("%s: %w", operation, err),
	)
}

func uuidPointer(value uuid.UUID) *uuid.UUID {
	copy := value
	return &copy
}

func utcSessionManagementTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func validSessionManagementUserAgent(value string) bool {
	if value == "" || len(value) > 512 || !utf8.ValidString(value) {
		return false
	}
	return strings.IndexFunc(value, func(character rune) bool {
		return unicode.IsControl(character) || unicode.Is(unicode.Cf, character)
	}) < 0
}

func nilSessionManagementDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

var _ ports.SessionManagementLifecycle = (*SessionManagementService)(nil)
