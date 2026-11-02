package onboarding

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	cultivationentity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	cultivationports "github.com/huynhanx03/judgify/internal/ports"
	identityports "github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	cultivationmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
	problemmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

type registrationOfferRepositoryStub struct {
	cultivationports.TraitOfferRepository
	offer          cultivationentity.TraitOffer
	lockErr        error
	consumeErr     error
	consumedUserID uuid.UUID
	consumeCalls   int
}

func (stub *registrationOfferRepositoryStub) LockOffer(
	context.Context,
	uuid.UUID,
) (cultivationentity.TraitOffer, error) {
	return stub.offer, stub.lockErr
}

func (stub *registrationOfferRepositoryStub) ConsumeOffer(
	_ context.Context,
	_ uuid.UUID,
	userID uuid.UUID,
	_ time.Time,
) error {
	stub.consumeCalls++
	stub.consumedUserID = userID
	return stub.consumeErr
}

type registrationTestDependencies struct {
	rewardProfiles *registrationRewardProfileStub
	userStats      *cultivationmocks.MockUserStatsRepository
	userElementExp *cultivationmocks.MockUserElementExpRepository
	elements       *cultivationmocks.MockElementRepository
	userDifficulty *cultivationmocks.MockUserDifficultyStatsRepository
	difficulties   *problemmocks.MockDifficultyRepository
}

type registrationRewardProfileStub struct {
	calls         int
	userID        uuid.UUID
	offerID       uuid.UUID
	selection     cultivationentity.RewardProfileSelection
	contextMarked bool
	err           error
}

func (stub *registrationRewardProfileStub) ProvisionOnboarding(
	ctx context.Context,
	userID uuid.UUID,
	offer cultivationentity.TraitOffer,
	selection cultivationentity.RewardProfileSelection,
) (cultivationentity.UserRewardProfileRevision, error) {
	stub.calls++
	stub.userID = userID
	stub.offerID = offer.ID
	stub.selection = selection
	stub.contextMarked, _ = ctx.Value(registrationTransactionMarker{}).(bool)
	return cultivationentity.UserRewardProfileRevision{}, stub.err
}

func TestRegistrationLifecycleAuthorizesAndRechecksDurableOffer(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	offer := validRegistrationOffer(t, now)
	repository := &registrationOfferRepositoryStub{offer: offer}
	lifecycle, _ := newRegistrationLifecycleTestFixture(t, now, offer, repository)
	authorized := authorizeRegistrationOffer(t, lifecycle, offer)

	require.NoError(t, lifecycle.Consume(context.Background(), authorized))

	repository.offer.Status = cultivationentity.TraitOfferSuperseded
	err := lifecycle.Consume(context.Background(), authorized)
	requireApplicationErrorCode(t, err, apperr.CodeConflict)
}

func TestRegistrationLifecycleRejectsOfferTampering(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	offer := validRegistrationOffer(t, now)
	repository := &registrationOfferRepositoryStub{offer: offer}
	lifecycle, _ := newRegistrationLifecycleTestFixture(t, now, offer, repository)

	_, err := lifecycle.Authorize(
		offer.PreSessionID,
		uuid.Nil,
		offer.Sequence,
		offer.CatalogRevisionID,
		hex.EncodeToString(offer.CatalogChecksum[:]),
		hex.EncodeToString(offer.OfferChecksum[:]),
		offer.Root.ID,
		[]uuid.UUID{offer.Talents[0].ID, offer.Talents[1].ID, offer.Talents[2].ID},
	)
	requireApplicationErrorCode(t, err, apperr.CodeBadRequest)

	_, err = lifecycle.Authorize(
		offer.PreSessionID,
		offer.ID,
		offer.Sequence,
		offer.CatalogRevisionID,
		hex.EncodeToString(offer.CatalogChecksum[:]),
		hex.EncodeToString(offer.OfferChecksum[:]),
		offer.Root.ID,
		[]uuid.UUID{offer.Talents[0].ID, offer.Talents[0].ID, offer.Talents[1].ID},
	)
	requireApplicationErrorCode(t, err, apperr.CodeBadRequest)

	authorized := authorizeRegistrationOffer(t, lifecycle, offer)
	authorized.OfferChecksum[0] ^= 0xff
	err = lifecycle.Consume(context.Background(), authorized)
	requireApplicationErrorCode(t, err, apperr.CodeBadRequest)
}

type registrationTransactionMarker struct{}

func TestRegistrationLifecycleProvisionConsumesOfferAndInitializesCultivation(t *testing.T) {
	now := time.Date(2026, 7, 22, 4, 0, 0, 0, time.UTC)
	offer := validRegistrationOffer(t, now)
	repository := &registrationOfferRepositoryStub{offer: offer}
	lifecycle, dependencies := newRegistrationLifecycleTestFixture(t, now, offer, repository)
	authorized := authorizeRegistrationOffer(t, lifecycle, offer)
	userID := uuid.New()
	elementIDs := []uuid.UUID{uuid.New(), uuid.New()}
	difficultyIDs := []uuid.UUID{uuid.New(), uuid.New()}
	ctx := context.WithValue(context.Background(), registrationTransactionMarker{}, true)
	marked := func(ctx context.Context) bool {
		value, _ := ctx.Value(registrationTransactionMarker{}).(bool)
		return value
	}

	gomock.InOrder(
		dependencies.userStats.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(callCtx context.Context, stats *cultivationentity.UserStats) error {
				require.True(t, marked(callCtx))
				require.Equal(t, userID, stats.UserID)
				require.Equal(t, cultivationentity.InitialRating, stats.Rating)
				return nil
			}),
		dependencies.elements.EXPECT().FindAllBounded(gomock.Any(), maximumRegistrationElements+1).
			Return([]*cultivationentity.Element{{ID: elementIDs[0]}, {ID: elementIDs[1]}}, nil),
		dependencies.userElementExp.EXPECT().CreateBulk(gomock.Any(), gomock.Any()).
			DoAndReturn(func(callCtx context.Context, values []*cultivationentity.UserElementExp) error {
				require.True(t, marked(callCtx))
				require.Len(t, values, len(elementIDs))
				return nil
			}),
		dependencies.difficulties.EXPECT().FindAllBounded(gomock.Any(), maximumRegistrationDifficulties+1).
			Return([]*problementity.Difficulty{{ID: difficultyIDs[0]}, {ID: difficultyIDs[1]}}, nil),
		dependencies.userDifficulty.EXPECT().CreateBulk(gomock.Any(), userID, difficultyIDs).
			Return(nil),
	)

	require.NoError(t, lifecycle.Provision(ctx, userID, authorized))
	require.Equal(t, 1, repository.consumeCalls)
	require.Equal(t, userID, repository.consumedUserID)
	require.Equal(t, 1, dependencies.rewardProfiles.calls)
	require.Equal(t, userID, dependencies.rewardProfiles.userID)
	require.Equal(t, offer.ID, dependencies.rewardProfiles.offerID)
	require.True(t, dependencies.rewardProfiles.contextMarked)
	require.Equal(t, offer.Root.ID, dependencies.rewardProfiles.selection.RootTraitID)
	require.Equal(t, authorized.TalentIDs, dependencies.rewardProfiles.selection.TalentTraitIDs)
}

func newRegistrationLifecycleTestFixture(
	t *testing.T,
	now time.Time,
	offer cultivationentity.TraitOffer,
	repository *registrationOfferRepositoryStub,
) (*RegistrationLifecycle, registrationTestDependencies) {
	t.Helper()
	controller := gomock.NewController(t)
	dependencies := registrationTestDependencies{
		rewardProfiles: &registrationRewardProfileStub{},
		userStats:      cultivationmocks.NewMockUserStatsRepository(controller),
		userElementExp: cultivationmocks.NewMockUserElementExpRepository(controller),
		elements:       cultivationmocks.NewMockElementRepository(controller),
		userDifficulty: cultivationmocks.NewMockUserDifficultyStatsRepository(controller),
		difficulties:   problemmocks.NewMockDifficultyRepository(controller),
	}
	lifecycle, err := NewRegistrationLifecycle(RegistrationDependencies{
		OfferRepository: repository,
		RewardProfiles:  dependencies.rewardProfiles, UserStats: dependencies.userStats,
		UserElementExp: dependencies.userElementExp, Elements: dependencies.elements,
		UserDifficulty: dependencies.userDifficulty, Difficulties: dependencies.difficulties,
		Clock: func() time.Time { return now },
	})
	require.NoError(t, err)
	return lifecycle, dependencies
}

func validRegistrationOffer(t *testing.T, now time.Time) cultivationentity.TraitOffer {
	t.Helper()
	trait := func(kind string, index int) cultivationentity.CatalogTrait {
		traitID := uuid.New()
		bonus := int64(0)
		effect, err := cultivationentity.NewTraitEffectRevision(
			uuid.New(), traitID, 1,
			cultivationentity.TraitEffectSpec{
				Version:   cultivationentity.TraitEffectSchemaVersion,
				Kind:      cultivationentity.TraitEffectExperienceBonus,
				Scope:     cultivationentity.TraitEffectScopeAll,
				FlatBonus: &bonus,
			},
			nil,
			now,
		)
		require.NoError(t, err)
		return cultivationentity.CatalogTrait{
			ID: traitID, Revision: 1, Code: fmt.Sprintf("%s_%d", kind, index), Type: kind,
			Name: kind, RarityID: uuid.New(), RarityCode: "common",
			RarityName: "Common", DisplayOrder: index, Weight: index + 1,
			EffectRevision: effect,
		}
	}
	talents := make([]cultivationentity.CatalogTrait, 6)
	for index := range talents {
		talents[index] = trait(cultivationentity.TraitTypeTalent, index)
	}
	offer := cultivationentity.TraitOffer{
		ID: uuid.New(), PreSessionID: uuid.New(), IdempotencyKey: uuid.New(),
		Sequence: 1, CatalogRevisionID: uuid.New(), CatalogChecksum: [32]byte{1},
		AlgorithmVersion: cultivationentity.OfferAlgorithmVersion,
		Root:             trait(cultivationentity.TraitTypeRootBone, 0), Talents: talents,
		Status: cultivationentity.TraitOfferIssued, Remaining: 2,
		IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	checksum, err := cultivationentity.TraitOfferChecksum(offer)
	require.NoError(t, err)
	offer.OfferChecksum = checksum
	require.True(t, offer.Valid())
	return offer
}

func authorizeRegistrationOffer(
	t *testing.T,
	lifecycle *RegistrationLifecycle,
	offer cultivationentity.TraitOffer,
) identityports.AuthorizedTraitOffer {
	t.Helper()
	authorized, err := lifecycle.Authorize(
		offer.PreSessionID, offer.ID, offer.Sequence, offer.CatalogRevisionID,
		hex.EncodeToString(offer.CatalogChecksum[:]),
		hex.EncodeToString(offer.OfferChecksum[:]), offer.Root.ID,
		[]uuid.UUID{offer.Talents[0].ID, offer.Talents[1].ID, offer.Talents[2].ID},
	)
	require.NoError(t, err)
	return authorized
}

func requireApplicationErrorCode(t *testing.T, err error, code int) {
	t.Helper()
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, code, applicationError.Code)
}
