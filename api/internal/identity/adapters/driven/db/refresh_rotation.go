package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshreceipt"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	"github.com/huynhanx03/judgify/internal/ent/generate/user"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const refreshRotationRepoName = "refresh rotation"

type RefreshRotationRepository struct {
	client *dbent.EntClient
}

func NewRefreshRotationRepository(client *dbent.EntClient) ports.RefreshRotationRepository {
	return &RefreshRotationRepository{client: client}
}

func (repository *RefreshRotationRepository) ContextByHash(
	ctx context.Context,
	tokenHash []byte,
	now time.Time,
) (ports.RefreshContext, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		len(tokenHash) != refreshTokenHashBytes || now.IsZero() {
		return ports.RefreshContext{}, fmt.Errorf("%s: invalid context lookup", refreshRotationRepoName)
	}
	record, err := repository.client.DB(ctx).AuthRefreshToken.Query().
		Where(authrefreshtoken.TokenHashEQ(append([]byte(nil), tokenHash...))).
		WithSession(func(sessionQuery *generate.AuthSessionQuery) {
			sessionQuery.WithUser()
		}).
		Only(ctx)
	if err != nil {
		return ports.RefreshContext{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	session, err := record.Edges.SessionOrErr()
	if err != nil {
		return ports.RefreshContext{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	owner, err := session.Edges.UserOrErr()
	if err != nil {
		return ports.RefreshContext{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	if record.Status == authrefreshtoken.StatusRevoked || !record.ExpiresAt.After(now.UTC()) ||
		session.Status != authsession.StatusActive || !session.IdleExpiresAt.After(now.UTC()) ||
		!session.AbsoluteExpiresAt.After(now.UTC()) || owner.Status != user.StatusActive ||
		owner.DeletedAt != nil || owner.SecurityVersion < 1 ||
		session.UserSecurityVersion != owner.SecurityVersion {
		return ports.RefreshContext{}, fmt.Errorf("%s: credential is not active", refreshRotationRepoName)
	}
	return ports.RefreshContext{
		TokenID:           record.ID,
		SessionID:         session.ID,
		UserID:            owner.ID,
		Username:          owner.Username,
		SecurityVersion:   uint64(owner.SecurityVersion),
		AuthenticatedAt:   session.LastReauthenticatedAt.UTC(),
		AbsoluteExpiresAt: session.AbsoluteExpiresAt.UTC(),
		IdleExpiresAt:     session.IdleExpiresAt.UTC(),
		RefreshExpiresAt:  record.ExpiresAt.UTC(),
	}, nil
}

func (repository *RefreshRotationRepository) Rotate(
	ctx context.Context,
	mutation ports.RefreshRotationMutation,
	now time.Time,
) (entity.RefreshRotationResult, error) {
	if repository == nil || repository.client == nil || ctx == nil || now.IsZero() ||
		len(mutation.PresentedHash) != refreshTokenHashBytes ||
		mutation.Replacement == nil || mutation.Receipt == nil {
		return entity.RefreshRotationResult{}, fmt.Errorf("%s: invalid mutation", refreshRotationRepoName)
	}
	now = now.UTC()
	client := repository.client.DB(ctx)
	presented, session, owner, err := lockRefreshRotationState(
		ctx,
		client,
		mutation.Expected,
		mutation.PresentedHash,
	)
	if err != nil {
		return entity.RefreshRotationResult{}, err
	}
	if presented.Status == authrefreshtoken.StatusConsumed {
		return repository.recoverOrContainReplay(
			ctx,
			client,
			presented,
			session,
			owner,
			mutation.Expected,
			mutation.Receipt.IdempotencyKey,
			now,
		)
	}
	if err := validateLockedRefreshContext(presented, session, owner, mutation.Expected, now); err != nil {
		return entity.RefreshRotationResult{}, err
	}
	if presented.Status != authrefreshtoken.StatusIssued || !presented.ExpiresAt.After(now) {
		return entity.RefreshRotationResult{}, fmt.Errorf("%s: credential is not active", refreshRotationRepoName)
	}

	replacement := mutation.Replacement
	if replacement.ID == uuid.Nil || replacement.SessionID != session.ID ||
		replacement.ParentID == nil || *replacement.ParentID != presented.ID ||
		len(replacement.TokenHash) != refreshTokenHashBytes ||
		replacement.Status != entity.RefreshTokenStatusIssued ||
		!replacement.IssuedAt.Equal(now) || !replacement.ExpiresAt.After(now) {
		return entity.RefreshRotationResult{}, fmt.Errorf("%s: invalid replacement", refreshRotationRepoName)
	}
	if _, err := client.AuthRefreshToken.Create().
		SetID(replacement.ID).
		SetSessionID(replacement.SessionID).
		SetTokenHash(append([]byte(nil), replacement.TokenHash...)).
		SetParentID(presented.ID).
		SetStatus(authrefreshtoken.StatusIssued).
		SetIssuedAt(now).
		SetExpiresAt(replacement.ExpiresAt.UTC()).
		Save(ctx); err != nil {
		return entity.RefreshRotationResult{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	updated, err := client.AuthRefreshToken.Update().
		Where(
			authrefreshtoken.ID(presented.ID),
			authrefreshtoken.StatusEQ(authrefreshtoken.StatusIssued),
		).
		SetStatus(authrefreshtoken.StatusConsumed).
		SetConsumedAt(now).
		SetReplacedByID(replacement.ID).
		Save(ctx)
	if err != nil {
		return entity.RefreshRotationResult{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	if updated != 1 {
		return entity.RefreshRotationResult{}, fmt.Errorf("%s: lost single-use transition", refreshRotationRepoName)
	}

	receipt := mutation.Receipt
	if receipt.ID == uuid.Nil || receipt.SessionID != session.ID ||
		receipt.PresentedTokenID != presented.ID || receipt.IdempotencyKey == uuid.Nil ||
		receipt.ReplacementTokenID != replacement.ID || len(receipt.RecoveryCiphertext) == 0 ||
		receipt.KeyID == "" || !receipt.ExpiresAt.After(now) {
		return entity.RefreshRotationResult{}, fmt.Errorf("%s: invalid receipt", refreshRotationRepoName)
	}
	if _, err := client.AuthRefreshReceipt.Create().
		SetID(receipt.ID).
		SetSessionID(receipt.SessionID).
		SetPresentedTokenID(receipt.PresentedTokenID).
		SetIdempotencyKey(receipt.IdempotencyKey).
		SetReplacementTokenID(receipt.ReplacementTokenID).
		SetRecoveryCiphertext(append([]byte(nil), receipt.RecoveryCiphertext...)).
		SetKeyID(receipt.KeyID).
		SetExpiresAt(receipt.ExpiresAt.UTC()).
		Save(ctx); err != nil {
		return entity.RefreshRotationResult{}, commonent.MapEntError(err, refreshRotationRepoName)
	}

	return refreshRotationResult(
		entity.RefreshRotationIssued,
		receipt.ID,
		replacement.ID,
		session,
		owner,
		replacement.ExpiresAt,
		receipt.RecoveryCiphertext,
	), nil
}

func (repository *RefreshRotationRepository) recoverOrContainReplay(
	ctx context.Context,
	client *generate.Client,
	presented *generate.AuthRefreshToken,
	session *generate.AuthSession,
	owner *generate.User,
	expected ports.RefreshContext,
	idempotencyKey uuid.UUID,
	now time.Time,
) (entity.RefreshRotationResult, error) {
	receipt, err := client.AuthRefreshReceipt.Query().Where(
		authrefreshreceipt.SessionID(presented.SessionID),
		authrefreshreceipt.PresentedTokenID(presented.ID),
		authrefreshreceipt.IdempotencyKey(idempotencyKey),
		authrefreshreceipt.ExpiresAtGT(now),
		authrefreshreceipt.ErasedAtIsNil(),
	).Only(ctx)
	if err == nil {
		// Exact receipt recovery releases credentials, so it must satisfy the same
		// locked canonical fence as a new issuance. A missing exact receipt is a
		// replay signal and is contained even when optimistic snapshot fields drift.
		if validateErr := validateLockedRefreshContext(presented, session, owner, expected, now); validateErr != nil {
			return entity.RefreshRotationResult{}, validateErr
		}
		replacement, loadErr := client.AuthRefreshToken.Get(ctx, receipt.ReplacementTokenID)
		if loadErr != nil {
			return entity.RefreshRotationResult{}, commonent.MapEntError(loadErr, refreshRotationRepoName)
		}
		return refreshRotationResult(
			entity.RefreshRotationRecovered,
			receipt.ID,
			replacement.ID,
			session,
			owner,
			replacement.ExpiresAt,
			receipt.RecoveryCiphertext,
		), nil
	}
	if !generate.IsNotFound(err) {
		return entity.RefreshRotationResult{}, commonent.MapEntError(err, refreshRotationRepoName)
	}

	if _, err := client.AuthRefreshToken.Update().
		Where(
			authrefreshtoken.SessionID(presented.SessionID),
			authrefreshtoken.StatusEQ(authrefreshtoken.StatusIssued),
		).
		SetStatus(authrefreshtoken.StatusRevoked).
		SetRevokedAt(now).
		Save(ctx); err != nil {
		return entity.RefreshRotationResult{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	if _, err := client.AuthSession.Update().
		Where(
			authsession.ID(presented.SessionID),
			authsession.StatusEQ(authsession.StatusActive),
		).
		SetStatus(authsession.StatusRevoked).
		SetRevokedAt(now).
		SetRevokeReason("refresh_replay").
		Save(ctx); err != nil {
		return entity.RefreshRotationResult{}, commonent.MapEntError(err, refreshRotationRepoName)
	}
	return entity.RefreshRotationResult{
		Outcome:   entity.RefreshRotationReplayContained,
		SessionID: presented.SessionID,
	}, nil
}

func lockRefreshRotationState(
	ctx context.Context,
	client *generate.Client,
	expected ports.RefreshContext,
	tokenHash []byte,
) (*generate.AuthRefreshToken, *generate.AuthSession, *generate.User, error) {
	if ctx == nil || client == nil || expected.UserID == uuid.Nil || expected.SessionID == uuid.Nil ||
		expected.TokenID == uuid.Nil || len(tokenHash) != refreshTokenHashBytes {
		return nil, nil, nil, fmt.Errorf("%s: invalid lock identity", refreshRotationRepoName)
	}
	// Identity commands share one global lock order. Session revocation already
	// locks user -> session; refresh must do the same before locking its token or
	// concurrent refresh/logout requests can form a PostgreSQL deadlock cycle.
	owner, err := client.User.Query().
		Where(user.ID(expected.UserID)).
		Select(
			user.FieldID,
			user.FieldUsername,
			user.FieldStatus,
			user.FieldSecurityVersion,
			user.FieldDeletedAt,
		).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, nil, nil, commonent.MapEntError(err, refreshRotationRepoName)
	}
	session, err := client.AuthSession.Query().
		Where(authsession.ID(expected.SessionID), authsession.UserID(expected.UserID)).
		Select(
			authsession.FieldID,
			authsession.FieldUserID,
			authsession.FieldUserSecurityVersion,
			authsession.FieldStatus,
			authsession.FieldLastReauthenticatedAt,
			authsession.FieldIdleExpiresAt,
			authsession.FieldAbsoluteExpiresAt,
		).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, nil, nil, commonent.MapEntError(err, refreshRotationRepoName)
	}
	presented, err := client.AuthRefreshToken.Query().
		Where(
			authrefreshtoken.ID(expected.TokenID),
			authrefreshtoken.SessionID(expected.SessionID),
			authrefreshtoken.TokenHashEQ(append([]byte(nil), tokenHash...)),
		).
		Select(
			authrefreshtoken.FieldID,
			authrefreshtoken.FieldSessionID,
			authrefreshtoken.FieldStatus,
			authrefreshtoken.FieldExpiresAt,
		).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, nil, nil, commonent.MapEntError(err, refreshRotationRepoName)
	}
	return presented, session, owner, nil
}

func validateLockedRefreshContext(
	presented *generate.AuthRefreshToken,
	session *generate.AuthSession,
	owner *generate.User,
	expected ports.RefreshContext,
	now time.Time,
) error {
	if presented.ID != expected.TokenID || presented.SessionID != expected.SessionID ||
		session.ID != expected.SessionID || session.UserID != expected.UserID ||
		owner.ID != expected.UserID || owner.Username != expected.Username ||
		owner.SecurityVersion < 1 || uint64(owner.SecurityVersion) != expected.SecurityVersion ||
		session.UserSecurityVersion != owner.SecurityVersion ||
		!session.LastReauthenticatedAt.Equal(expected.AuthenticatedAt) ||
		!session.AbsoluteExpiresAt.Equal(expected.AbsoluteExpiresAt) ||
		session.IdleExpiresAt.Before(expected.IdleExpiresAt) ||
		!presented.ExpiresAt.Equal(expected.RefreshExpiresAt) ||
		session.Status != authsession.StatusActive || owner.Status != user.StatusActive ||
		owner.DeletedAt != nil || !session.IdleExpiresAt.After(now) ||
		!session.AbsoluteExpiresAt.After(now) || !presented.ExpiresAt.After(now) {
		return fmt.Errorf("%s: live state changed", refreshRotationRepoName)
	}
	return nil
}

func refreshRotationResult(
	outcome entity.RefreshRotationOutcome,
	receiptID uuid.UUID,
	replacementTokenID uuid.UUID,
	session *generate.AuthSession,
	owner *generate.User,
	refreshExpiresAt time.Time,
	ciphertext []byte,
) entity.RefreshRotationResult {
	return entity.RefreshRotationResult{
		Outcome:            outcome,
		ReceiptID:          receiptID,
		ReplacementTokenID: replacementTokenID,
		SessionID:          session.ID,
		UserID:             owner.ID,
		Username:           owner.Username,
		SecurityVersion:    uint64(owner.SecurityVersion),
		AuthenticatedAt:    session.LastReauthenticatedAt.UTC(),
		AbsoluteExpiresAt:  session.AbsoluteExpiresAt.UTC(),
		IdleExpiresAt:      session.IdleExpiresAt.UTC(),
		RefreshExpiresAt:   refreshExpiresAt.UTC(),
		RecoveryCiphertext: append([]byte(nil), ciphertext...),
	}
}

func (repository *RefreshRotationRepository) MarkReceiptRecovered(
	ctx context.Context,
	receiptID uuid.UUID,
	recoveredAt time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil ||
		receiptID == uuid.Nil || recoveredAt.IsZero() {
		return fmt.Errorf("%s: invalid recovery marker", refreshRotationRepoName)
	}
	_, err := repository.client.DB(ctx).AuthRefreshReceipt.Update().Where(
		authrefreshreceipt.ID(receiptID),
		authrefreshreceipt.ErasedAtIsNil(),
		authrefreshreceipt.ExpiresAtGT(recoveredAt.UTC()),
	).SetRecoveredAt(recoveredAt.UTC()).Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, refreshRotationRepoName)
	}
	return nil
}

var _ ports.RefreshRotationRepository = (*RefreshRotationRepository)(nil)
