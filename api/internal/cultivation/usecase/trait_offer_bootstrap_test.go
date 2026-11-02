package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type passTraitOfferUnitOfWork struct{}

func (passTraitOfferUnitOfWork) Within(
	ctx context.Context,
	callback func(context.Context) error,
) error {
	return callback(ctx)
}

type bootstrapTraitOfferRepository struct {
	ports.TraitOfferRepository
	live    []entity.CatalogTrait
	latest  entity.CatalogRevision
	found   bool
	stored  int
	locked  int
	loadErr error
}

func (repository *bootstrapTraitOfferRepository) LockCatalog(context.Context) error {
	repository.locked++
	return nil
}

func (repository *bootstrapTraitOfferRepository) LoadLatestCatalog(
	context.Context,
) (entity.CatalogRevision, bool, error) {
	return repository.latest, repository.found, nil
}

func (repository *bootstrapTraitOfferRepository) LoadLiveCatalog(
	context.Context,
) ([]entity.CatalogTrait, error) {
	if repository.loadErr != nil {
		return nil, repository.loadErr
	}
	return repository.live, nil
}

func (repository *bootstrapTraitOfferRepository) StoreCatalog(
	context.Context,
	entity.CatalogRevision,
) error {
	repository.stored++
	return nil
}

func newBootstrapTraitOfferService(
	t *testing.T,
	repository ports.TraitOfferRepository,
) ports.TraitOfferService {
	t.Helper()
	service, err := NewTraitOfferService(
		repository,
		passTraitOfferUnitOfWork{},
		ports.TraitOfferServiceOptions{},
	)
	require.NoError(t, err)
	return service
}

// A migrated and bootstrapped database has no cultivation traits yet. If startup
// demanded a launchable catalog the installation could never boot, because the
// only way to author traits is the API that would be refusing to start.
func TestRefreshCatalogAcceptsFreshlyBootstrappedDatabaseWithoutTraits(t *testing.T) {
	t.Parallel()

	repository := &bootstrapTraitOfferRepository{}
	service := newBootstrapTraitOfferService(t, repository)

	require.NoError(t, service.RefreshCatalog(context.Background()))
	require.Positive(t, repository.locked)
	require.Zero(t, repository.stored)
}

func TestRefreshCatalogStillRejectsInvalidCatalogOnceOneHasLaunched(t *testing.T) {
	t.Parallel()

	repository := &bootstrapTraitOfferRepository{
		found: true,
		latest: entity.CatalogRevision{
			ID: uuid.New(), Revision: 3, AlgorithmVersion: entity.OfferAlgorithmVersion,
			MaximumOffers: 6, OfferTTL: 10 * time.Minute, CreatedAt: time.Now().UTC(),
		},
		live: []entity.CatalogTrait{{
			ID: uuid.New(), Revision: 1, Weight: 1, Name: "Broken", RarityID: uuid.New(),
		}},
	}
	service := newBootstrapTraitOfferService(t, repository)

	require.ErrorIs(t, service.RefreshCatalog(context.Background()), entity.ErrTraitOfferInvalid)
	require.Zero(t, repository.stored)
}

func TestInvalidatePoolKeepsTheSameFirstCatalogTolerance(t *testing.T) {
	t.Parallel()

	repository := &bootstrapTraitOfferRepository{}
	service := newBootstrapTraitOfferService(t, repository)

	require.NoError(t, service.InvalidatePool(context.Background()))
	require.Zero(t, repository.stored)
}
