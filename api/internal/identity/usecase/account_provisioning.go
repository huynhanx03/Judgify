package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

// persistPreparedAccount is the single transaction-bound write order shared
// by public registration and administrative invitation. A nil credential is
// legal only for a non-authenticating invited account.
func persistPreparedAccount(
	ctx context.Context,
	userRepository ports.UserRepository,
	credentialRepository ports.CredentialRepository,
	attributeRepository ports.UserAttributeValueRepository,
	authorization ports.AuthorizationService,
	user *entity.User,
	credential *entity.Credential,
	mutations []ports.RevisionedUserAttributeValueMutation,
	roles []string,
) error {
	if ctx == nil || ctx.Err() != nil || userRepository == nil ||
		credentialRepository == nil || authorization == nil || user == nil ||
		user.ID == uuid.Nil || len(roles) == 0 ||
		(credential == nil && user.SecurityStatus != entity.UserSecurityStatusInvited) ||
		(credential != nil && credential.UserID != user.ID) {
		return apperr.New(apperr.CodeInternalServer, "invalid prepared account", nil)
	}
	if err := userRepository.Create(ctx, user); err != nil {
		return err
	}
	if credential != nil {
		if err := credentialRepository.Create(ctx, credential); err != nil {
			return err
		}
	}
	if len(mutations) > 0 {
		if attributeRepository == nil {
			return ports.ErrUserAttributeValueWriterUnavailable
		}
		for _, mutation := range mutations {
			if mutation.UserID != user.ID || mutation.Unset {
				return ports.ErrInvalidUserAttributeValue
			}
			if _, err := attributeRepository.ApplyActiveMutation(ctx, mutation); err != nil {
				return err
			}
		}
	}
	if err := authorization.SetUserRoles(ctx, user.ID, roles); err != nil {
		return apperr.New(apperr.CodeDatabaseError, "failed to assign user roles", err)
	}
	return nil
}
