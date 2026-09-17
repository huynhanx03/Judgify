package http

import (
	"fmt"
	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/identity/contract"
)

func toUpdateProfileAttributesRequest(v *updateProfileAttributesRequest) (*contract.UpdateProfileAttributesRequest, error) {
	if v == nil {
		return nil, nil
	}
	r := &contract.UpdateProfileAttributesRequest{Mutations: make([]contract.ProfileAttributeMutation, len(v.Mutations))}
	for i, m := range v.Mutations {
		definition, e := uuid.Parse(m.DefinitionID)
		if e != nil || definition == uuid.Nil {
			return nil, fmt.Errorf("invalid definition id")
		}
		revision, e := uuid.Parse(m.ExpectedDefinitionRevisionID)
		if e != nil || revision == uuid.Nil {
			return nil, fmt.Errorf("invalid definition revision id")
		}
		r.Mutations[i] = contract.ProfileAttributeMutation{DefinitionID: definition, ExpectedDefinitionRevisionID: revision, ExpectedValueVersion: m.ExpectedValueVersion, Value: m.Value, Unset: m.Unset}
	}
	return r, nil
}
func toProfileAttrsResponse(v *contract.ProfileAttrs) *profileAttrsResponse {
	if v == nil {
		return nil
	}
	return &profileAttrsResponse{ID: v.ID.String(), Username: v.Username, FirstName: v.FirstName, LastName: v.LastName, Gender: v.Gender, Birthday: v.Birthday, JoinedAt: v.JoinedAt}
}
func toProfileAttributesResponse(v *contract.ProfileAttributesResponse) *profileAttributesResponse {
	if v == nil {
		return nil
	}
	r := &profileAttributesResponse{Profile: *toProfileAttrsResponse(&v.Profile), Attributes: make([]profileAttributeValueResponse, len(v.Attributes))}
	for i, x := range v.Attributes {
		r.Attributes[i] = profileAttributeValueResponse{DefinitionID: x.DefinitionID.String(), DefinitionRevisionID: x.DefinitionRevisionID.String(), Key: x.Key, Kind: x.Kind, RequiredOnOnboarding: x.RequiredOnOnboarding, DataType: x.DataType, Label: x.Label, Description: x.Description, Visibility: x.Visibility, UserEditable: x.UserEditable, DisplayOrder: x.DisplayOrder, Validation: x.Validation, HasValue: x.HasValue, Value: x.Value, ValueVersion: x.ValueVersion}
	}
	return r
}
func toUserResponse(v *contract.UserResponse) *userResponse {
	if v == nil {
		return nil
	}
	return &userResponse{ID: v.ID.String(), Username: v.Username, Status: v.Status, Version: v.Version, SuspendedAt: v.SuspendedAt, Roles: append([]string(nil), v.Roles...), AuthorizationRevision: v.AuthorizationRevision, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toUserPage(v *d.Paginated[*contract.UserResponse]) *d.Paginated[*userResponse] {
	if v == nil {
		return nil
	}
	r := &d.Paginated[*userResponse]{Pagination: v.Pagination}
	if v.Records != nil {
		values := make([]*userResponse, len(*v.Records))
		for i, x := range *v.Records {
			values[i] = toUserResponse(x)
		}
		r.Records = &values
	}
	return r
}
