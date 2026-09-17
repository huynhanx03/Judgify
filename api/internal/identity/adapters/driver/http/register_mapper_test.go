package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToRegisterRequestCopiesTransportAndOnboardingEvidence(t *testing.T) {
	t.Parallel()
	value := &registerRequest{
		Username: "judgify", Password: "correct-horse", RecoveryEmail: "user@example.com",
		OfferID: uuid.New(), OfferSequence: 1, CatalogRevisionID: uuid.New(), CatalogChecksum: "catalog", OfferChecksum: "offer", RootBoneID: uuid.New(), TalentIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		ProfileSchemaRevision: 3, ProfileSchemaChecksum: "schema", ProfileValues: []registerProfileValueRequest{{DefinitionID: uuid.New(), DefinitionRevisionID: uuid.New(), Value: []byte(`"value"`)}},
		IPHash: []byte{1, 2, 3},
	}

	result := toRegisterRequest(value)
	value.IPHash[0] = 9
	value.ProfileValues[0].Value[0] = 'X'
	require.Equal(t, []byte{1, 2, 3}, result.IPHash)
	require.EqualValues(t, []byte(`"value"`), result.ProfileValues[0].Value)
	require.Equal(t, value.OfferID, result.OfferID)
	require.Equal(t, value.ProfileSchemaRevision, result.ProfileSchemaRevision)
}
