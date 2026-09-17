package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func (s *authenticationService) OAuthIdentities(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.OAuthIdentityListResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || userID == uuid.Nil ||
		isNilTokenDependency(s.fedIdentityRepo) {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth identity query", nil)
	}
	identities, err := s.fedIdentityRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]dto.OAuthIdentityResponse, 0, len(identities))
	for _, identity := range identities {
		if identity == nil || identity.ID == uuid.Nil || identity.UserID != userID ||
			!configuredOAuthProvider(s, identity.Provider) {
			return nil, apperr.New(apperr.CodeInternalServer, "invalid oauth identity projection", nil)
		}
		items = append(items, dto.OAuthIdentityResponse{
			Provider:            identity.Provider,
			DisplayName:         identity.DisplayName,
			AvatarURL:           identity.AvatarURL,
			LastAuthenticatedAt: identity.LastAuthenticatedAt,
			LinkedAt:            identity.CreatedAt,
		})
	}
	logger.FromContext(ctx).Debug(
		"oauth identities projected",
		zap.String("user_id", userID.String()),
		zap.Int("identity_count", len(items)),
	)
	return &dto.OAuthIdentityListResponse{Items: items}, nil
}

func (s *authenticationService) OAuthUnlink(
	ctx context.Context,
	userID, sessionID uuid.UUID,
	provider string,
) (*dto.OAuthFinalizeLinkResponse, error) {
	provider = normalizedOAuthProvider(provider)
	if s == nil || ctx == nil || ctx.Err() != nil || userID == uuid.Nil ||
		sessionID == uuid.Nil || !configuredOAuthProvider(s, provider) ||
		isNilTokenDependency(s.fedIdentityRepo) || isNilTokenDependency(s.txMgr) ||
		isNilTokenDependency(s.journal) {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid oauth unlink request", nil)
	}
	mutationID, err := uuid.NewV7()
	if err != nil {
		return nil, oauthInternalFailure()
	}
	now := time.Now().UTC()
	err = s.txMgr.DoInTx(ctx, func(transactionContext context.Context) error {
		identity, unlinkErr := s.fedIdentityRepo.Unlink(
			transactionContext, userID, sessionID, provider, now,
		)
		if unlinkErr != nil {
			return unlinkErr
		}
		if identity == nil || identity.ID == uuid.Nil || identity.UserID != userID ||
			identity.Provider != provider {
			return ports.ErrFederatedIdentityNotFound
		}
		identityID := identity.ID
		actorSessionID := sessionID
		return s.journal.Append(transactionContext, entity.SecurityMutation{
			ID: mutationID, Kind: entity.SecurityMutationOAuthIdentityUnlinked,
			UserID: userID, SessionID: &actorSessionID,
			FederatedIdentityID: &identityID, Provider: provider,
			OccurredAt: now,
		})
	})
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrLastAuthenticationMethod):
			return nil, apperr.New(
				apperr.CodeConflict,
				"cannot remove the last authentication method",
				err,
			)
		case errors.Is(err, ports.ErrFederatedIdentityNotFound):
			return nil, apperr.New(apperr.CodeNotFound, "oauth identity not found", err)
		default:
			return nil, err
		}
	}
	logger.FromContext(ctx).Info(
		"oauth identity unlinked",
		zap.String("user_id", userID.String()),
		zap.String("session_id", sessionID.String()),
		zap.String("provider", provider),
	)
	return &dto.OAuthFinalizeLinkResponse{Success: true}, nil
}

func configuredOAuthProvider(s *authenticationService, provider string) bool {
	if s == nil || provider == "" || strings.TrimSpace(provider) != provider {
		return false
	}
	configured, ok := s.oauthProviders[provider]
	return ok && configured.Provider != nil && configured.RedirectURI != ""
}
