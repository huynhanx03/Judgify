package db

import (
	"context"
	"crypto/subtle"
	stdsql "database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

const maximumRewardProfileTraits = 4

type rewardProfileExecutor interface {
	ExecContext(context.Context, string, ...any) (stdsql.Result, error)
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

type RewardProfileRepository struct {
	client *dbent.EntClient
	newID  func() (uuid.UUID, error)
}

func NewRewardProfileRepository(client *dbent.EntClient) (*RewardProfileRepository, error) {
	if client == nil || client.Client() == nil {
		return nil, entity.ErrInvalidRewardProfile
	}
	return &RewardProfileRepository{client: client, newID: uuid.NewV7}, nil
}

var _ ports.RewardProfileRepository = (*RewardProfileRepository)(nil)

func (repository *RewardProfileRepository) Current(
	ctx context.Context,
	userID uuid.UUID,
) (entity.UserRewardProfileRevision, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	return loadRewardProfile(ctx, repository.client.DB(ctx).UserRewardProfileRevision, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE user_id = $1
		ORDER BY effective_at DESC, revision DESC, id DESC
		LIMIT 1`, userID)
}

func (repository *RewardProfileRepository) EffectiveAt(
	ctx context.Context,
	userID uuid.UUID,
	at time.Time,
) (entity.UserRewardProfileRevision, error) {
	if repository == nil || repository.client == nil || ctx == nil ||
		userID == uuid.Nil || at.IsZero() {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	return loadRewardProfile(ctx, repository.client.DB(ctx).UserRewardProfileRevision, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE user_id = $1 AND effective_at <= $2
		ORDER BY effective_at DESC, revision DESC, id DESC
		LIMIT 1`, userID, at.UTC())
}

func loadEffectiveRewardProfile(
	ctx context.Context,
	executor rewardProfileExecutor,
	userID uuid.UUID,
	at time.Time,
) (entity.UserRewardProfileRevision, error) {
	if ctx == nil || executor == nil || userID == uuid.Nil || at.IsZero() {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	return loadRewardProfile(ctx, executor, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE user_id = $1 AND effective_at <= $2
		ORDER BY effective_at DESC, revision DESC, id DESC
		LIMIT 1`, userID, at.UTC())
}

func (repository *RewardProfileRepository) PreviewAssignment(
	ctx context.Context,
	userID uuid.UUID,
	selection entity.RewardProfileSelection,
) (entity.RewardProfileAssignmentPreview, error) {
	if repository == nil || repository.client == nil || ctx == nil || userID == uuid.Nil {
		return entity.RewardProfileAssignmentPreview{}, entity.ErrInvalidRewardProfile
	}
	selection, err := entity.NormalizeRewardProfileSelection(selection)
	if err != nil {
		return entity.RewardProfileAssignmentPreview{}, err
	}
	executor := repository.client.DB(ctx).UserRewardProfileRevision
	current, err := loadRewardProfile(ctx, executor, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE user_id = $1
		ORDER BY effective_at DESC, revision DESC, id DESC
		LIMIT 1`, userID)
	if err != nil {
		return entity.RewardProfileAssignmentPreview{}, err
	}
	traits, err := loadRewardProfileCandidates(ctx, executor, selection, false)
	if err != nil {
		return entity.RewardProfileAssignmentPreview{}, err
	}
	return entity.NewRewardProfileAssignmentPreview(current, traits)
}

func (repository *RewardProfileRepository) Assign(
	ctx context.Context,
	command entity.RewardProfileAssignmentCommand,
) (entity.RewardProfileAssignmentResult, error) {
	if repository == nil || repository.client == nil || repository.newID == nil || ctx == nil {
		return entity.RewardProfileAssignmentResult{}, entity.ErrInvalidRewardProfile
	}
	command, err := entity.NormalizeRewardProfileAssignmentCommand(command)
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, err
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return entity.RewardProfileAssignmentResult{}, err
	}
	executor := repository.client.DB(ctx).UserRewardProfileRevision
	requestChecksum := entity.RewardProfileAssignmentRequestChecksum(command)
	if result, found, replayErr := readRewardProfileReplay(
		ctx, executor, command.ID, requestChecksum,
	); replayErr != nil || found {
		return result, replayErr
	}
	if err := acquireRewardTimelineLock(ctx, executor); err != nil {
		return entity.RewardProfileAssignmentResult{}, fmt.Errorf("lock reward profile timeline: %w", err)
	}
	if result, found, replayErr := readRewardProfileReplay(
		ctx, executor, command.ID, requestChecksum,
	); replayErr != nil || found {
		return result, replayErr
	}
	// user_reward_profile_revisions is append-only; rows can never be updated
	// or deleted. acquireRewardTimelineLock (pg_advisory_xact_lock) taken above
	// already serialises all writers, so FOR UPDATE is both unprivileged and
	// redundant.
	current, err := loadRewardProfile(ctx, executor, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE user_id = $1
		ORDER BY revision DESC
		LIMIT 1`, command.UserID)
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, err
	}
	traits, err := loadRewardProfileCandidates(ctx, executor, command.Selection, true)
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, err
	}
	preview, err := entity.NewRewardProfileAssignmentPreview(current, traits)
	if err != nil || current.Revision != command.ExpectedRevision ||
		subtle.ConstantTimeCompare(preview.Checksum[:], command.PreviewChecksum[:]) != 1 {
		return entity.RewardProfileAssignmentResult{}, entity.ErrRewardProfileConflict
	}
	effectiveAt, err := rewardTimelineNow(ctx, executor)
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, fmt.Errorf("read reward profile effective time: %w", err)
	}
	profileID, err := repository.newID()
	if err != nil || profileID == uuid.Nil {
		return entity.RewardProfileAssignmentResult{}, entity.ErrInvalidRewardProfile
	}
	profile, err := entity.NewUserRewardProfileRevision(
		profileID,
		command.UserID,
		current.Revision+1,
		entity.RewardProfileSourceAdministrative,
		nil,
		&command.ID,
		requestChecksum,
		effectiveAt,
		traits,
	)
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, err
	}
	if err := repository.append(ctx, executor, profile); err != nil {
		return entity.RewardProfileAssignmentResult{}, err
	}
	return entity.RewardProfileAssignmentResult{Profile: profile}, nil
}

func (repository *RewardProfileRepository) ProvisionOnboarding(
	ctx context.Context,
	userID uuid.UUID,
	offer entity.TraitOffer,
	selection entity.RewardProfileSelection,
) (entity.UserRewardProfileRevision, error) {
	if repository == nil || repository.client == nil || repository.newID == nil ||
		ctx == nil || userID == uuid.Nil || !offer.Valid() {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	selection, err := entity.NormalizeRewardProfileSelection(selection)
	if err != nil || selection.RootTraitID != offer.Root.ID {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	traits, err := rewardProfileTraitsFromOffer(offer, selection)
	if err != nil {
		return entity.UserRewardProfileRevision{}, err
	}
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return entity.UserRewardProfileRevision{}, err
	}
	executor := repository.client.DB(ctx).UserRewardProfileRevision
	requestChecksum := entity.RewardProfileOnboardingRequestChecksum(userID, offer, selection)
	if profile, found, replayErr := readOnboardingRewardProfileReplay(
		ctx, executor, offer.ID, userID, requestChecksum,
	); replayErr != nil || found {
		return profile, replayErr
	}
	if err := acquireRewardTimelineLock(ctx, executor); err != nil {
		return entity.UserRewardProfileRevision{}, fmt.Errorf("lock onboarding reward profile timeline: %w", err)
	}
	if profile, found, replayErr := readOnboardingRewardProfileReplay(
		ctx, executor, offer.ID, userID, requestChecksum,
	); replayErr != nil || found {
		return profile, replayErr
	}
	// user_reward_profile_revisions is append-only; rows can never be updated
	// or deleted. acquireRewardTimelineLock (pg_advisory_xact_lock) taken above
	// already serialises all onboarding writers, so FOR UPDATE is both
	// unprivileged and redundant.
	var existing int64
	err = querySingleRow(ctx, executor, `
		SELECT revision
		FROM user_reward_profile_revisions
		WHERE user_id = $1
		ORDER BY revision DESC
		LIMIT 1`, userID).Scan(&existing)
	if err == nil || !errors.Is(err, stdsql.ErrNoRows) {
		if err != nil {
			return entity.UserRewardProfileRevision{}, fmt.Errorf("lock onboarding reward profile: %w", err)
		}
		return entity.UserRewardProfileRevision{}, entity.ErrRewardProfileConflict
	}
	effectiveAt, err := rewardTimelineNow(ctx, executor)
	if err != nil {
		return entity.UserRewardProfileRevision{}, fmt.Errorf("read onboarding reward profile time: %w", err)
	}
	profileID, err := repository.newID()
	if err != nil || profileID == uuid.Nil {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	profile, err := entity.NewUserRewardProfileRevision(
		profileID,
		userID,
		1,
		entity.RewardProfileSourceOnboarding,
		&offer.ID,
		nil,
		requestChecksum,
		effectiveAt,
		traits,
	)
	if err != nil {
		return entity.UserRewardProfileRevision{}, err
	}
	if err := repository.append(ctx, executor, profile); err != nil {
		return entity.UserRewardProfileRevision{}, err
	}
	return profile, nil
}

func (repository *RewardProfileRepository) append(
	ctx context.Context,
	executor rewardProfileExecutor,
	profile entity.UserRewardProfileRevision,
) error {
	if !profile.Valid() {
		return entity.ErrInvalidRewardProfile
	}
	_, err := executor.ExecContext(ctx, `
		INSERT INTO user_reward_profile_revisions (
			id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`,
		profile.ID,
		profile.UserID,
		profile.Revision,
		profile.AssignmentChecksum[:],
		profile.Source,
		profile.SourceOfferID,
		profile.CommandID,
		profile.RequestChecksum[:],
		profile.EffectiveAt,
	)
	if err != nil {
		return fmt.Errorf("append reward profile revision: %w", err)
	}
	for _, assignment := range profile.Traits {
		assignmentID, idErr := repository.newID()
		if idErr != nil || assignmentID == uuid.Nil {
			return entity.ErrInvalidRewardProfile
		}
		_, err = executor.ExecContext(ctx, `
			INSERT INTO user_reward_profile_traits (
				id, profile_revision_id, trait_id, trait_effect_revision_id,
				trait_code, trait_type, trait_name, trait_description,
				rarity_code, rarity_name, slot, slot_order, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			assignmentID,
			profile.ID,
			assignment.TraitID,
			assignment.EffectRevision.ID,
			assignment.TraitCode,
			assignment.TraitType,
			assignment.TraitName,
			assignment.Description,
			assignment.RarityCode,
			assignment.RarityName,
			assignment.Slot,
			assignment.SlotOrder,
			profile.EffectiveAt,
		)
		if err != nil {
			return fmt.Errorf("append reward profile trait: %w", err)
		}
	}
	return repository.replaceUserTraitProjection(ctx, executor, profile)
}

func (repository *RewardProfileRepository) replaceUserTraitProjection(
	ctx context.Context,
	executor rewardProfileExecutor,
	profile entity.UserRewardProfileRevision,
) error {
	if _, err := executor.ExecContext(ctx, `DELETE FROM user_traits WHERE user_id = $1`, profile.UserID); err != nil {
		return fmt.Errorf("clear user trait projection: %w", err)
	}
	for _, assignment := range profile.Traits {
		projectionID, err := repository.newID()
		if err != nil || projectionID == uuid.Nil {
			return entity.ErrInvalidRewardProfile
		}
		if _, err := executor.ExecContext(ctx, `
			INSERT INTO user_traits (
				id, created_at, updated_at, user_id, trait_id
			) VALUES ($1,$2,$2,$3,$4)`,
			projectionID,
			profile.EffectiveAt,
			profile.UserID,
			assignment.TraitID,
		); err != nil {
			return fmt.Errorf("rebuild user trait projection: %w", err)
		}
	}
	return nil
}

func readRewardProfileReplay(
	ctx context.Context,
	executor rewardProfileExecutor,
	commandID uuid.UUID,
	requestChecksum [32]byte,
) (entity.RewardProfileAssignmentResult, bool, error) {
	profile, err := loadRewardProfile(ctx, executor, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE command_id = $1`, commandID)
	if errors.Is(err, entity.ErrRewardProfileNotFound) {
		return entity.RewardProfileAssignmentResult{}, false, nil
	}
	if err != nil {
		return entity.RewardProfileAssignmentResult{}, false, err
	}
	if subtle.ConstantTimeCompare(profile.RequestChecksum[:], requestChecksum[:]) != 1 {
		return entity.RewardProfileAssignmentResult{}, true, entity.ErrRewardProfileConflict
	}
	return entity.RewardProfileAssignmentResult{Profile: profile, Replayed: true}, true, nil
}

func readOnboardingRewardProfileReplay(
	ctx context.Context,
	executor rewardProfileExecutor,
	offerID uuid.UUID,
	userID uuid.UUID,
	requestChecksum [32]byte,
) (entity.UserRewardProfileRevision, bool, error) {
	profile, err := loadRewardProfile(ctx, executor, `
		SELECT id, user_id, revision, assignment_checksum, source,
			source_offer_id, command_id, request_checksum, effective_at
		FROM user_reward_profile_revisions
		WHERE source_offer_id = $1`, offerID)
	if errors.Is(err, entity.ErrRewardProfileNotFound) {
		return entity.UserRewardProfileRevision{}, false, nil
	}
	if err != nil {
		return entity.UserRewardProfileRevision{}, false, err
	}
	if profile.UserID != userID ||
		subtle.ConstantTimeCompare(profile.RequestChecksum[:], requestChecksum[:]) != 1 {
		return entity.UserRewardProfileRevision{}, true, entity.ErrRewardProfileConflict
	}
	return profile, true, nil
}

func loadRewardProfile(
	ctx context.Context,
	executor rewardProfileExecutor,
	statement string,
	arguments ...any,
) (entity.UserRewardProfileRevision, error) {
	if ctx == nil || executor == nil {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	var profile entity.UserRewardProfileRevision
	var assignmentChecksum, requestChecksum []byte
	var sourceOfferID, commandID stdsql.NullString
	err := querySingleRow(ctx, executor, statement, arguments...).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Revision,
		&assignmentChecksum,
		&profile.Source,
		&sourceOfferID,
		&commandID,
		&requestChecksum,
		&profile.EffectiveAt,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return entity.UserRewardProfileRevision{}, entity.ErrRewardProfileNotFound
	}
	if err != nil {
		return entity.UserRewardProfileRevision{}, err
	}
	if len(assignmentChecksum) != 32 || len(requestChecksum) != 32 {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	copy(profile.AssignmentChecksum[:], assignmentChecksum)
	copy(profile.RequestChecksum[:], requestChecksum)
	profile.EffectiveAt = profile.EffectiveAt.UTC()
	if sourceOfferID.Valid {
		value, parseErr := uuid.Parse(sourceOfferID.String)
		if parseErr != nil || value == uuid.Nil {
			return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
		}
		profile.SourceOfferID = &value
	}
	if commandID.Valid {
		value, parseErr := uuid.Parse(commandID.String)
		if parseErr != nil || value == uuid.Nil {
			return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
		}
		profile.CommandID = &value
	}
	traits, err := loadRewardProfileTraits(ctx, executor, profile.ID)
	if err != nil {
		return entity.UserRewardProfileRevision{}, err
	}
	profile.Traits = traits
	if !profile.Valid() {
		return entity.UserRewardProfileRevision{}, entity.ErrInvalidRewardProfile
	}
	return profile, nil
}

func loadRewardProfileTraits(
	ctx context.Context,
	executor rewardProfileExecutor,
	profileID uuid.UUID,
) ([]entity.RewardProfileTrait, error) {
	rows, err := executor.QueryContext(ctx, `
		SELECT p.trait_id, p.trait_code, p.trait_type, p.trait_name,
			p.trait_description, p.rarity_code, p.rarity_name, p.slot, p.slot_order,
			e.id, e.trait_id, e.revision, e.schema_version, e.effect,
			e.target_element_ids, e.checksum, e.effective_at
		FROM user_reward_profile_traits p
		JOIN trait_effect_revisions e ON e.id = p.trait_effect_revision_id
		WHERE p.profile_revision_id = $1
		ORDER BY CASE p.slot WHEN 'root_bone' THEN 0 ELSE 1 END, p.slot_order
		LIMIT $2`, profileID, maximumRewardProfileTraits+1)
	if err != nil {
		return nil, fmt.Errorf("load reward profile traits: %w", err)
	}
	defer func() { _ = rows.Close() }()
	traits := make([]entity.RewardProfileTrait, 0, maximumRewardProfileTraits)
	for rows.Next() {
		assignment, scanErr := scanRewardProfileTrait(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		traits = append(traits, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(traits) != maximumRewardProfileTraits {
		return nil, entity.ErrInvalidRewardProfile
	}
	return traits, nil
}

func scanRewardProfileTrait(rows *stdsql.Rows) (entity.RewardProfileTrait, error) {
	var assignment entity.RewardProfileTrait
	var effect entity.TraitEffectRevision
	var schemaVersion int
	var effectDocument, targetDocument, checksum []byte
	if err := rows.Scan(
		&assignment.TraitID,
		&assignment.TraitCode,
		&assignment.TraitType,
		&assignment.TraitName,
		&assignment.Description,
		&assignment.RarityCode,
		&assignment.RarityName,
		&assignment.Slot,
		&assignment.SlotOrder,
		&effect.ID,
		&effect.TraitID,
		&effect.Revision,
		&schemaVersion,
		&effectDocument,
		&targetDocument,
		&checksum,
		&effect.EffectiveAt,
	); err != nil {
		return entity.RewardProfileTrait{}, err
	}
	if err := decodeRewardProfileEffect(
		&effect, schemaVersion, effectDocument, targetDocument, checksum,
	); err != nil || effect.TraitID != assignment.TraitID {
		return entity.RewardProfileTrait{}, entity.ErrInvalidRewardProfile
	}
	assignment.EffectRevision = effect
	return assignment, nil
}

func decodeRewardProfileEffect(
	effect *entity.TraitEffectRevision,
	schemaVersion int,
	effectDocument []byte,
	targetDocument []byte,
	checksum []byte,
) error {
	if effect == nil || schemaVersion != entity.TraitEffectSchemaVersion || len(checksum) != 32 ||
		len(effectDocument) == 0 || len(effectDocument) > entity.MaximumTraitEffectBytes ||
		len(targetDocument) == 0 || len(targetDocument) > entity.MaximumTraitEffectBytes {
		return entity.ErrInvalidTraitEffect
	}
	if err := commonjson.UnmarshalStrictLimit(
		effectDocument, &effect.Effect, entity.MaximumTraitEffectBytes,
	); err != nil {
		return entity.ErrInvalidTraitEffect
	}
	if err := commonjson.UnmarshalStrictLimit(
		targetDocument, &effect.TargetElementIDs, entity.MaximumTraitEffectBytes,
	); err != nil {
		return entity.ErrInvalidTraitEffect
	}
	copy(effect.Checksum[:], checksum)
	effect.EffectiveAt = effect.EffectiveAt.UTC()
	if !effect.Valid() {
		return entity.ErrInvalidTraitEffect
	}
	return nil
}

func loadRewardProfileCandidates(
	ctx context.Context,
	executor rewardProfileExecutor,
	selection entity.RewardProfileSelection,
	lock bool,
) ([]entity.RewardProfileTrait, error) {
	selection, err := entity.NormalizeRewardProfileSelection(selection)
	if err != nil {
		return nil, entity.ErrInvalidRewardProfile
	}
	ids := append([]uuid.UUID{selection.RootTraitID}, selection.TalentTraitIDs...)
	placeholders := make([]string, len(ids))
	arguments := make([]any, 0, len(ids))
	for index, traitID := range ids {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		arguments = append(arguments, traitID)
	}
	statement := `
		SELECT t.id, t.code, t.type, t.name, t.description,
			r.code, r.name,
			e.id, e.trait_id, e.revision, e.schema_version, e.effect,
			e.target_element_ids, e.checksum, e.effective_at
		FROM traits t
		JOIN rarities r ON r.id = t.rarity_id AND r.deleted_at IS NULL
		JOIN LATERAL (
			SELECT current_effect.id, current_effect.trait_id, current_effect.revision,
				current_effect.schema_version, current_effect.effect,
				current_effect.target_element_ids, current_effect.checksum,
				current_effect.effective_at
			FROM trait_effect_revisions current_effect
			WHERE current_effect.trait_id = t.id
				AND current_effect.revision = t.version
			ORDER BY current_effect.effective_at DESC, current_effect.revision DESC, current_effect.id DESC
			LIMIT 1
		) e ON TRUE
		WHERE t.deleted_at IS NULL AND t.active = TRUE
			AND t.id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY t.id`
	if lock {
		statement += ` FOR SHARE OF t`
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, fmt.Errorf("load reward profile candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	byID := make(map[uuid.UUID]entity.RewardProfileTrait, len(ids))
	for rows.Next() {
		var assignment entity.RewardProfileTrait
		var effect entity.TraitEffectRevision
		var schemaVersion int
		var effectDocument, targetDocument, checksum []byte
		if err := rows.Scan(
			&assignment.TraitID,
			&assignment.TraitCode,
			&assignment.TraitType,
			&assignment.TraitName,
			&assignment.Description,
			&assignment.RarityCode,
			&assignment.RarityName,
			&effect.ID,
			&effect.TraitID,
			&effect.Revision,
			&schemaVersion,
			&effectDocument,
			&targetDocument,
			&checksum,
			&effect.EffectiveAt,
		); err != nil {
			return nil, err
		}
		if err := decodeRewardProfileEffect(
			&effect, schemaVersion, effectDocument, targetDocument, checksum,
		); err != nil || effect.TraitID != assignment.TraitID {
			return nil, entity.ErrInvalidRewardProfile
		}
		assignment.EffectRevision = effect
		if _, duplicate := byID[assignment.TraitID]; duplicate {
			return nil, entity.ErrInvalidRewardProfile
		}
		byID[assignment.TraitID] = assignment
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(byID) != len(ids) {
		return nil, entity.ErrInvalidRewardProfile
	}
	traits := make([]entity.RewardProfileTrait, 0, len(ids))
	root := byID[selection.RootTraitID]
	if root.TraitType != entity.TraitTypeRootBone {
		return nil, entity.ErrInvalidRewardProfile
	}
	root.Slot = entity.RewardProfileSlotRootBone
	root.SlotOrder = 0
	traits = append(traits, root)
	for index, traitID := range selection.TalentTraitIDs {
		talent := byID[traitID]
		if talent.TraitType != entity.TraitTypeTalent {
			return nil, entity.ErrInvalidRewardProfile
		}
		talent.Slot = entity.RewardProfileSlotTalent
		talent.SlotOrder = index
		traits = append(traits, talent)
	}
	return traits, nil
}

func rewardProfileTraitsFromOffer(
	offer entity.TraitOffer,
	selection entity.RewardProfileSelection,
) ([]entity.RewardProfileTrait, error) {
	if !offer.Valid() || selection.RootTraitID != offer.Root.ID {
		return nil, entity.ErrInvalidRewardProfile
	}
	byID := make(map[uuid.UUID]entity.CatalogTrait, len(offer.Talents))
	for _, talent := range offer.Talents {
		byID[talent.ID] = talent
	}
	fromCatalog := func(current entity.CatalogTrait, slot string, order int) entity.RewardProfileTrait {
		return entity.RewardProfileTrait{
			TraitID:        current.ID,
			TraitCode:      current.Code,
			TraitType:      current.Type,
			TraitName:      current.Name,
			Description:    current.Description,
			RarityCode:     current.RarityCode,
			RarityName:     current.RarityName,
			Slot:           slot,
			SlotOrder:      order,
			EffectRevision: current.EffectRevision,
		}
	}
	traits := []entity.RewardProfileTrait{
		fromCatalog(offer.Root, entity.RewardProfileSlotRootBone, 0),
	}
	seen := map[uuid.UUID]struct{}{offer.Root.ID: {}}
	for index, traitID := range selection.TalentTraitIDs {
		talent, found := byID[traitID]
		if !found {
			return nil, entity.ErrInvalidRewardProfile
		}
		if _, duplicate := seen[traitID]; duplicate {
			return nil, entity.ErrInvalidRewardProfile
		}
		seen[traitID] = struct{}{}
		traits = append(traits, fromCatalog(talent, entity.RewardProfileSlotTalent, index))
	}
	if len(traits) != maximumRewardProfileTraits {
		return nil, entity.ErrInvalidRewardProfile
	}
	return traits, nil
}
