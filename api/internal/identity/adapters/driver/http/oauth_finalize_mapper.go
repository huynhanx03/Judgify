package http

import (
	"github.com/google/uuid"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func toOAuthFinalizeLoginRequest(value *oauthFinalizeLoginRequest) *dto.OAuthFinalizeLoginRequest {
	if value == nil {
		return nil
	}
	result := &dto.OAuthFinalizeLoginRequest{
		CompletionTicket: value.CompletionTicket,
		PreSessionID:     value.PreSessionID,
		IdempotencyKey:   value.IdempotencyKey,
		UserAgent:        value.UserAgent,
		IPHash:           append([]byte(nil), value.IPHash...),
	}
	if value.Onboarding != nil {
		profileValues := make([]dto.ProfileValueInput, len(value.Onboarding.ProfileValues))
		for index, profileValue := range value.Onboarding.ProfileValues {
			profileValues[index] = dto.ProfileValueInput{DefinitionID: profileValue.DefinitionID, DefinitionRevisionID: profileValue.DefinitionRevisionID, Value: append([]byte(nil), profileValue.Value...)}
		}
		result.Onboarding = &dto.OAuthOnboardingRequest{
			TraitOfferSelectionRequest: dto.TraitOfferSelectionRequest{
				OfferID: value.Onboarding.OfferID, OfferSequence: value.Onboarding.OfferSequence,
				CatalogRevisionID: value.Onboarding.CatalogRevisionID, CatalogChecksum: value.Onboarding.CatalogChecksum,
				OfferChecksum: value.Onboarding.OfferChecksum, RootBoneID: value.Onboarding.RootBoneID,
				TalentIDs: append([]uuid.UUID(nil), value.Onboarding.TalentIDs...),
			},
			ProfileSchemaEvidence: dto.ProfileSchemaEvidence{ProfileSchemaRevision: value.Onboarding.ProfileSchemaRevision, ProfileSchemaChecksum: value.Onboarding.ProfileSchemaChecksum, ProfileValues: profileValues},
		}
	}
	return result
}
