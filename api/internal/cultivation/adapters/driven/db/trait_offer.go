package db

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const traitOfferCatalogLock = "judgify.cultivation.registration-trait-catalog.v1"

type TraitOfferRepository struct{ client *internalent.EntClient }

func NewTraitOfferRepository(client *internalent.EntClient) (*TraitOfferRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrTraitOfferUnavailable
	}
	return &TraitOfferRepository{client: client}, nil
}

func (repository *TraitOfferRepository) LockCatalog(ctx context.Context) error {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	_, err := repository.client.DB(ctx).Trait.ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		traitOfferCatalogLock,
	)
	if err != nil {
		return fmt.Errorf("lock cultivation catalog: %w", err)
	}
	return nil
}

func (repository *TraitOfferRepository) LockPreSession(ctx context.Context, preSessionID uuid.UUID) error {
	if preSessionID == uuid.Nil {
		return entity.ErrTraitOfferInvalid
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	_, err := repository.client.DB(ctx).Trait.ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"judgify.cultivation.trait-offer."+preSessionID.String(),
	)
	if err != nil {
		return fmt.Errorf("lock onboarding pre-session: %w", err)
	}
	return nil
}

func (repository *TraitOfferRepository) LoadLatestCatalog(ctx context.Context) (entity.CatalogRevision, bool, error) {
	rows, err := repository.client.DB(ctx).Trait.QueryContext(ctx, `
		SELECT id, revision, algorithm_version, catalog_checksum,
		       root_pool, talent_pool, maximum_offers, offer_ttl_seconds, created_at
		FROM cultivation_catalog_revisions
		ORDER BY revision DESC
		LIMIT 1`)
	if err != nil {
		return entity.CatalogRevision{}, false, fmt.Errorf("load cultivation catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.CatalogRevision{}, false, rows.Err()
	}
	catalog, err := scanCatalog(rows)
	return catalog, err == nil, err
}

func (repository *TraitOfferRepository) LoadLiveCatalog(ctx context.Context) ([]entity.CatalogTrait, error) {
	rows, err := repository.client.DB(ctx).Trait.QueryContext(ctx, `
		SELECT trait.id, trait.version, trait.code, trait.type, trait.name,
		       trait.description, trait.display_order,
		       rarity.id, rarity.code, rarity.name, rarity.weight,
		       effect.id, effect.trait_id, effect.revision,
		       effect.schema_version, effect.effect, effect.target_element_ids,
		       effect.checksum, effect.effective_at
		FROM traits trait
		JOIN rarities rarity ON rarity.id = trait.rarity_id AND rarity.deleted_at IS NULL
		JOIN LATERAL (
			SELECT id, trait_id, revision, schema_version, effect,
			       target_element_ids, checksum, effective_at
			FROM trait_effect_revisions
			WHERE trait_id = trait.id AND revision = trait.version
			ORDER BY effective_at DESC, revision DESC, id DESC
			LIMIT 1
		) effect ON true
		WHERE trait.deleted_at IS NULL AND trait.active = true
		ORDER BY trait.id
		FOR SHARE OF trait, rarity`)
	if err != nil {
		return nil, fmt.Errorf("load live cultivation catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()
	candidates := make([]entity.CatalogTrait, 0, 16)
	for rows.Next() {
		var candidate entity.CatalogTrait
		var effect entity.TraitEffectRevision
		var schemaVersion int
		var effectDocument, targetDocument, checksum []byte
		if err := rows.Scan(
			&candidate.ID, &candidate.Revision, &candidate.Code, &candidate.Type,
			&candidate.Name, &candidate.Description, &candidate.DisplayOrder,
			&candidate.RarityID,
			&candidate.RarityCode, &candidate.RarityName, &candidate.Weight,
			&effect.ID, &effect.TraitID, &effect.Revision, &schemaVersion,
			&effectDocument, &targetDocument, &checksum, &effect.EffectiveAt,
		); err != nil {
			return nil, fmt.Errorf("scan live cultivation catalog: %w", err)
		}
		if schemaVersion != entity.TraitEffectSchemaVersion || len(checksum) != 32 ||
			commonjson.UnmarshalStrictLimit(
				effectDocument, &effect.Effect, entity.MaximumTraitEffectBytes,
			) != nil || commonjson.UnmarshalStrictLimit(
			targetDocument, &effect.TargetElementIDs, entity.MaximumTraitEffectBytes,
		) != nil {
			return nil, entity.ErrTraitOfferUnavailable
		}
		copy(effect.Checksum[:], checksum)
		effect.EffectiveAt = effect.EffectiveAt.UTC()
		if !effect.Valid() || effect.TraitID != candidate.ID ||
			effect.Revision != candidate.Revision {
			return nil, entity.ErrTraitOfferUnavailable
		}
		candidate.EffectRevision = effect
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (repository *TraitOfferRepository) StoreCatalog(ctx context.Context, catalog entity.CatalogRevision) error {
	if !catalog.Valid() {
		return entity.ErrTraitOfferInvalid
	}
	rootJSON, err := json.Marshal(catalog.RootPool)
	if err != nil {
		return err
	}
	talentJSON, err := json.Marshal(catalog.TalentPool)
	if err != nil {
		return err
	}
	_, err = repository.client.DB(ctx).Trait.ExecContext(ctx, `
		INSERT INTO cultivation_catalog_revisions (
			id, revision, algorithm_version, catalog_checksum, root_pool,
			talent_pool, maximum_offers, offer_ttl_seconds, created_at
		) VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7,$8,$9)`,
		catalog.ID, catalog.Revision, catalog.AlgorithmVersion,
		catalog.Checksum[:], rootJSON, talentJSON, catalog.MaximumOffers,
		int64(catalog.OfferTTL/time.Second), catalog.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("store cultivation catalog revision: %w", err)
	}
	return nil
}

func (repository *TraitOfferRepository) LoadByIdempotency(
	ctx context.Context,
	preSessionID uuid.UUID,
	idempotencyKey uuid.UUID,
) (entity.TraitOffer, bool, error) {
	return repository.loadOffer(
		ctx,
		"offer.pre_session_id = $1 AND offer.idempotency_key = $2",
		[]any{preSessionID, idempotencyKey},
		true,
	)
}

func (repository *TraitOfferRepository) CountForPreSession(ctx context.Context, preSessionID uuid.UUID) (int, error) {
	rows, err := repository.client.DB(ctx).Trait.QueryContext(ctx, `
		SELECT COUNT(*) FROM onboarding_trait_offers WHERE pre_session_id = $1`,
		preSessionID,
	)
	if err != nil {
		return 0, fmt.Errorf("count onboarding trait offers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var count int
	if !rows.Next() {
		return 0, sql.ErrNoRows
	}
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func (repository *TraitOfferRepository) NextSequence(ctx context.Context, preSessionID uuid.UUID) (int64, error) {
	rows, err := repository.client.DB(ctx).Trait.QueryContext(ctx, `
		SELECT COALESCE(MAX(offer_sequence), 0) + 1
		FROM onboarding_trait_offers
		WHERE pre_session_id = $1`, preSessionID)
	if err != nil {
		return 0, fmt.Errorf("next onboarding trait offer sequence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var sequence int64
	if !rows.Next() || rows.Scan(&sequence) != nil || sequence < 1 {
		return 0, entity.ErrTraitOfferUnavailable
	}
	return sequence, rows.Err()
}

func (repository *TraitOfferRepository) ExpireIssued(ctx context.Context, preSessionID uuid.UUID, now time.Time) error {
	_, err := repository.client.DB(ctx).Trait.ExecContext(ctx, `
		UPDATE onboarding_trait_offers
		SET status = 'expired'
		WHERE pre_session_id = $1 AND status = 'issued' AND expires_at <= $2`,
		preSessionID, now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("expire onboarding trait offers: %w", err)
	}
	return nil
}

func (repository *TraitOfferRepository) SupersedeIssued(ctx context.Context, preSessionID uuid.UUID, now time.Time) error {
	_, err := repository.client.DB(ctx).Trait.ExecContext(ctx, `
		UPDATE onboarding_trait_offers
		SET status = 'superseded'
		WHERE pre_session_id = $1 AND status = 'issued' AND expires_at > $2`,
		preSessionID, now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("supersede onboarding trait offer: %w", err)
	}
	return nil
}

func (repository *TraitOfferRepository) StoreOffer(ctx context.Context, offer entity.TraitOffer) error {
	if !offer.Valid() || offer.Status != entity.TraitOfferIssued {
		return entity.ErrTraitOfferInvalid
	}
	_, err := repository.client.DB(ctx).Trait.ExecContext(ctx, `
		INSERT INTO onboarding_trait_offers (
			id, pre_session_id, idempotency_key, offer_sequence,
			catalog_revision_id, algorithm_version, root_bone_trait_id,
			root_bone_trait_revision, offer_checksum, status, issued_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'issued',$10,$11)`,
		offer.ID, offer.PreSessionID, offer.IdempotencyKey, offer.Sequence,
		offer.CatalogRevisionID, offer.AlgorithmVersion, offer.Root.ID,
		offer.Root.Revision, offer.OfferChecksum[:], offer.IssuedAt.UTC(),
		offer.ExpiresAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("store onboarding trait offer: %w", err)
	}
	for order, talent := range offer.Talents {
		if _, err := repository.client.DB(ctx).Trait.ExecContext(ctx, `
			INSERT INTO onboarding_trait_offer_talents (
				offer_id, trait_id, trait_revision, display_order
			) VALUES ($1,$2,$3,$4)`,
			offer.ID, talent.ID, talent.Revision, order,
		); err != nil {
			return fmt.Errorf("store onboarding trait offer talent: %w", err)
		}
	}
	return nil
}

func (repository *TraitOfferRepository) LockOffer(ctx context.Context, offerID uuid.UUID) (entity.TraitOffer, error) {
	offer, found, err := repository.loadOffer(
		ctx, "offer.id = $1", []any{offerID}, true,
	)
	if err != nil {
		return entity.TraitOffer{}, err
	}
	if !found {
		return entity.TraitOffer{}, entity.ErrTraitOfferUnavailable
	}
	return offer, nil
}

func (repository *TraitOfferRepository) ConsumeOffer(
	ctx context.Context,
	offerID uuid.UUID,
	userID uuid.UUID,
	consumedAt time.Time,
) error {
	result, err := repository.client.DB(ctx).Trait.ExecContext(ctx, `
		UPDATE onboarding_trait_offers
		SET status = 'consumed', consumed_by_user_id = $2, consumed_at = $3
		WHERE id = $1 AND status = 'issued' AND expires_at > $3
		  AND consumed_by_user_id IS NULL`,
		offerID, userID, consumedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("consume onboarding trait offer: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return entity.ErrTraitOfferConflict
	}
	return nil
}

func (repository *TraitOfferRepository) loadOffer(
	ctx context.Context,
	predicate string,
	arguments []any,
	lock bool,
) (entity.TraitOffer, bool, error) {
	statement := `
		SELECT offer.id, offer.pre_session_id, offer.idempotency_key,
		       offer.offer_sequence, offer.catalog_revision_id,
		       offer.algorithm_version, offer.root_bone_trait_id,
		       offer.root_bone_trait_revision, offer.offer_checksum,
		       offer.status, offer.issued_at, offer.expires_at,
		       offer.consumed_by_user_id, offer.consumed_at,
		       catalog.catalog_checksum, catalog.root_pool, catalog.talent_pool,
		       catalog.maximum_offers
		FROM onboarding_trait_offers offer
		JOIN cultivation_catalog_revisions catalog ON catalog.id = offer.catalog_revision_id
		WHERE ` + predicate
	if lock {
		statement += " FOR UPDATE OF offer"
	}
	rows, err := repository.client.DB(ctx).Trait.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return entity.TraitOffer{}, false, fmt.Errorf("load onboarding trait offer: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return entity.TraitOffer{}, false, rows.Err()
	}
	var offer entity.TraitOffer
	var rootID uuid.UUID
	var rootRevision int64
	var offerChecksum, catalogChecksum, rootJSON, talentJSON []byte
	var consumedBy uuid.NullUUID
	var consumedAt sql.NullTime
	var maximumOffers int
	if err := rows.Scan(
		&offer.ID, &offer.PreSessionID, &offer.IdempotencyKey, &offer.Sequence,
		&offer.CatalogRevisionID, &offer.AlgorithmVersion, &rootID,
		&rootRevision, &offerChecksum, &offer.Status, &offer.IssuedAt,
		&offer.ExpiresAt, &consumedBy, &consumedAt, &catalogChecksum,
		&rootJSON, &talentJSON, &maximumOffers,
	); err != nil {
		return entity.TraitOffer{}, false, fmt.Errorf("scan onboarding trait offer: %w", err)
	}
	if len(offerChecksum) != 32 || len(catalogChecksum) != 32 {
		return entity.TraitOffer{}, false, entity.ErrTraitOfferUnavailable
	}
	copy(offer.OfferChecksum[:], offerChecksum)
	copy(offer.CatalogChecksum[:], catalogChecksum)
	if consumedBy.Valid {
		value := consumedBy.UUID
		offer.ConsumedByUserID = &value
	}
	if consumedAt.Valid {
		value := consumedAt.Time.UTC()
		offer.ConsumedAt = &value
	}
	offer.Remaining = maximumOffers
	var roots, talents []entity.CatalogTrait
	if json.Unmarshal(rootJSON, &roots) != nil || json.Unmarshal(talentJSON, &talents) != nil {
		return entity.TraitOffer{}, false, entity.ErrTraitOfferUnavailable
	}
	// These pools come from the same checksum-less JSON that scanCatalog reads,
	// so their effect revisions need the same derivation before anything
	// validates them.
	if rehydrateEffectChecksums(roots) != nil || rehydrateEffectChecksums(talents) != nil {
		return entity.TraitOffer{}, false, entity.ErrTraitOfferUnavailable
	}
	offer.Root, _ = catalogTraitByIdentity(roots, rootID, rootRevision)
	_ = rows.Close()
	talentRows, err := repository.client.DB(ctx).Trait.QueryContext(ctx, `
		SELECT trait_id, trait_revision
		FROM onboarding_trait_offer_talents
		WHERE offer_id = $1
		ORDER BY display_order`, offer.ID)
	if err != nil {
		return entity.TraitOffer{}, false, err
	}
	defer func() { _ = talentRows.Close() }()
	offer.Talents = make([]entity.CatalogTrait, 0, 6)
	for talentRows.Next() {
		var talentID uuid.UUID
		var revision int64
		if err := talentRows.Scan(&talentID, &revision); err != nil {
			return entity.TraitOffer{}, false, err
		}
		candidate, found := catalogTraitByIdentity(talents, talentID, revision)
		if !found {
			return entity.TraitOffer{}, false, entity.ErrTraitOfferUnavailable
		}
		offer.Talents = append(offer.Talents, candidate)
	}
	if err := talentRows.Err(); err != nil {
		return entity.TraitOffer{}, false, err
	}
	if !offer.Valid() {
		return entity.TraitOffer{}, false, entity.ErrTraitOfferUnavailable
	}
	calculated, err := entity.TraitOfferChecksum(offer)
	if err != nil || subtle.ConstantTimeCompare(calculated[:], offer.OfferChecksum[:]) != 1 {
		return entity.TraitOffer{}, false, entity.ErrTraitOfferUnavailable
	}
	return offer, true, nil
}

func rehydrateEffectChecksums(pool []entity.CatalogTrait) error {
	for index := range pool {
		checksum, err := pool[index].EffectRevision.CalculateChecksum()
		if err != nil {
			return err
		}
		pool[index].EffectRevision.Checksum = checksum
	}
	return nil
}

func scanCatalog(scanner interface{ Scan(...any) error }) (entity.CatalogRevision, error) {
	var catalog entity.CatalogRevision
	var checksum, rootJSON, talentJSON []byte
	var ttlSeconds int64
	if err := scanner.Scan(
		&catalog.ID, &catalog.Revision, &catalog.AlgorithmVersion,
		&checksum, &rootJSON, &talentJSON, &catalog.MaximumOffers,
		&ttlSeconds, &catalog.CreatedAt,
	); err != nil {
		return entity.CatalogRevision{}, err
	}
	if len(checksum) != 32 || ttlSeconds < 1 ||
		json.Unmarshal(rootJSON, &catalog.RootPool) != nil ||
		json.Unmarshal(talentJSON, &catalog.TalentPool) != nil {
		return entity.CatalogRevision{}, entity.ErrTraitOfferUnavailable
	}
	copy(catalog.Checksum[:], checksum)
	catalog.OfferTTL = time.Duration(ttlSeconds) * time.Second
	// Each effect revision's checksum is excluded from its JSON form, so the
	// stored pools carry none and every revision would fail validation on the
	// way back in. Deriving them again is safe because the catalog checksum
	// verified below covers exactly the JSON that was persisted.
	if err := rehydrateEffectChecksums(catalog.RootPool); err != nil {
		return entity.CatalogRevision{}, entity.ErrTraitOfferUnavailable
	}
	if err := rehydrateEffectChecksums(catalog.TalentPool); err != nil {
		return entity.CatalogRevision{}, entity.ErrTraitOfferUnavailable
	}
	if !catalog.Valid() {
		return entity.CatalogRevision{}, entity.ErrTraitOfferUnavailable
	}
	calculated, err := entity.CatalogChecksum(
		catalog.RootPool, catalog.TalentPool, catalog.MaximumOffers, catalog.OfferTTL,
	)
	if err != nil || subtle.ConstantTimeCompare(calculated[:], catalog.Checksum[:]) != 1 {
		return entity.CatalogRevision{}, entity.ErrTraitOfferUnavailable
	}
	return catalog, nil
}

func catalogTraitByIdentity(
	candidates []entity.CatalogTrait,
	id uuid.UUID,
	revision int64,
) (entity.CatalogTrait, bool) {
	for _, candidate := range candidates {
		if candidate.ID == id && candidate.Revision == revision {
			return candidate, true
		}
	}
	return entity.CatalogTrait{}, false
}

var _ ports.TraitOfferRepository = (*TraitOfferRepository)(nil)
