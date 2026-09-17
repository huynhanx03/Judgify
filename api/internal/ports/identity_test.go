package ports

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestIdentityCampaignAudienceErrorsAreStable(t *testing.T) {
	if !errors.Is(ErrInvalidCampaignAudience, ErrInvalidCampaignAudience) {
		t.Fatal("invalid campaign audience error must be usable with errors.Is")
	}
}

func TestAuthorizedTraitOfferKeepsUUIDIdentifiers(t *testing.T) {
	offer := AuthorizedTraitOffer{OfferID: uuid.New(), CatalogChecksum: [sha256.Size]byte{1}}
	if offer.OfferID == uuid.Nil || offer.CatalogChecksum[0] != 1 {
		t.Fatalf("offer did not retain internal identifiers: %#v", offer)
	}
}
