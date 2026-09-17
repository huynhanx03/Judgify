package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authrefreshtoken"
	"github.com/huynhanx03/judgify/internal/ent/generate/authsession"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const refreshTokenRepoName = "refresh token"

const refreshTokenHashBytes = 32

// RefreshTokenRepository persists only opaque-token hashes and lineage.
type RefreshTokenRepository struct {
	client *dbent.EntClient
}

func NewRefreshTokenRepository(client *dbent.EntClient) ports.RefreshTokenRepository {
	return &RefreshTokenRepository{client: client}
}

func (repository *RefreshTokenRepository) Create(ctx context.Context, token *entity.RefreshToken) error {
	if repository == nil || repository.client == nil || ctx == nil || token == nil {
		return fmt.Errorf("%s: invalid create input", refreshTokenRepoName)
	}
	if err := validateRefreshTokenForCreate(token); err != nil {
		return err
	}
	status := token.Status
	if status == "" {
		status = entity.RefreshTokenStatusIssued
	}
	create := repository.client.DB(ctx).AuthRefreshToken.Create().
		SetID(token.ID).
		SetSessionID(token.SessionID).
		SetTokenHash(append([]byte(nil), token.TokenHash...)).
		SetNillableParentID(token.ParentID).
		SetStatus(authrefreshtoken.Status(status)).
		SetIssuedAt(token.IssuedAt.UTC()).
		SetExpiresAt(token.ExpiresAt.UTC()).
		SetNillableConsumedAt(utcTimePointer(token.ConsumedAt)).
		SetNillableReplacedByID(token.ReplacedByID).
		SetNillableRevokedAt(utcTimePointer(token.RevokedAt))
	if !token.CreatedAt.IsZero() {
		create.SetCreatedAt(token.CreatedAt.UTC())
	}
	if !token.UpdatedAt.IsZero() {
		create.SetUpdatedAt(token.UpdatedAt.UTC())
	}
	record, err := create.Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, refreshTokenRepoName)
	}
	mapRefreshToken(record, token)
	return nil
}

func (repository *RefreshTokenRepository) Get(ctx context.Context, id uuid.UUID) (*entity.RefreshToken, error) {
	if repository == nil || repository.client == nil || ctx == nil || id == uuid.Nil {
		return nil, fmt.Errorf("%s: invalid lookup", refreshTokenRepoName)
	}
	record, err := repository.client.DB(ctx).AuthRefreshToken.Get(ctx, id)
	if err != nil {
		return nil, commonent.MapEntError(err, refreshTokenRepoName)
	}
	return refreshTokenEntity(record), nil
}

func (repository *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash []byte) (*entity.RefreshToken, error) {
	if repository == nil || repository.client == nil || ctx == nil || len(tokenHash) != refreshTokenHashBytes {
		return nil, fmt.Errorf("%s: invalid hash lookup", refreshTokenRepoName)
	}
	record, err := repository.client.DB(ctx).AuthRefreshToken.Query().
		Where(authrefreshtoken.TokenHashEQ(append([]byte(nil), tokenHash...))).
		Only(ctx)
	if err != nil {
		return nil, commonent.MapEntError(err, refreshTokenRepoName)
	}
	return refreshTokenEntity(record), nil
}

func (repository *RefreshTokenRepository) RevokeIssuedForSession(
	ctx context.Context,
	sessionID uuid.UUID,
	at time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil || sessionID == uuid.Nil || at.IsZero() {
		return fmt.Errorf("%s: invalid revoke", refreshTokenRepoName)
	}
	_, err := repository.client.DB(ctx).AuthRefreshToken.Update().
		Where(
			authrefreshtoken.SessionID(sessionID),
			authrefreshtoken.StatusEQ(authrefreshtoken.StatusIssued),
		).
		SetStatus(authrefreshtoken.StatusRevoked).
		SetRevokedAt(at.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, refreshTokenRepoName)
	}
	return nil
}

func (repository *RefreshTokenRepository) RevokeIssuedForUser(
	ctx context.Context,
	userID uuid.UUID,
	at time.Time,
) error {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil || at.IsZero() {
		return fmt.Errorf("%s: invalid revoke-user", refreshTokenRepoName)
	}
	_, err := repository.client.DB(ctx).AuthRefreshToken.Update().
		Where(
			authrefreshtoken.StatusEQ(authrefreshtoken.StatusIssued),
			authrefreshtoken.HasSessionWith(authsession.UserID(userID)),
		).
		SetStatus(authrefreshtoken.StatusRevoked).
		SetRevokedAt(at.UTC()).
		Save(ctx)
	if err != nil {
		return commonent.MapEntError(err, refreshTokenRepoName)
	}
	return nil
}

func refreshTokenEntity(record *generate.AuthRefreshToken) *entity.RefreshToken {
	if record == nil {
		return nil
	}
	result := &entity.RefreshToken{}
	mapRefreshToken(record, result)
	return result
}

func mapRefreshToken(record *generate.AuthRefreshToken, destination *entity.RefreshToken) {
	if record == nil || destination == nil {
		return
	}
	destination.ID = record.ID
	destination.SessionID = record.SessionID
	destination.TokenHash = append([]byte(nil), record.TokenHash...)
	destination.ParentID = copyUUIDPointer(record.ParentID)
	destination.Status = entity.RefreshTokenStatus(record.Status)
	destination.IssuedAt = record.IssuedAt.UTC()
	destination.ExpiresAt = record.ExpiresAt.UTC()
	destination.ConsumedAt = utcTimePointer(record.ConsumedAt)
	destination.ReplacedByID = copyUUIDPointer(record.ReplacedByID)
	destination.RevokedAt = utcTimePointer(record.RevokedAt)
	destination.CreatedAt = record.CreatedAt.UTC()
	destination.UpdatedAt = record.UpdatedAt.UTC()
}

func copyUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func validateRefreshTokenForCreate(token *entity.RefreshToken) error {
	if token.ID == uuid.Nil || token.SessionID == uuid.Nil || len(token.TokenHash) != refreshTokenHashBytes ||
		token.IssuedAt.IsZero() || token.ExpiresAt.IsZero() || !token.ExpiresAt.After(token.IssuedAt) {
		return fmt.Errorf("%s: invalid create input", refreshTokenRepoName)
	}
	status := token.Status
	if status == "" {
		status = entity.RefreshTokenStatusIssued
	}
	switch status {
	case entity.RefreshTokenStatusIssued:
		if token.ConsumedAt != nil || token.ReplacedByID != nil || token.RevokedAt != nil {
			return fmt.Errorf("%s: inconsistent issued state", refreshTokenRepoName)
		}
	case entity.RefreshTokenStatusConsumed:
		if token.ConsumedAt == nil || token.ReplacedByID == nil || token.RevokedAt != nil {
			return fmt.Errorf("%s: incomplete consumed state", refreshTokenRepoName)
		}
	case entity.RefreshTokenStatusRevoked:
		if token.RevokedAt == nil || token.ConsumedAt != nil || token.ReplacedByID != nil {
			return fmt.Errorf("%s: incomplete revoked state", refreshTokenRepoName)
		}
	default:
		return fmt.Errorf("%s: invalid status", refreshTokenRepoName)
	}
	return nil
}

var _ ports.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
