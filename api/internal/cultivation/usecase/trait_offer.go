package usecase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	defaultMaximumTraitOffers = 3
	defaultTraitOfferTTL      = 10 * time.Minute
)

type traitOfferService struct {
	repository   ports.TraitOfferRepository
	transactions platformtx.UnitOfWork
	maximum      int
	ttl          time.Duration
	entropy      io.Reader
	entropyMu    sync.Mutex
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

func NewTraitOfferService(
	repository ports.TraitOfferRepository,
	transactions platformtx.UnitOfWork,
	options ports.TraitOfferServiceOptions,
) (ports.TraitOfferService, error) {
	if repository == nil || transactions == nil {
		return nil, entity.ErrTraitOfferUnavailable
	}
	if options.MaximumOffers == 0 {
		options.MaximumOffers = defaultMaximumTraitOffers
	}
	if options.OfferTTL == 0 {
		options.OfferTTL = defaultTraitOfferTTL
	}
	if options.MaximumOffers < 1 || options.MaximumOffers > 10 ||
		options.OfferTTL < time.Minute || options.OfferTTL > 24*time.Hour {
		return nil, entity.ErrTraitOfferInvalid
	}
	if options.Entropy == nil {
		options.Entropy = rand.Reader
	}
	return &traitOfferService{
		repository: repository, transactions: transactions,
		maximum: options.MaximumOffers, ttl: options.OfferTTL,
		entropy: options.Entropy,
		now:     func() time.Time { return time.Now().UTC() },
		newID:   uuid.NewV7,
	}, nil
}

// RefreshCatalog runs at API startup. A migrated and bootstrapped database
// carries no cultivation traits yet, so requiring a launchable catalog here
// would make a fresh installation unbootable: the administrator can only
// assemble the catalog through an API that would refuse to start.
func (service *traitOfferService) RefreshCatalog(ctx context.Context) error {
	return service.resyncCatalog(ctx)
}

func (service *traitOfferService) InvalidatePool(ctx context.Context) error {
	return service.resyncCatalog(ctx)
}

func (service *traitOfferService) resyncCatalog(ctx context.Context) error {
	if service == nil || ctx == nil || ctx.Err() != nil {
		return entity.ErrTraitOfferUnavailable
	}
	refresh := func(txCtx context.Context) error {
		if err := service.repository.LockCatalog(txCtx); err != nil {
			return err
		}
		_, hadCatalog, err := service.repository.LoadLatestCatalog(txCtx)
		if err != nil {
			return err
		}
		_, err = service.syncCatalogLocked(txCtx)
		if errors.Is(err, entity.ErrTraitOfferInvalid) && !hadCatalog {
			// Allow an administrator or seed transaction to assemble the first
			// usable catalog incrementally. Once any catalog has launched, every
			// mutation must yield another valid revision or it is rolled back.
			return nil
		}
		return err
	}
	if _, transactionErr := platformtx.Client(ctx, nil); transactionErr == nil {
		return refresh(ctx)
	} else if !errors.Is(transactionErr, platformtx.ErrTransactionRequired) {
		return transactionErr
	}
	return service.transactions.Within(ctx, refresh)
}

func (service *traitOfferService) Create(
	ctx context.Context,
	command ports.CreateOfferCommand,
) (*contract.TraitOfferResponse, error) {
	if service == nil || ctx == nil || ctx.Err() != nil ||
		command.PreSessionID == uuid.Nil || command.IdempotencyKey == uuid.Nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid trait offer request", entity.ErrTraitOfferInvalid)
	}
	logger.FromContext(ctx).Debug(
		"creating onboarding trait offer",
		zap.String("pre_session_id", command.PreSessionID.String()),
	)
	var offer entity.TraitOffer
	recovered := false
	err := service.transactions.Within(ctx, func(txCtx context.Context) error {
		if err := service.repository.LockPreSession(txCtx, command.PreSessionID); err != nil {
			return err
		}
		now := service.now().UTC()
		if err := service.repository.ExpireIssued(txCtx, command.PreSessionID, now); err != nil {
			return err
		}
		existing, found, err := service.repository.LoadByIdempotency(
			txCtx, command.PreSessionID, command.IdempotencyKey,
		)
		if err != nil {
			return err
		}
		if found {
			count, countErr := service.repository.CountForPreSession(txCtx, command.PreSessionID)
			if countErr != nil {
				return countErr
			}
			existing.Remaining = max(0, existing.Remaining-count)
			if existing.Status != entity.TraitOfferIssued {
				return traitOfferStateError(existing.Status)
			}
			offer = existing
			recovered = true
			logger.FromContext(txCtx).Debug(
				"recovered idempotent onboarding trait offer",
				zap.String("offer_id", existing.ID.String()),
				zap.Int64("offer_sequence", existing.Sequence),
			)
			return nil
		}

		catalog, found, err := service.repository.LoadLatestCatalog(txCtx)
		if err != nil {
			return err
		}
		if !found {
			catalog, err = service.syncCatalog(txCtx)
			if err != nil {
				return err
			}
		}
		count, err := service.repository.CountForPreSession(txCtx, command.PreSessionID)
		if err != nil {
			return err
		}
		if count >= catalog.MaximumOffers {
			return entity.ErrTraitOfferQuota
		}
		sequence, err := service.repository.NextSequence(txCtx, command.PreSessionID)
		if err != nil {
			return err
		}

		service.entropyMu.Lock()
		roots, sampleErr := sampleWeightedWithoutReplacement(
			service.entropy, catalog.RootPool, 1,
		)
		var talents []entity.CatalogTrait
		if sampleErr == nil {
			talents, sampleErr = sampleWeightedWithoutReplacement(
				service.entropy, catalog.TalentPool, 6,
			)
		}
		service.entropyMu.Unlock()
		if sampleErr != nil {
			return sampleErr
		}
		offerID, err := service.newID()
		if err != nil {
			return err
		}
		offer = entity.TraitOffer{
			ID: offerID, PreSessionID: command.PreSessionID,
			IdempotencyKey: command.IdempotencyKey, Sequence: sequence,
			CatalogRevisionID: catalog.ID, CatalogChecksum: catalog.Checksum,
			AlgorithmVersion: catalog.AlgorithmVersion,
			Root:             roots[0], Talents: append([]entity.CatalogTrait(nil), talents...),
			Status:    entity.TraitOfferIssued,
			Remaining: catalog.MaximumOffers - count - 1,
			IssuedAt:  now, ExpiresAt: now.Add(catalog.OfferTTL).UTC(),
		}
		offer.OfferChecksum, err = entity.TraitOfferChecksum(offer)
		if err != nil || !offer.Valid() {
			return entity.ErrTraitOfferInvalid
		}
		if err := service.repository.SupersedeIssued(txCtx, command.PreSessionID, now); err != nil {
			return err
		}
		return service.repository.StoreOffer(txCtx, offer)
	})
	if err != nil {
		return nil, mapTraitOfferError(err)
	}
	fields := []zap.Field{
		zap.String("offer_id", offer.ID.String()),
		zap.String("pre_session_id", offer.PreSessionID.String()),
		zap.String("catalog_revision_id", offer.CatalogRevisionID.String()),
		zap.Int64("offer_sequence", offer.Sequence),
		zap.Int("remaining_offers", offer.Remaining),
		zap.Time("expires_at", offer.ExpiresAt),
	}
	serverTime := service.now().UTC()
	if !serverTime.Before(offer.ExpiresAt) {
		return nil, mapTraitOfferError(entity.ErrTraitOfferExpired)
	}
	if recovered {
		logger.FromContext(ctx).Debug("onboarding trait offer replayed", fields...)
	} else {
		logger.FromContext(ctx).Info("onboarding trait offer issued", fields...)
	}
	return traitOfferResponse(offer, serverTime), nil
}

func (service *traitOfferService) syncCatalog(ctx context.Context) (entity.CatalogRevision, error) {
	if err := service.repository.LockCatalog(ctx); err != nil {
		return entity.CatalogRevision{}, err
	}
	return service.syncCatalogLocked(ctx)
}

func (service *traitOfferService) syncCatalogLocked(ctx context.Context) (entity.CatalogRevision, error) {
	live, err := service.repository.LoadLiveCatalog(ctx)
	if err != nil {
		return entity.CatalogRevision{}, err
	}
	roots := make([]entity.CatalogTrait, 0, len(live))
	talents := make([]entity.CatalogTrait, 0, len(live))
	seen := make(map[uuid.UUID]struct{}, len(live))
	for _, candidate := range live {
		if candidate.ID == uuid.Nil || candidate.Revision < 1 || candidate.Weight < 1 ||
			candidate.Name == "" || candidate.RarityID == uuid.Nil {
			return entity.CatalogRevision{}, entity.ErrTraitOfferInvalid
		}
		if _, duplicate := seen[candidate.ID]; duplicate {
			return entity.CatalogRevision{}, entity.ErrTraitOfferInvalid
		}
		seen[candidate.ID] = struct{}{}
		switch candidate.Type {
		case entity.TraitTypeRootBone:
			roots = append(roots, candidate)
		case entity.TraitTypeTalent:
			talents = append(talents, candidate)
		default:
			return entity.CatalogRevision{}, entity.ErrTraitOfferInvalid
		}
	}
	sortCatalogTraits(roots)
	sortCatalogTraits(talents)
	checksum, err := entity.CatalogChecksum(roots, talents, service.maximum, service.ttl)
	if err != nil {
		return entity.CatalogRevision{}, err
	}
	latest, found, err := service.repository.LoadLatestCatalog(ctx)
	if err != nil {
		return entity.CatalogRevision{}, err
	}
	if found && latest.Checksum == checksum {
		return latest, nil
	}
	revision := int64(1)
	if found {
		revision = latest.Revision + 1
	}
	id, err := service.newID()
	if err != nil {
		return entity.CatalogRevision{}, err
	}
	catalog := entity.CatalogRevision{
		ID: id, Revision: revision, AlgorithmVersion: entity.OfferAlgorithmVersion,
		Checksum: checksum, RootPool: roots, TalentPool: talents,
		MaximumOffers: service.maximum, OfferTTL: service.ttl,
		CreatedAt: service.now().UTC(),
	}
	if !catalog.Valid() {
		return entity.CatalogRevision{}, entity.ErrTraitOfferInvalid
	}
	if err := service.repository.StoreCatalog(ctx, catalog); err != nil {
		return entity.CatalogRevision{}, err
	}
	return catalog, nil
}

func sortCatalogTraits(candidates []entity.CatalogTrait) {
	slices.SortFunc(candidates, func(left, right entity.CatalogTrait) int {
		return bytes.Compare(left.ID[:], right.ID[:])
	})
}

func traitOfferResponse(offer entity.TraitOffer, serverTime time.Time) *contract.TraitOfferResponse {
	talents := make([]*contract.TraitOfferTraitResponse, len(offer.Talents))
	for index, talent := range offer.Talents {
		talents[index] = traitOfferTraitResponse(talent)
	}
	return &contract.TraitOfferResponse{
		ID: offer.ID, Sequence: offer.Sequence,
		CatalogRevisionID: offer.CatalogRevisionID,
		CatalogChecksum:   hex.EncodeToString(offer.CatalogChecksum[:]),
		AlgorithmVersion:  offer.AlgorithmVersion,
		OfferChecksum:     hex.EncodeToString(offer.OfferChecksum[:]),
		RootBone:          traitOfferTraitResponse(offer.Root), Talents: talents,
		RemainingOffers: offer.Remaining, ExpiresAt: offer.ExpiresAt.UTC(),
		ServerTime: serverTime.UTC(),
	}
}

func traitOfferTraitResponse(candidate entity.CatalogTrait) *contract.TraitOfferTraitResponse {
	return &contract.TraitOfferTraitResponse{
		ID: candidate.ID, Revision: candidate.Revision, Type: candidate.Type,
		Name: candidate.Name, Description: candidate.Description,
		RarityID: candidate.RarityID, RarityCode: candidate.RarityCode,
		RarityName: candidate.RarityName,
	}
}

func traitOfferStateError(status string) error {
	switch status {
	case entity.TraitOfferExpired:
		return entity.ErrTraitOfferExpired
	case entity.TraitOfferSuperseded:
		return entity.ErrTraitOfferSuperseded
	case entity.TraitOfferConsumed:
		return entity.ErrTraitOfferConsumed
	default:
		return entity.ErrTraitOfferUnavailable
	}
}

func mapTraitOfferError(err error) error {
	switch {
	case errors.Is(err, entity.ErrTraitOfferQuota):
		return apperr.New(apperr.CodeTooManyRequests, "trait offer quota exhausted", err)
	case errors.Is(err, entity.ErrTraitOfferExpired),
		errors.Is(err, entity.ErrTraitOfferSuperseded),
		errors.Is(err, entity.ErrTraitOfferConsumed),
		errors.Is(err, entity.ErrTraitOfferConflict):
		return apperr.New(apperr.CodeConflict, "trait offer is no longer available", err)
	case errors.Is(err, entity.ErrTraitOfferInvalid):
		return apperr.New(apperr.CodeBadRequest, "invalid trait offer request", err)
	case errors.Is(err, errTraitOfferEntropy):
		return apperr.New(apperr.CodeInternalServer, "trait offer entropy unavailable", err)
	default:
		return apperr.New(apperr.CodeInternalServer, "trait offer service unavailable", err)
	}
}

var _ ports.TraitOfferService = (*traitOfferService)(nil)
