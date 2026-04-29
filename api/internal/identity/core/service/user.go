package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/common/tx"
	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
	"github.com/huynhanx03/judgify/internal/identity/utils"
)


type userService struct {
	userRepo       ports.UserRepository
	credentialRepo ports.CredentialRepository
	roleRepo       ports.RoleRepository
	attrDefRepo    ports.AttributeDefinitionRepository
	attrValueRepo  ports.UserAttributeValueRepository
	cache          cache.LocalCache[string, any]
	txMgr          tx.Manager
}

// NewUserService creates a new UserService instance.
func NewUserService(
	userRepo ports.UserRepository,
	credentialRepo ports.CredentialRepository,
	roleRepo ports.RoleRepository,
	attrDefRepo ports.AttributeDefinitionRepository,
	attrValueRepo ports.UserAttributeValueRepository,
	cache cache.LocalCache[string, any],
	txMgr tx.Manager,
) ports.UserService {
	return &userService{
		userRepo:       userRepo,
		credentialRepo: credentialRepo,
		roleRepo:       roleRepo,
		attrDefRepo:    attrDefRepo,
		attrValueRepo:  attrValueRepo,
		cache:          cache,
		txMgr:          txMgr,
	}
}

// Find retrieves users with pagination.
func (s *userService) Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserResponse], error) {
	result, err := s.userRepo.Find(ctx, opts)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.UserResponse, len(*result.Records))
	for i, u := range *result.Records {
		responses[i] = &dto.UserResponse{
			ID:        u.ID,
			Username:  u.Username,
			RoleID:    u.RoleID,
			RoleName:  u.RoleName,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return &d.Paginated[*dto.UserResponse]{
		Records:    &responses,
		Pagination: result.Pagination,
	}, nil
}

// UpdateUser updates a user's role.
func (s *userService) UpdateUser(ctx context.Context, req *dto.UpdateUserRequest) (*dto.UserResponse, error) {
	user, err := s.userRepo.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	role, err := s.roleRepo.Get(ctx, req.RoleID)
	if err != nil {
		return nil, err
	}

	user.RoleID = req.RoleID
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("user role updated", zap.Int("user_id", req.ID), zap.Int("role_id", req.RoleID))
	return &dto.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		RoleID:    user.RoleID,
		RoleName:  role.Name,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

// Delete deletes a user by ID.
func (s *userService) Delete(ctx context.Context, id int) error {
	exists, err := s.userRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.New(response.CodeNotFound, fmt.Sprintf(apperr.MsgNotFound, constant.ObjUser), nil)
	}

	if err := s.userRepo.Delete(ctx, id); err != nil {
		return err
	}

	logger.FromContext(ctx).Info("user deleted successfully", zap.Int("user_id", id))
	return nil
}

// UpdateProfile updates user profile attributes.
func (s *userService) UpdateProfile(ctx context.Context, userID int, req *dto.UpdateProfileRequest) (*dto.ProfileAttrs, error) {
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	attrUpdates := map[string]string{
		constant.AttributeKeyFirstName: req.FirstName,
		constant.AttributeKeyLastName:  req.LastName,
		constant.AttributeKeyGender:    strconv.Itoa(req.Gender),
		constant.AttributeKeyBirthday:  req.Birthday,
	}

	err = s.txMgr.DoInTx(ctx, func(ctx context.Context) error {
		existingAttrs, err := s.attrValueRepo.GetByUserID(ctx, userID)
		if err != nil {
			return err
		}

		existingAttrMap := make(map[int]*entity.UserAttributeValue)
		for _, attr := range existingAttrs {
			existingAttrMap[attr.AttributeID] = attr
		}

		var newAttrs []*entity.UserAttributeValue
		var updateAttrs []*entity.UserAttributeValue

		for key, value := range attrUpdates {
			if value == "" {
				continue
			}

			def, err := utils.GetAttributeDefinition(ctx, key, s.attrDefRepo, s.cache)
			if err != nil {
				return err
			}

			if existingAttr, ok := existingAttrMap[def.ID]; ok {
				if existingAttr.Value != value {
					existingAttr.Value = value
					updateAttrs = append(updateAttrs, existingAttr)
				}
			} else {
				newAttrs = append(newAttrs, &entity.UserAttributeValue{
					UserID:      user.ID,
					AttributeID: def.ID,
					Value:       value,
				})
			}
		}

		if len(updateAttrs) > 0 {
			if err := s.attrValueRepo.UpdateBulk(ctx, updateAttrs); err != nil {
				return err
			}
		}

		if len(newAttrs) > 0 {
			if err := s.attrValueRepo.CreateBulk(ctx, newAttrs); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("user profile updated successfully", zap.Int("user_id", userID))

	gender, _ := strconv.Atoi(attrUpdates[constant.AttributeKeyGender])

	return &dto.ProfileAttrs{
		Username:  user.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Gender:    gender,
		Birthday:  req.Birthday,
	}, nil
}

// GetProfile gets user profile with attributes.
func (s *userService) GetProfile(ctx context.Context, userID int) (*dto.ProfileAttrs, error) {
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	existingAttrs, err := s.attrValueRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	firstNameVal := utils.GetAttributeValue(ctx, constant.AttributeKeyFirstName, existingAttrs, s.attrDefRepo, s.cache)
	lastNameVal := utils.GetAttributeValue(ctx, constant.AttributeKeyLastName, existingAttrs, s.attrDefRepo, s.cache)
	genderVal := utils.GetAttributeValue(ctx, constant.AttributeKeyGender, existingAttrs, s.attrDefRepo, s.cache)
	birthdayVal := utils.GetAttributeValue(ctx, constant.AttributeKeyBirthday, existingAttrs, s.attrDefRepo, s.cache)

	genderInt, _ := strconv.Atoi(genderVal)

	return &dto.ProfileAttrs{
		Username:  user.Username,
		FirstName: firstNameVal,
		LastName:  lastNameVal,
		Gender:    genderInt,
		Birthday:  birthdayVal,
		JoinedAt:  user.CreatedAt.Format("2006-01-02"),
	}, nil
}

// GetRole gets the role for a user.
func (s *userService) GetRole(ctx context.Context, userID int) (*dto.RoleResponse, error) {
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	role, err := s.roleRepo.Get(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	return &dto.RoleResponse{
		ID:    role.ID,
		Name:  role.Name,
		Level: role.Level,
	}, nil
}
