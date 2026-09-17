package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/constant"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil"
	idMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newUserTestEnv(t *testing.T) (
	*userService,
	*idMocks.MockUserRepository,
	*idMocks.MockRoleRepository,
	*idMocks.MockAuthorizationService,
	*idMocks.MockUserAttributeValueRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	userRepo := idMocks.NewMockUserRepository(ctrl)
	roleRepo := idMocks.NewMockRoleRepository(ctrl)
	authorization := idMocks.NewMockAuthorizationService(ctrl)
	attributeValues := idMocks.NewMockUserAttributeValueRepository(ctrl)

	service := NewUserService(
		userRepo,
		roleRepo,
		authorization,
		authenticationTestTokenLifecycle{},
		attributeValues,
		testutil.TxManager{},
	).(*userService)
	return service, userRepo, roleRepo, authorization, attributeValues
}

func TestGetUserRoleReturnsFirstAssignedRole(t *testing.T) {
	service, _, roleRepo, authorization, _ := newUserTestEnv(t)
	ctx := context.Background()
	userID := testutil.ID(1)

	authorization.EXPECT().GetUserRoles(ctx, userID).Return([]string{"admin"}, nil)
	roleRepo.EXPECT().GetByKey(ctx, "admin").Return(&entity.Role{
		ID: testutil.ID(2), Key: "admin", Name: "Admin",
	}, nil)

	response, err := service.GetRole(ctx, userID)

	require.NoError(t, err)
	require.Equal(t, testutil.ID(2), response.ID)
	require.Equal(t, "admin", response.Key)
	require.Equal(t, "Admin", response.Name)
}

func TestGetUserRoleRejectsUserWithoutAssignment(t *testing.T) {
	service, _, _, authorization, _ := newUserTestEnv(t)
	ctx := context.Background()
	userID := testutil.ID(1)
	authorization.EXPECT().GetUserRoles(ctx, userID).Return(nil, nil)

	response, err := service.GetRole(ctx, userID)

	require.Nil(t, response)
	require.ErrorContains(t, err, "user has no assigned role")
}

func TestGetProfileUsesActiveRevisionedAttributeProjection(t *testing.T) {
	service, userRepo, _, _, attributeValues := newUserTestEnv(t)
	ctx := context.Background()
	userID := testutil.ID(1)
	joinedAt := time.Date(2026, 8, 13, 8, 30, 0, 0, time.UTC)
	attribute := validProfileAttribute(testutil.ID(2), testutil.ID(3))
	attribute.HasValue = true
	attribute.CanonicalValue = json.RawMessage(`"Jerry"`)
	attribute.ValueVersion = 7

	userRepo.EXPECT().Get(ctx, userID).Return(&entity.User{
		ID: userID, Username: "judge-user", CreatedAt: joinedAt,
	}, nil)
	attributeValues.EXPECT().ListActiveProfileAttributes(ctx, userID).Return(
		[]ports.ActiveProfileAttributeValue{attribute},
		nil,
	)

	profile, err := service.GetProfile(ctx, userID)

	require.NoError(t, err)
	require.Equal(t, userID, profile.ID)
	require.Equal(t, "judge-user", profile.Username)
	require.Equal(t, "Jerry", profile.FirstName)
	require.Equal(t, joinedAt.Format(time.RFC3339), profile.JoinedAt)
}

func TestUpdateProfileAttributesWritesCanonicalValueAgainstReviewedRevision(t *testing.T) {
	service, userRepo, _, _, attributeValues := newUserTestEnv(t)
	ctx := context.Background()
	userID := testutil.ID(1)
	definitionID := testutil.ID(2)
	revisionID := testutil.ID(3)
	current := validProfileAttribute(definitionID, revisionID)
	current.HasValue = true
	current.CanonicalValue = json.RawMessage(`"Old name"`)
	current.ValueVersion = 2
	updated := current
	updated.CanonicalValue = json.RawMessage(`"Jerry"`)
	updated.ValueVersion = 3

	userRepo.EXPECT().Get(ctx, userID).Return(&entity.User{
		ID: userID, Username: "judge-user", CreatedAt: time.Now().UTC(),
	}, nil)
	gomock.InOrder(
		attributeValues.EXPECT().ListActiveProfileAttributes(ctx, userID).Return(
			[]ports.ActiveProfileAttributeValue{current},
			nil,
		),
		attributeValues.EXPECT().ApplyActiveMutation(
			ctx,
			ports.RevisionedUserAttributeValueMutation{
				UserID:                       userID,
				DefinitionID:                 definitionID,
				ExpectedDefinitionRevisionID: revisionID,
				ExpectedValueVersion:         2,
				CanonicalValue:               json.RawMessage(`"Jerry"`),
			},
		).Return(&entity.UserAttributeValue{}, nil),
		attributeValues.EXPECT().ListActiveProfileAttributes(ctx, userID).Return(
			[]ports.ActiveProfileAttributeValue{updated},
			nil,
		),
	)

	response, err := service.UpdateProfileAttributes(
		ctx,
		userID,
		&dto.UpdateProfileAttributesRequest{Mutations: []dto.ProfileAttributeMutation{{
			DefinitionID:                 definitionID,
			ExpectedDefinitionRevisionID: revisionID,
			ExpectedValueVersion:         2,
			Value:                        json.RawMessage(`"Jerry"`),
		}}},
	)

	require.NoError(t, err)
	require.Len(t, response.Attributes, 1)
	require.JSONEq(t, `"Jerry"`, string(response.Attributes[0].Value))
	require.Equal(t, int64(3), response.Attributes[0].ValueVersion)
	require.Equal(t, "Jerry", response.Profile.FirstName)
}

func validProfileAttribute(
	definitionID uuid.UUID,
	revisionID uuid.UUID,
) ports.ActiveProfileAttributeValue {
	return ports.ActiveProfileAttributeValue{
		DefinitionID:         definitionID,
		DefinitionRevisionID: revisionID,
		Key:                  constant.AttributeKeyFirstName,
		Kind:                 "system",
		DataType:             entity.AttributeValueTypeString,
		Label:                "First name",
		Visibility:           entity.AttributeVisibilityPrivate,
		UserEditable:         true,
		DisplayOrder:         10,
		Validation:           json.RawMessage(`{}`),
	}
}
