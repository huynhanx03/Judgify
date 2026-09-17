package http

import (
	"fmt"
	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/identity/contract"
)

func parseAttributeDefinitionID(v string) (uuid.UUID, error) {
	id, e := uuid.Parse(v)
	if e != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid attribute definition id")
	}
	return id, nil
}
func parseOptionalAttributeDefinitionID(v string) (uuid.UUID, error) {
	if v == "" {
		return uuid.Nil, nil
	}
	return parseAttributeDefinitionID(v)
}
func toCreateAttributeDefinitionRequest(v *createAttributeDefinitionRequest) (*contract.CreateAttributeDefinitionRequest, error) {
	if v == nil {
		return nil, nil
	}
	id, e := parseAttributeDefinitionID(v.CommandID)
	if e != nil {
		return nil, e
	}
	return &contract.CreateAttributeDefinitionRequest{CommandID: id, Reason: v.Reason, Key: v.Key, DataType: v.DataType, Label: v.Label, Description: v.Description, Visibility: v.Visibility, UserEditable: v.UserEditable, DisplayOrder: v.DisplayOrder, RequiredOnOnboarding: v.RequiredOnOnboarding, Validation: v.Validation}, nil
}
func toUpdateAttributeDefinitionRequest(v *updateAttributeDefinitionRequest) (*contract.UpdateAttributeDefinitionRequest, error) {
	if v == nil {
		return nil, nil
	}
	id, e := parseAttributeDefinitionID(v.ID)
	if e != nil {
		return nil, e
	}
	command, e := parseAttributeDefinitionID(v.CommandID)
	if e != nil {
		return nil, e
	}
	return &contract.UpdateAttributeDefinitionRequest{ID: id, CommandID: command, ExpectedVersion: v.ExpectedVersion, Reason: v.Reason, DataType: v.DataType, Label: v.Label, Description: v.Description, Visibility: v.Visibility, UserEditable: v.UserEditable, DisplayOrder: v.DisplayOrder, RequiredOnOnboarding: v.RequiredOnOnboarding, Validation: v.Validation}, nil
}
func toGetAttributeDefinitionRequest(v *getAttributeDefinitionRequest) (*contract.GetAttributeDefinitionRequest, error) {
	if v == nil {
		return nil, nil
	}
	id, e := parseAttributeDefinitionID(v.ID)
	if e != nil {
		return nil, e
	}
	return &contract.GetAttributeDefinitionRequest{ID: id}, nil
}
func toListAttributeDefinitionRevisionsRequest(v *listAttributeDefinitionRevisionsRequest) (*contract.ListAttributeDefinitionRevisionsRequest, error) {
	if v == nil {
		return nil, nil
	}
	id, e := parseAttributeDefinitionID(v.ID)
	if e != nil {
		return nil, e
	}
	return &contract.ListAttributeDefinitionRevisionsRequest{ID: id, Cursor: v.Cursor, Limit: v.Limit}, nil
}
func toAttributeLifecycleCommandRequest(v *attributeLifecycleCommandRequest) (*contract.AttributeLifecycleCommandRequest, error) {
	if v == nil {
		return nil, nil
	}
	id, e := parseAttributeDefinitionID(v.ID)
	if e != nil {
		return nil, e
	}
	command, e := parseAttributeDefinitionID(v.CommandID)
	if e != nil {
		return nil, e
	}
	candidate, e := parseOptionalAttributeDefinitionID(v.CandidateRevisionID)
	if e != nil {
		return nil, e
	}
	return &contract.AttributeLifecycleCommandRequest{ID: id, CommandID: command, ExpectedVersion: v.ExpectedVersion, CandidateRevisionID: candidate, Reason: v.Reason, ReviewedPreviewHash: v.ReviewedPreviewHash, ConfirmationToken: v.ConfirmationToken}, nil
}
func toAttributeDefinitionRevisionResponse(v *contract.AttributeDefinitionRevisionResponse) *attributeDefinitionRevisionResponse {
	if v == nil {
		return nil
	}
	return &attributeDefinitionRevisionResponse{ID: v.ID.String(), DefinitionID: v.DefinitionID.String(), Revision: v.Revision, DataType: v.DataType, Label: v.Label, Description: v.Description, Visibility: v.Visibility, UserEditable: v.UserEditable, DisplayOrder: v.DisplayOrder, Validation: v.Validation, CreatedAt: v.CreatedAt}
}
func idString(v *uuid.UUID) *string {
	if v == nil {
		return nil
	}
	result := v.String()
	return &result
}
func toAttributeDefinitionResponse(v *contract.AttributeDefinitionResponse) *attributeDefinitionResponse {
	if v == nil {
		return nil
	}
	return &attributeDefinitionResponse{ID: v.ID.String(), Key: v.Key, Kind: v.Kind, RequiredOnOnboarding: v.RequiredOnOnboarding, Status: v.Status, Version: v.Version, DraftRevisionID: idString(v.DraftRevisionID), ActiveRevisionID: idString(v.ActiveRevisionID), DraftRevision: toAttributeDefinitionRevisionResponse(v.DraftRevision), ActiveRevision: toAttributeDefinitionRevisionResponse(v.ActiveRevision)}
}
func toAttributeDefinitionPage(v *d.Paginated[*contract.AttributeDefinitionResponse]) *d.Paginated[*attributeDefinitionResponse] {
	if v == nil {
		return nil
	}
	r := &d.Paginated[*attributeDefinitionResponse]{Pagination: v.Pagination}
	if v.Records != nil {
		values := make([]*attributeDefinitionResponse, len(*v.Records))
		for i, x := range *v.Records {
			values[i] = toAttributeDefinitionResponse(x)
		}
		r.Records = &values
	}
	return r
}
func toAttributeDefinitionRevisionPageResponse(v *contract.AttributeDefinitionRevisionPage) *attributeDefinitionRevisionPageResponse {
	if v == nil {
		return nil
	}
	r := &attributeDefinitionRevisionPageResponse{NextCursor: v.NextCursor, Records: make([]*attributeDefinitionRevisionResponse, len(v.Records))}
	for i, x := range v.Records {
		r.Records[i] = toAttributeDefinitionRevisionResponse(x)
	}
	return r
}
func toAttributeLifecyclePreviewResponse(v *contract.AttributeLifecyclePreviewResponse) *attributeLifecyclePreviewResponse {
	if v == nil {
		return nil
	}
	r := &attributeLifecyclePreviewResponse{ReviewHash: v.ReviewHash, ConfirmationToken: v.ConfirmationToken, ExpiresAt: v.ExpiresAt}
	r.Preview.Action = v.Preview.Action
	r.Preview.DefinitionID = v.Preview.DefinitionID.String()
	r.Preview.ExpectedVersion = v.Preview.ExpectedVersion
	r.Preview.ResultingVersion = v.Preview.ResultingVersion
	r.Preview.CurrentRevision = toAttributeDefinitionRevisionResponse(v.Preview.CurrentRevision)
	r.Preview.CandidateRevision = toAttributeDefinitionRevisionResponse(v.Preview.CandidateRevision)
	r.Preview.SourceValueCount = v.Preview.SourceValueCount
	r.Preview.TargetValueCount = v.Preview.TargetValueCount
	r.Preview.RequiresMigration = v.Preview.RequiresMigration
	return r
}
func toAttributeLifecycleReceiptResponse(v *contract.AttributeLifecycleReceipt) *attributeLifecycleReceiptResponse {
	if v == nil {
		return nil
	}
	r := &attributeLifecycleReceiptResponse{Status: v.Status, Action: v.Action, CommandID: v.CommandID.String(), DefinitionID: v.DefinitionID.String(), Version: v.Version, Definition: toAttributeDefinitionResponse(v.Definition), ActiveRevision: toAttributeDefinitionRevisionResponse(v.ActiveRevision), Operation: v.Operation, RecordedAt: v.RecordedAt, IdempotentReplay: v.IdempotentReplay}
	if v.EventID != nil {
		x := v.EventID.String()
		r.EventID = &x
	}
	return r
}
