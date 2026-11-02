package usecase

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

func TestOAuthFinalizeFingerprintCanonicallyBindsSelectedOffer(t *testing.T) {
	ticket := &entity.OAuthCompletionTicket{
		ID: uuid.New(), FlowID: uuid.New(), Provider: "google",
		Purpose: entity.OAuthPurposeLogin,
		Binding: entity.OAuthBinding{PreSessionID: uuid.New()},
	}
	first := oauthOnboardingSelection()
	second := first
	second.TalentIDs = []uuid.UUID{
		first.TalentIDs[2], first.TalentIDs[0], first.TalentIDs[1],
	}

	firstFingerprint, err := oauthFinalizeFingerprint(ticket, oauthFinalizeRequest(ticket.Binding.PreSessionID, &first))
	require.NoError(t, err)
	secondFingerprint, err := oauthFinalizeFingerprint(ticket, oauthFinalizeRequest(ticket.Binding.PreSessionID, &second))
	require.NoError(t, err)
	require.Equal(t, firstFingerprint, secondFingerprint)

	second.TalentIDs[0] = uuid.New()
	changedFingerprint, err := oauthFinalizeFingerprint(ticket, oauthFinalizeRequest(ticket.Binding.PreSessionID, &second))
	require.NoError(t, err)
	require.False(t, bytes.Equal(firstFingerprint, changedFingerprint))
}

func TestOAuthFinalizeFingerprintDistinguishesLoginFromOnboarding(t *testing.T) {
	ticket := &entity.OAuthCompletionTicket{
		ID: uuid.New(), FlowID: uuid.New(), Provider: "github",
		Purpose: entity.OAuthPurposeLogin,
		Binding: entity.OAuthBinding{PreSessionID: uuid.New()},
	}
	loginFingerprint, err := oauthFinalizeFingerprint(ticket, oauthFinalizeRequest(ticket.Binding.PreSessionID, nil))
	require.NoError(t, err)
	onboarding := oauthOnboardingSelection()
	onboardingFingerprint, err := oauthFinalizeFingerprint(ticket, oauthFinalizeRequest(ticket.Binding.PreSessionID, &onboarding))
	require.NoError(t, err)
	require.False(t, bytes.Equal(loginFingerprint, onboardingFingerprint))
}

func TestOAuthFinalizeFingerprintRejectsChangedPreSessionAndDuplicateTalents(t *testing.T) {
	ticket := &entity.OAuthCompletionTicket{
		ID: uuid.New(), FlowID: uuid.New(), Provider: "google",
		Purpose: entity.OAuthPurposeLogin,
		Binding: entity.OAuthBinding{PreSessionID: uuid.New()},
	}
	_, err := oauthFinalizeFingerprint(ticket, oauthFinalizeRequest(uuid.New(), nil))
	require.Error(t, err)

	onboarding := oauthOnboardingSelection()
	onboarding.TalentIDs[2] = onboarding.TalentIDs[0]
	_, err = oauthFinalizeFingerprint(
		ticket,
		oauthFinalizeRequest(ticket.Binding.PreSessionID, &onboarding),
	)
	require.Error(t, err)
}

func oauthFinalizeRequest(
	preSessionID uuid.UUID,
	onboarding *dto.OAuthOnboardingRequest,
) *dto.OAuthFinalizeLoginRequest {
	return &dto.OAuthFinalizeLoginRequest{
		Onboarding:   onboarding,
		PreSessionID: preSessionID,
		UserAgent:    "oauth-onboarding-test",
		IPHash:       bytes.Repeat([]byte{0x42}, 32),
	}
}

func oauthOnboardingSelection() dto.OAuthOnboardingRequest {
	return dto.OAuthOnboardingRequest{
		TraitOfferSelectionRequest: dto.TraitOfferSelectionRequest{
			OfferID: uuid.New(), OfferSequence: 2,
			CatalogRevisionID: uuid.New(),
			CatalogChecksum:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			OfferChecksum:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			RootBoneID:        uuid.New(),
			TalentIDs:         []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		},
		ProfileSchemaEvidence: dto.ProfileSchemaEvidence{
			ProfileSchemaRevision: 1,
			ProfileSchemaChecksum: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			ProfileValues: []dto.ProfileValueInput{{
				DefinitionID:         uuid.New(),
				DefinitionRevisionID: uuid.New(),
				Value:                []byte(`"Alice"`),
			}},
		},
	}
}
