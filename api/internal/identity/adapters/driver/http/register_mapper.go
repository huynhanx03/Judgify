package http

import (
	"github.com/google/uuid"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func toRegisterRequest(value *registerRequest) *dto.RegisterRequest {
	if value == nil {
		return nil
	}
	profileValues := make([]dto.ProfileValueInput, len(value.ProfileValues))
	for index, item := range value.ProfileValues {
		profileValues[index] = dto.ProfileValueInput{DefinitionID: item.DefinitionID, DefinitionRevisionID: item.DefinitionRevisionID, Value: append([]byte(nil), item.Value...)}
	}
	return &dto.RegisterRequest{Username: value.Username, Password: value.Password, RecoveryEmail: value.RecoveryEmail,
		TraitOfferSelectionRequest: dto.TraitOfferSelectionRequest{OfferID: value.OfferID, OfferSequence: value.OfferSequence, CatalogRevisionID: value.CatalogRevisionID, CatalogChecksum: value.CatalogChecksum, OfferChecksum: value.OfferChecksum, RootBoneID: value.RootBoneID, TalentIDs: append([]uuid.UUID(nil), value.TalentIDs...)},
		ProfileSchemaEvidence:      dto.ProfileSchemaEvidence{ProfileSchemaRevision: value.ProfileSchemaRevision, ProfileSchemaChecksum: value.ProfileSchemaChecksum, ProfileValues: profileValues}, PreSessionID: value.PreSessionID, IdempotencyKey: value.IdempotencyKey, UserAgent: value.UserAgent, IPHash: append([]byte(nil), value.IPHash...)}
}
