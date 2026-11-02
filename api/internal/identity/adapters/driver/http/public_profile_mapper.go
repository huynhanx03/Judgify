package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toPublicProfileResponse(value *dto.PublicProfileResponse) *publicProfileResponse {
	if value == nil {
		return nil
	}
	attributes := make([]publicProfileAttributeResponse, len(value.Attributes))
	for i, item := range value.Attributes {
		attributes[i] = publicProfileAttributeResponse{Key: item.Key, DataType: item.DataType, Label: item.Label, Description: item.Description, Value: item.Value}
	}
	return &publicProfileResponse{Username: value.Username, JoinedAt: value.JoinedAt, Attributes: attributes}
}
func toProfilePrivacyResponse(value *dto.ProfilePrivacyResponse) *profilePrivacyResponse {
	if value == nil {
		return nil
	}
	return &profilePrivacyResponse{Visibility: value.Visibility, Version: value.Version, UpdatedAt: value.UpdatedAt}
}
func toProfilePrivacyMutationReceipt(value *dto.ProfilePrivacyMutationReceipt) *profilePrivacyMutationReceipt {
	if value == nil {
		return nil
	}
	return &profilePrivacyMutationReceipt{CommandID: value.CommandID.String(), EventID: value.EventID.String(), State: *toProfilePrivacyResponse(&value.State), CommittedAt: value.CommittedAt}
}
