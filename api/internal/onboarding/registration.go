package onboarding

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	cultivationports "github.com/huynhanx03/judgify/internal/ports"
	identityports "github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const (
	registrationTalentCount         = 3
	maximumRegistrationElements     = 256
	maximumRegistrationDifficulties = 1_024
)

var ErrInvalidRegistrationLifecycle = errors.New("onboarding: invalid registration lifecycle")

type RewardProfileInitializer interface {
	ProvisionOnboarding(
		context.Context,
		uuid.UUID,
		entity.TraitOffer,
		entity.RewardProfileSelection,
	) (entity.UserRewardProfileRevision, error)
}

type UserStatsInitializer interface {
	Create(context.Context, *entity.UserStats) error
}

type UserElementInitializer interface {
	CreateBulk(context.Context, []*entity.UserElementExp) error
}

type ElementCatalog interface {
	FindAllBounded(context.Context, int) ([]*entity.Element, error)
}

type UserDifficultyInitializer interface {
	CreateBulk(context.Context, uuid.UUID, []uuid.UUID) error
}

type DifficultyCatalog interface {
	FindAllBounded(context.Context, int) ([]*problementity.Difficulty, error)
}

type RegistrationDependencies struct {
	OfferRepository cultivationports.TraitOfferRepository
	RewardProfiles  RewardProfileInitializer
	UserStats       UserStatsInitializer
	UserElementExp  UserElementInitializer
	Elements        ElementCatalog
	UserDifficulty  UserDifficultyInitializer
	Difficulties    DifficultyCatalog
	Clock           func() time.Time
}

// RegistrationLifecycle is the application-level bridge between Identity,
// Cultivation, and Problem. Identity owns the transaction; every repository
// resolves its transaction-scoped Ent client from the supplied context.
type RegistrationLifecycle struct {
	dependencies RegistrationDependencies
}

func NewRegistrationLifecycle(
	dependencies RegistrationDependencies,
) (*RegistrationLifecycle, error) {
	if nilDependency(dependencies.OfferRepository) ||
		nilDependency(dependencies.RewardProfiles) ||
		nilDependency(dependencies.UserStats) || nilDependency(dependencies.UserElementExp) ||
		nilDependency(dependencies.Elements) || nilDependency(dependencies.UserDifficulty) ||
		nilDependency(dependencies.Difficulties) || dependencies.Clock == nil {
		return nil, ErrInvalidRegistrationLifecycle
	}
	return &RegistrationLifecycle{dependencies: dependencies}, nil
}

func (lifecycle *RegistrationLifecycle) Authorize(
	preSessionID uuid.UUID,
	offerID uuid.UUID,
	expectedSequence int64,
	catalogRevisionID uuid.UUID,
	catalogChecksum string,
	offerChecksum string,
	rootBoneID uuid.UUID,
	talentIDs []uuid.UUID,
) (identityports.AuthorizedTraitOffer, error) {
	if lifecycle == nil || preSessionID == uuid.Nil {
		return identityports.AuthorizedTraitOffer{}, registrationUnavailable()
	}
	if offerID == uuid.Nil || expectedSequence < 1 || catalogRevisionID == uuid.Nil {
		return identityports.AuthorizedTraitOffer{}, invalidRegistrationOffer(
			entity.ErrTraitOfferInvalid,
		)
	}
	catalogDigest, err := decodeRegistrationChecksum(catalogChecksum)
	if err != nil {
		return identityports.AuthorizedTraitOffer{}, invalidRegistrationOffer(err)
	}
	offerDigest, err := decodeRegistrationChecksum(offerChecksum)
	if err != nil {
		return identityports.AuthorizedTraitOffer{}, invalidRegistrationOffer(err)
	}
	authorized := identityports.AuthorizedTraitOffer{
		OfferID: offerID, PreSessionID: preSessionID,
		ExpectedSequence:  expectedSequence,
		CatalogRevisionID: catalogRevisionID,
		CatalogChecksum:   catalogDigest, OfferChecksum: offerDigest,
		RootBoneID: rootBoneID,
		TalentIDs:  append([]uuid.UUID(nil), talentIDs...),
	}
	if !validAuthorizedOffer(authorized) {
		return identityports.AuthorizedTraitOffer{}, invalidRegistrationOffer(
			entity.ErrTraitOfferInvalid,
		)
	}
	return authorized, nil
}

func (lifecycle *RegistrationLifecycle) Consume(
	ctx context.Context,
	authorized identityports.AuthorizedTraitOffer,
) error {
	if lifecycle == nil || ctx == nil || !validAuthorizedOffer(authorized) ||
		nilDependency(lifecycle.dependencies.OfferRepository) ||
		lifecycle.dependencies.Clock == nil {
		return registrationUnavailable()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	offer, err := lifecycle.dependencies.OfferRepository.LockOffer(ctx, authorized.OfferID)
	if err != nil {
		return mapOfferConsumptionError(ctx, err)
	}
	if err := validateAuthorizedOffer(
		offer, authorized, lifecycle.dependencies.Clock().UTC(),
	); err != nil {
		return mapOfferConsumptionError(ctx, err)
	}
	return nil
}

func (lifecycle *RegistrationLifecycle) Provision(
	ctx context.Context,
	userID uuid.UUID,
	authorized identityports.AuthorizedTraitOffer,
) error {
	if lifecycle == nil || ctx == nil || userID == uuid.Nil || !validAuthorizedOffer(authorized) {
		return registrationUnavailable()
	}
	now := lifecycle.dependencies.Clock().UTC()
	offer, err := lifecycle.dependencies.OfferRepository.LockOffer(ctx, authorized.OfferID)
	if err != nil {
		return mapOfferConsumptionError(ctx, err)
	}
	if err := validateAuthorizedOffer(offer, authorized, now); err != nil {
		return mapOfferConsumptionError(ctx, err)
	}
	if err := lifecycle.dependencies.OfferRepository.ConsumeOffer(
		ctx, authorized.OfferID, userID, now,
	); err != nil {
		return mapOfferConsumptionError(ctx, err)
	}
	if _, err := lifecycle.dependencies.RewardProfiles.ProvisionOnboarding(
		ctx,
		userID,
		offer,
		entity.RewardProfileSelection{
			RootTraitID:    authorized.RootBoneID,
			TalentTraitIDs: append([]uuid.UUID(nil), authorized.TalentIDs...),
		},
	); err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to provision reward profile", err)
	}
	if err := lifecycle.dependencies.UserStats.Create(ctx, &entity.UserStats{
		UserID: userID,
		Rating: entity.InitialRating,
	}); err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to init user stats", err)
	}

	elements, err := lifecycle.dependencies.Elements.FindAllBounded(
		ctx,
		maximumRegistrationElements+1,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to load elements", err)
	}
	if len(elements) == 0 || len(elements) > maximumRegistrationElements {
		return registrationUnavailable()
	}
	elementExps := make([]*entity.UserElementExp, len(elements))
	for index, element := range elements {
		if element == nil || element.ID == uuid.Nil {
			return registrationUnavailable()
		}
		elementExps[index] = &entity.UserElementExp{UserID: userID, ElementID: element.ID}
	}
	if err := lifecycle.dependencies.UserElementExp.CreateBulk(ctx, elementExps); err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to init element exp", err)
	}

	difficulties, err := lifecycle.dependencies.Difficulties.FindAllBounded(
		ctx,
		maximumRegistrationDifficulties+1,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to load difficulties", err)
	}
	if len(difficulties) == 0 || len(difficulties) > maximumRegistrationDifficulties {
		return registrationUnavailable()
	}
	difficultyIDs := make([]uuid.UUID, len(difficulties))
	for index, difficulty := range difficulties {
		if difficulty == nil || difficulty.ID == uuid.Nil {
			return registrationUnavailable()
		}
		difficultyIDs[index] = difficulty.ID
	}
	if err := lifecycle.dependencies.UserDifficulty.CreateBulk(ctx, userID, difficultyIDs); err != nil {
		return apperr.New(apperr.CodeInternalServer, "failed to init difficulty stats", err)
	}
	return nil
}

func validAuthorizedOffer(authorized identityports.AuthorizedTraitOffer) bool {
	if authorized.OfferID == uuid.Nil || authorized.PreSessionID == uuid.Nil ||
		authorized.ExpectedSequence < 1 || authorized.CatalogRevisionID == uuid.Nil ||
		authorized.CatalogChecksum == ([32]byte{}) ||
		authorized.OfferChecksum == ([32]byte{}) || authorized.RootBoneID == uuid.Nil ||
		len(authorized.TalentIDs) != registrationTalentCount {
		return false
	}
	seen := map[uuid.UUID]struct{}{authorized.RootBoneID: {}}
	for _, talentID := range authorized.TalentIDs {
		if talentID == uuid.Nil {
			return false
		}
		if _, duplicate := seen[talentID]; duplicate {
			return false
		}
		seen[talentID] = struct{}{}
	}
	return true
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func decodeRegistrationChecksum(value string) ([32]byte, error) {
	var checksum [32]byte
	if len(value) != hex.EncodedLen(len(checksum)) {
		return checksum, entity.ErrTraitOfferInvalid
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(checksum) {
		return checksum, entity.ErrTraitOfferInvalid
	}
	copy(checksum[:], decoded)
	if checksum == ([32]byte{}) {
		return [32]byte{}, entity.ErrTraitOfferInvalid
	}
	return checksum, nil
}

func validateAuthorizedOffer(
	offer entity.TraitOffer,
	authorized identityports.AuthorizedTraitOffer,
	now time.Time,
) error {
	if !offer.Valid() || !validAuthorizedOffer(authorized) {
		return entity.ErrTraitOfferUnavailable
	}
	switch offer.Status {
	case entity.TraitOfferExpired:
		return entity.ErrTraitOfferExpired
	case entity.TraitOfferSuperseded:
		return entity.ErrTraitOfferSuperseded
	case entity.TraitOfferConsumed:
		return entity.ErrTraitOfferConsumed
	case entity.TraitOfferIssued:
	default:
		return entity.ErrTraitOfferUnavailable
	}
	if !now.Before(offer.ExpiresAt) {
		return entity.ErrTraitOfferExpired
	}
	if offer.ID != authorized.OfferID ||
		offer.PreSessionID != authorized.PreSessionID ||
		offer.Sequence != authorized.ExpectedSequence ||
		offer.CatalogRevisionID != authorized.CatalogRevisionID ||
		offer.Root.ID != authorized.RootBoneID ||
		subtle.ConstantTimeCompare(offer.CatalogChecksum[:], authorized.CatalogChecksum[:]) != 1 ||
		subtle.ConstantTimeCompare(offer.OfferChecksum[:], authorized.OfferChecksum[:]) != 1 {
		return entity.ErrTraitOfferInvalid
	}
	calculated, err := entity.TraitOfferChecksum(offer)
	if err != nil || subtle.ConstantTimeCompare(calculated[:], authorized.OfferChecksum[:]) != 1 {
		return entity.ErrTraitOfferInvalid
	}
	available := make(map[uuid.UUID]struct{}, len(offer.Talents))
	for _, talent := range offer.Talents {
		available[talent.ID] = struct{}{}
	}
	for _, selected := range authorized.TalentIDs {
		if _, ok := available[selected]; !ok {
			return entity.ErrTraitOfferInvalid
		}
	}
	return nil
}

func mapOfferConsumptionError(ctx context.Context, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case errors.Is(err, entity.ErrTraitOfferExpired),
		errors.Is(err, entity.ErrTraitOfferSuperseded),
		errors.Is(err, entity.ErrTraitOfferConsumed),
		errors.Is(err, entity.ErrTraitOfferConflict):
		return apperr.New(
			apperr.CodeConflict,
			"onboarding trait offer is no longer available",
			err,
		)
	case errors.Is(err, entity.ErrTraitOfferInvalid),
		errors.Is(err, entity.ErrTraitOfferUnavailable):
		return invalidRegistrationOffer(err)
	default:
		return apperr.New(
			apperr.CodeInternalServer,
			"registration service unavailable",
			err,
		)
	}
}

func invalidRegistrationOffer(cause error) error {
	return apperr.New(apperr.CodeBadRequest, "invalid onboarding trait offer", cause)
}

func registrationUnavailable() error {
	return apperr.New(
		apperr.CodeInternalServer,
		"registration service unavailable",
		ErrInvalidRegistrationLifecycle,
	)
}

var _ identityports.RegistrationLifecycle = (*RegistrationLifecycle)(nil)
