package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toProfileSchemaResponse(value *dto.ProfileSchemaResponse) *profileSchemaResponse {
	if value == nil {
		return nil
	}
	result := &profileSchemaResponse{Revision: value.Revision, Checksum: value.Checksum, ServerTime: value.ServerTime, Fields: make([]profileSchemaFieldResponse, len(value.Fields))}
	for index, field := range value.Fields {
		result.Fields[index] = profileSchemaFieldResponse{
			DefinitionID: field.DefinitionID.String(), DefinitionRevisionID: field.DefinitionRevisionID.String(),
			Key: field.Key, DataType: field.DataType, Label: field.Label, Description: field.Description,
			Visibility: field.Visibility, UserEditable: field.UserEditable, DisplayOrder: field.DisplayOrder,
			Validation: append([]byte(nil), field.Validation...),
		}
	}
	return result
}
