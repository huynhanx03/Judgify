package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	d "github.com/huynhanx03/go-common/pkg/dto"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

type userService struct {
	userRepo       ports.UserRepository
	roleRepo       ports.RoleRepository
	authzService   ports.AuthorizationService
	tokenLifecycle ports.TokenLifecycle
	attrValueRepo  ports.UserAttributeValueRepository
	txMgr          tx.Manager
	lifecycle      *userLifecycleDependencies
}

type UserServiceOption func(*userService)

// NewUserService creates a new UserService instance.
func NewUserService(
	userRepo ports.UserRepository,
	roleRepo ports.RoleRepository,
	authzService ports.AuthorizationService,
	tokenLifecycle ports.TokenLifecycle,
	attrValueRepo ports.UserAttributeValueRepository,
	txMgr tx.Manager,
	options ...UserServiceOption,
) ports.UserService {
	result := &userService{
		userRepo:       userRepo,
		roleRepo:       roleRepo,
		authzService:   authzService,
		tokenLifecycle: tokenLifecycle,
		attrValueRepo:  attrValueRepo,
		txMgr:          txMgr,
	}
	for _, option := range options {
		if option != nil {
			option(result)
		}
	}
	return result
}

// Find retrieves users with pagination.
func (s *userService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserResponse], error) {
	result, err := s.userRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.UserResponse, len(*result.Records))
	for i, u := range *result.Records {
		roleSnapshot, err := s.authzService.GetUserRoleSnapshot(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		responses[i] = &dto.UserResponse{
			ID:                    u.ID,
			Username:              u.Username,
			Status:                string(u.SecurityStatus),
			Version:               int64(u.SecurityVersion),
			SuspendedAt:           u.SuspendedAt,
			Roles:                 roleSnapshot.Roles,
			AuthorizationRevision: roleSnapshot.Revision,
			CreatedAt:             u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:             u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return &d.Paginated[*dto.UserResponse]{
		Records:    &responses,
		Pagination: result.Pagination,
	}, nil
}

// GetProfile gets user profile with attributes.
func (s *userService) GetProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileAttrs, error) {
	projection, err := s.GetProfileAttributes(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile := projection.Profile
	return &profile, nil
}

// GetRole gets the first assigned role for a user.
func (s *userService) GetRole(ctx context.Context, userID uuid.UUID) (*dto.RoleResponse, error) {
	roles, err := s.authzService.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "user has no assigned role", nil)
	}

	role, err := s.roleRepo.GetByKey(ctx, roles[0])
	if err != nil {
		return nil, err
	}

	return ToRoleResponse(role), nil
}
