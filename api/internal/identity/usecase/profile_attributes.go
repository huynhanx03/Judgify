package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/identity/attributevalue"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func (s *userService) GetProfileAttributes(
	ctx context.Context,
	userID uuid.UUID,
) (*dto.ProfileAttributesResponse, error) {
	reader, _, err := s.profileAttributeValuePlane()
	if err != nil || ctx == nil || userID == uuid.Nil {
		if err != nil {
			return nil, err
		}
		return nil, apperr.New(apperr.CodeBadRequest, "invalid profile request", nil)
	}
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	attributes, err := reader.ListActiveProfileAttributes(ctx, userID)
	if err != nil {
		return nil, err
	}
	response, err := buildProfileAttributesResponse(user, attributes)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Debug(
		"self profile attributes loaded",
		zap.String("user_id", userID.String()),
		zap.Int("attribute_count", len(response.Attributes)),
	)
	return response, nil
}

func (s *userService) UpdateProfileAttributes(
	ctx context.Context,
	userID uuid.UUID,
	req *dto.UpdateProfileAttributesRequest,
) (*dto.ProfileAttributesResponse, error) {
	reader, writer, err := s.profileAttributeValuePlane()
	if err != nil {
		return nil, err
	}
	if isNilTokenDependency(s.txMgr) {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"profile attribute mutation plane unavailable",
			nil,
		)
	}
	if ctx == nil || userID == uuid.Nil || req == nil ||
		len(req.Mutations) == 0 || len(req.Mutations) > 256 {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid profile attribute mutation", nil)
	}

	var response *dto.ProfileAttributesResponse
	err = s.txMgr.DoInTx(ctx, func(transactionContext context.Context) error {
		user, getErr := s.userRepo.Get(transactionContext, userID)
		if getErr != nil {
			return getErr
		}
		current, listErr := reader.ListActiveProfileAttributes(transactionContext, userID)
		if listErr != nil {
			return listErr
		}
		mutations, planErr := planSelfProfileAttributeMutations(userID, req.Mutations, current)
		if planErr != nil {
			logger.FromContext(transactionContext).Debug(
				"self profile attribute mutation rejected during planning",
				zap.String("user_id", userID.String()),
				zap.Int("mutation_count", len(req.Mutations)),
				zap.Error(planErr),
			)
			return planErr
		}
		logger.FromContext(transactionContext).Debug(
			"self profile attribute mutation planned",
			zap.String("user_id", userID.String()),
			zap.Int("mutation_count", len(mutations)),
		)
		for _, mutation := range mutations {
			if _, writeErr := writer.ApplyActiveMutation(transactionContext, mutation); writeErr != nil {
				logger.FromContext(transactionContext).Debug(
					"self profile attribute mutation rejected during persistence",
					zap.String("user_id", userID.String()),
					zap.String("definition_id", mutation.DefinitionID.String()),
					zap.Error(writeErr),
				)
				return mapSelfProfileAttributeMutationError(writeErr, mutation.DefinitionID)
			}
		}
		updated, listErr := reader.ListActiveProfileAttributes(transactionContext, userID)
		if listErr != nil {
			return listErr
		}
		response, listErr = buildProfileAttributesResponse(user, updated)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"self profile attributes updated",
		zap.String("user_id", userID.String()),
		zap.Int("mutation_count", len(req.Mutations)),
	)
	return response, nil
}

func (s *userService) profileAttributeValuePlane() (
	ports.ActiveProfileAttributeValueReader,
	ports.RevisionedUserAttributeValueWriter,
	error,
) {
	if s == nil || isNilTokenDependency(s.attrValueRepo) {
		return nil, nil, apperr.New(
			apperr.CodeInternalServer,
			"profile attribute value plane unavailable",
			ports.ErrUserAttributeValueWriterUnavailable,
		)
	}
	return s.attrValueRepo, s.attrValueRepo, nil
}

func planSelfProfileAttributeMutations(
	userID uuid.UUID,
	requested []dto.ProfileAttributeMutation,
	current []ports.ActiveProfileAttributeValue,
) ([]ports.RevisionedUserAttributeValueMutation, error) {
	byDefinition := make(map[uuid.UUID]ports.ActiveProfileAttributeValue, len(current))
	willHaveValue := make(map[uuid.UUID]bool, len(current))
	for _, attribute := range current {
		if err := validateActiveProfileAttributeProjection(attribute); err != nil {
			return nil, apperr.New(apperr.CodeInternalServer, "invalid profile schema projection", nil)
		}
		if _, duplicate := byDefinition[attribute.DefinitionID]; duplicate {
			return nil, apperr.New(apperr.CodeInternalServer, "invalid profile schema projection", nil)
		}
		byDefinition[attribute.DefinitionID] = attribute
		willHaveValue[attribute.DefinitionID] = attribute.HasValue
	}

	ordered := append([]dto.ProfileAttributeMutation(nil), requested...)
	sort.Slice(ordered, func(left, right int) bool {
		return bytes.Compare(ordered[left].DefinitionID[:], ordered[right].DefinitionID[:]) < 0
	})
	result := make([]ports.RevisionedUserAttributeValueMutation, 0, len(ordered))
	var previous uuid.UUID
	for index, request := range ordered {
		if request.DefinitionID == uuid.Nil ||
			request.ExpectedDefinitionRevisionID == uuid.Nil ||
			request.ExpectedValueVersion < 0 ||
			request.Unset == (len(request.Value) > 0) {
			return nil, invalidSelfProfileAttributeMutation(request.DefinitionID)
		}
		if index > 0 && request.DefinitionID == previous {
			return nil, apperr.New(
				apperr.CodeBadRequest,
				"duplicate profile attribute mutation",
				nil,
			).WithDetails(map[string]any{
				"reason":        "duplicate_definition",
				"definition_id": request.DefinitionID.String(),
			})
		}
		previous = request.DefinitionID
		attribute, exists := byDefinition[request.DefinitionID]
		if !exists || attribute.Visibility == entity.AttributeVisibilityAdmin ||
			attribute.DefinitionRevisionID != request.ExpectedDefinitionRevisionID {
			return nil, profileAttributeConflict(
				"profile schema changed; reload and try again",
				"profile_schema_changed",
				request.DefinitionID,
				ports.ErrUserAttributeDefinitionRevisionConflict,
			)
		}
		if !attribute.UserEditable {
			return nil, profileAttributeConflict(
				"profile attribute is not user editable",
				"attribute_not_editable",
				request.DefinitionID,
				ports.ErrUserAttributeDefinitionRevisionConflict,
			)
		}
		expectedCurrentVersion := int64(0)
		if attribute.HasValue {
			expectedCurrentVersion = attribute.ValueVersion
		}
		if request.ExpectedValueVersion != expectedCurrentVersion {
			return nil, profileAttributeConflict(
				"profile value changed; reload and try again",
				"profile_value_changed",
				request.DefinitionID,
				ports.ErrUserAttributeValueConflict,
			)
		}
		mutation := ports.RevisionedUserAttributeValueMutation{
			UserID: userID, DefinitionID: request.DefinitionID,
			ExpectedDefinitionRevisionID: request.ExpectedDefinitionRevisionID,
			ExpectedValueVersion:         request.ExpectedValueVersion,
			Unset:                        request.Unset,
		}
		if !request.Unset {
			canonical, canonicalErr := attributevalue.Canonicalize(
				request.Value,
				profileAttributeRevision(attribute),
			)
			if canonicalErr != nil {
				return nil, apperr.New(
					apperr.CodeParamInvalid,
					"invalid profile attribute value",
					canonicalErr,
				).WithDetails(map[string]any{
					"reason":        "invalid_attribute_value",
					"definition_id": request.DefinitionID.String(),
				})
			}
			mutation.CanonicalValue = canonical
		}
		willHaveValue[request.DefinitionID] = !request.Unset
		result = append(result, mutation)
	}
	for _, attribute := range current {
		if attribute.RequiredOnOnboarding && !willHaveValue[attribute.DefinitionID] {
			return nil, apperr.New(
				apperr.CodeParamInvalid,
				"required profile attribute value is missing",
				nil,
			).WithDetails(map[string]any{
				"reason":        "required_profile_value_missing",
				"definition_id": attribute.DefinitionID.String(),
			})
		}
	}
	return result, nil
}

func buildProfileAttributesResponse(
	user *entity.User,
	attributes []ports.ActiveProfileAttributeValue,
) (*dto.ProfileAttributesResponse, error) {
	if user == nil || user.ID == uuid.Nil {
		return nil, apperr.New(apperr.CodeInternalServer, "invalid profile projection", nil)
	}
	response := &dto.ProfileAttributesResponse{
		Profile: dto.ProfileAttrs{
			ID: user.ID, Username: user.Username,
			JoinedAt: user.CreatedAt.UTC().Format(time.RFC3339),
		},
		Attributes: make([]dto.ProfileAttributeValueResponse, 0, len(attributes)),
	}
	for _, attribute := range attributes {
		if err := validateActiveProfileAttributeProjection(attribute); err != nil {
			return nil, apperr.New(apperr.CodeInternalServer, "invalid profile schema projection", err)
		}
		revision := profileAttributeRevision(attribute)
		item := dto.ProfileAttributeValueResponse{
			DefinitionID:         attribute.DefinitionID,
			DefinitionRevisionID: attribute.DefinitionRevisionID,
			Key:                  attribute.Key, Kind: attribute.Kind,
			RequiredOnOnboarding: attribute.RequiredOnOnboarding,
			DataType:             string(attribute.DataType), Label: attribute.Label,
			Description:  attribute.Description,
			Visibility:   string(attribute.Visibility),
			UserEditable: attribute.UserEditable,
			DisplayOrder: attribute.DisplayOrder,
			Validation:   append([]byte(nil), attribute.Validation...),
			HasValue:     attribute.HasValue,
			ValueVersion: attribute.ValueVersion,
		}
		if attribute.HasValue {
			canonical, err := attributevalue.Canonicalize(attribute.CanonicalValue, revision)
			if err != nil || !bytes.Equal(canonical, attribute.CanonicalValue) {
				return nil, apperr.New(apperr.CodeInternalServer, "invalid profile value projection", err)
			}
			item.Value = append(json.RawMessage(nil), canonical...)
			legacy, err := attributevalue.LegacyProjection(
				canonical,
				attribute.DataType,
			)
			if err != nil {
				return nil, apperr.New(apperr.CodeInternalServer, "invalid profile value projection", err)
			}
			switch attribute.Key {
			case constant.AttributeKeyFirstName:
				response.Profile.FirstName = legacy
			case constant.AttributeKeyLastName:
				response.Profile.LastName = legacy
			case constant.AttributeKeyGender:
				gender, parseErr := strconv.Atoi(legacy)
				if parseErr != nil || gender < 0 || gender > 2 {
					return nil, apperr.New(
						apperr.CodeInternalServer,
						"invalid fixed profile projection",
						parseErr,
					)
				}
				response.Profile.Gender = gender
			case constant.AttributeKeyBirthday:
				response.Profile.Birthday = legacy
			}
		}
		response.Attributes = append(response.Attributes, item)
	}
	sort.Slice(response.Attributes, func(left, right int) bool {
		if response.Attributes[left].DisplayOrder != response.Attributes[right].DisplayOrder {
			return response.Attributes[left].DisplayOrder < response.Attributes[right].DisplayOrder
		}
		if response.Attributes[left].Key != response.Attributes[right].Key {
			return response.Attributes[left].Key < response.Attributes[right].Key
		}
		return bytes.Compare(
			response.Attributes[left].DefinitionID[:],
			response.Attributes[right].DefinitionID[:],
		) < 0
	})
	return response, nil
}

func validateActiveProfileAttributeProjection(
	attribute ports.ActiveProfileAttributeValue,
) error {
	if attribute.DefinitionID == uuid.Nil || attribute.DefinitionRevisionID == uuid.Nil ||
		(attribute.Kind != "system" && attribute.Kind != "custom") ||
		strings.TrimSpace(attribute.Key) != attribute.Key ||
		!attributeDefinitionKeyPattern.MatchString(attribute.Key) ||
		strings.TrimSpace(attribute.Label) != attribute.Label ||
		len(attribute.Label) < 2 || len(attribute.Label) > 128 ||
		len(attribute.Description) > 1024 ||
		attribute.DisplayOrder < 0 || attribute.DisplayOrder > 10000 ||
		!attribute.Visibility.Valid() || attribute.Visibility == entity.AttributeVisibilityAdmin ||
		(attribute.RequiredOnOnboarding && !attribute.UserEditable) ||
		attributevalue.ValidateRevision(profileAttributeRevision(attribute)) != nil {
		return errors.New("invalid active profile attribute definition")
	}
	if attribute.HasValue {
		if len(attribute.CanonicalValue) == 0 || attribute.ValueVersion < 1 {
			return errors.New("invalid active profile attribute value")
		}
		return nil
	}
	if len(attribute.CanonicalValue) != 0 || attribute.ValueVersion != 0 {
		return errors.New("inconsistent empty profile attribute value")
	}
	return nil
}

func profileAttributeRevision(
	attribute ports.ActiveProfileAttributeValue,
) entity.AttributeDefinitionRevision {
	return entity.AttributeDefinitionRevision{
		ID:           attribute.DefinitionRevisionID,
		DefinitionID: attribute.DefinitionID,
		DataType:     attribute.DataType,
		Label:        attribute.Label,
		Description:  attribute.Description,
		Visibility:   attribute.Visibility,
		UserEditable: attribute.UserEditable,
		DisplayOrder: attribute.DisplayOrder,
		Validation:   append([]byte(nil), attribute.Validation...),
	}
}

func mapSelfProfileAttributeMutationError(err error, definitionID uuid.UUID) error {
	switch {
	case errors.Is(err, ports.ErrUserAttributeMigrationActive):
		return profileAttributeConflict(
			"profile attribute migration is active; try again after it completes",
			"migration_active",
			definitionID,
			err,
		)
	case errors.Is(err, ports.ErrUserAttributeMigrationConflict):
		return profileAttributeConflict(
			"profile value does not satisfy the migration target; reload and try again",
			"migration_conflict",
			definitionID,
			err,
		)
	case errors.Is(err, ports.ErrUserAttributeDefinitionRevisionConflict):
		return profileAttributeConflict(
			"profile schema changed; reload and try again",
			"profile_schema_changed",
			definitionID,
			err,
		)
	case errors.Is(err, ports.ErrUserAttributeValueConflict):
		return profileAttributeConflict(
			"profile value changed; reload and try again",
			"profile_value_changed",
			definitionID,
			err,
		)
	case errors.Is(err, attributevalue.ErrInvalidValue):
		return apperr.New(
			apperr.CodeParamInvalid,
			"invalid profile attribute value",
			err,
		).WithDetails(map[string]any{
			"reason":        "invalid_attribute_value",
			"definition_id": definitionID.String(),
		})
	default:
		return err
	}
}

func invalidSelfProfileAttributeMutation(definitionID uuid.UUID) error {
	details := map[string]any{"reason": "invalid_attribute_mutation"}
	if definitionID != uuid.Nil {
		details["definition_id"] = definitionID.String()
	}
	return apperr.New(
		apperr.CodeBadRequest,
		"invalid profile attribute mutation",
		nil,
	).WithDetails(details)
}

func profileAttributeConflict(
	message string,
	reason string,
	definitionID uuid.UUID,
	cause error,
) error {
	return apperr.New(apperr.CodeConflict, message, cause).WithDetails(map[string]any{
		"reason":        reason,
		"definition_id": definitionID.String(),
	})
}
