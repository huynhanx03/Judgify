package service

import (
	"context"
	"net/http"
	"strconv"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
	"github.com/huynhanx03/judgify/internal/identity/utils"
)

const userServiceName = "UserService"

type userService struct {
	userRepo      ports.UserRepository
	credentialRepo ports.CredentialRepository
	roleRepo      ports.RoleRepository
	attrDefRepo   ports.AttributeDefinitionRepository
	attrValueRepo ports.UserAttributeValueRepository
	cache         cache.LocalCache[string, any]
}

// NewUserService creates a new UserService instance.
func NewUserService(
	userRepo ports.UserRepository,
	credentialRepo ports.CredentialRepository,
	roleRepo ports.RoleRepository,
	attrDefRepo ports.AttributeDefinitionRepository,
	attrValueRepo ports.UserAttributeValueRepository,
	cache cache.LocalCache[string, any],
) ports.UserService {
	return &userService{
		userRepo:      userRepo,
		credentialRepo: credentialRepo,
		roleRepo:      roleRepo,
		attrDefRepo:   attrDefRepo,
		attrValueRepo: attrValueRepo,
		cache:         cache,
	}
}

// Delete deletes a user by ID.
func (s *userService) Delete(ctx context.Context, id int) error {
	exists, err := s.userRepo.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return apperr.NewError(userServiceName, response.CodeNotFound, apperr.MsgNotFound, http.StatusNotFound, nil)
	}

	return s.userRepo.Delete(ctx, id)
}

// UpdateProfile updates user profile attributes.
func (s *userService) UpdateProfile(ctx context.Context, userID int, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
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

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
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

	gender, _ := strconv.Atoi(attrUpdates[constant.AttributeKeyGender])

	return &dto.ProfileResponse{
		Username:  user.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Gender:    gender,
		Birthday:  req.Birthday,
	}, nil
}

// GetProfile gets user profile with attributes.
func (s *userService) GetProfile(ctx context.Context, userID int) (*dto.ProfileResponse, error) {
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

	return &dto.ProfileResponse{
		Username:  user.Username,
		FirstName: firstNameVal,
		LastName:  lastNameVal,
		Gender:    genderInt,
		Birthday:  birthdayVal,
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
