package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	RewardProfileSlotRootBone = "root_bone"
	RewardProfileSlotTalent   = "talent"

	RewardProfileSourceOnboarding     = "onboarding"
	RewardProfileSourceAdministrative = "administrative"

	rewardProfileChecksumDomain = "judgify.cultivation.user-reward-profile.v1"
)

var (
	ErrInvalidRewardProfile  = errors.New("cultivation: invalid reward profile")
	ErrRewardProfileConflict = errors.New("cultivation: reward profile conflict")
	ErrRewardProfileNotFound = errors.New("cultivation: reward profile not found")
)

type RewardProfileSelection struct {
	RootTraitID    uuid.UUID   `json:"root_trait_id"`
	TalentTraitIDs []uuid.UUID `json:"talent_trait_ids"`
}

type RewardProfileAssignmentPreview struct {
	UserID          uuid.UUID            `json:"user_id"`
	CurrentRevision int64                `json:"current_revision"`
	NextRevision    int64                `json:"next_revision"`
	Traits          []RewardProfileTrait `json:"traits"`
	Checksum        [sha256.Size]byte    `json:"-"`
}

type RewardProfileAssignmentCommand struct {
	ID               uuid.UUID              `json:"id"`
	UserID           uuid.UUID              `json:"user_id"`
	ExpectedRevision int64                  `json:"expected_revision"`
	Selection        RewardProfileSelection `json:"selection"`
	PreviewChecksum  [sha256.Size]byte      `json:"-"`
	Reason           string                 `json:"reason"`
}

type RewardProfileAssignmentResult struct {
	Profile  UserRewardProfileRevision
	Replayed bool
}

// RewardProfileTrait is one normalized immutable assignment. Display fields
// are snapshots for audit/UI; reward replay trusts only identity, slot, and the
// checksum-valid effect revision.
type RewardProfileTrait struct {
	TraitID        uuid.UUID           `json:"trait_id"`
	TraitCode      string              `json:"trait_code"`
	TraitType      string              `json:"trait_type"`
	TraitName      string              `json:"trait_name"`
	Description    string              `json:"description,omitempty"`
	RarityCode     string              `json:"rarity_code"`
	RarityName     string              `json:"rarity_name"`
	Slot           string              `json:"slot"`
	SlotOrder      int                 `json:"slot_order"`
	EffectRevision TraitEffectRevision `json:"effect_revision"`
}

type UserRewardProfileRevision struct {
	ID                 uuid.UUID            `json:"id"`
	UserID             uuid.UUID            `json:"user_id"`
	Revision           int64                `json:"revision"`
	AssignmentChecksum [sha256.Size]byte    `json:"-"`
	Source             string               `json:"source"`
	SourceOfferID      *uuid.UUID           `json:"source_offer_id,omitempty"`
	CommandID          *uuid.UUID           `json:"command_id,omitempty"`
	RequestChecksum    [sha256.Size]byte    `json:"-"`
	EffectiveAt        time.Time            `json:"effective_at"`
	Traits             []RewardProfileTrait `json:"traits"`
}

func NormalizeRewardProfileSelection(value RewardProfileSelection) (RewardProfileSelection, error) {
	normalized := RewardProfileSelection{
		RootTraitID:    value.RootTraitID,
		TalentTraitIDs: append([]uuid.UUID(nil), value.TalentTraitIDs...),
	}
	if normalized.RootTraitID == uuid.Nil || len(normalized.TalentTraitIDs) != 3 {
		return RewardProfileSelection{}, ErrInvalidRewardProfile
	}
	seen := map[uuid.UUID]struct{}{normalized.RootTraitID: {}}
	for _, traitID := range normalized.TalentTraitIDs {
		if traitID == uuid.Nil {
			return RewardProfileSelection{}, ErrInvalidRewardProfile
		}
		if _, duplicate := seen[traitID]; duplicate {
			return RewardProfileSelection{}, ErrInvalidRewardProfile
		}
		seen[traitID] = struct{}{}
	}
	return normalized, nil
}

func NormalizeRewardProfileAssignmentCommand(
	command RewardProfileAssignmentCommand,
) (RewardProfileAssignmentCommand, error) {
	selection, err := NormalizeRewardProfileSelection(command.Selection)
	command.Reason = strings.TrimSpace(command.Reason)
	if err != nil || command.ID == uuid.Nil || command.UserID == uuid.Nil ||
		command.ExpectedRevision < 1 || command.PreviewChecksum == ([sha256.Size]byte{}) ||
		!validRewardRuleReason(command.Reason) {
		return RewardProfileAssignmentCommand{}, ErrInvalidRewardProfile
	}
	command.Selection = selection
	return command, nil
}

func NewRewardProfileAssignmentPreview(
	current UserRewardProfileRevision,
	traits []RewardProfileTrait,
) (RewardProfileAssignmentPreview, error) {
	if !current.Valid() {
		return RewardProfileAssignmentPreview{}, ErrInvalidRewardProfile
	}
	previewEffectiveAt := time.Unix(1, 0).UTC()
	for _, assignment := range traits {
		if assignment.EffectRevision.EffectiveAt.After(previewEffectiveAt) {
			previewEffectiveAt = assignment.EffectRevision.EffectiveAt.UTC()
		}
	}
	normalized := UserRewardProfileRevision{
		ID:              uuid.NewSHA1(uuid.NameSpaceURL, []byte("judgify/reward-profile-preview")),
		UserID:          current.UserID,
		Revision:        current.Revision + 1,
		Source:          RewardProfileSourceAdministrative,
		CommandID:       rewardProfileUUIDPointer(uuid.NewSHA1(uuid.NameSpaceURL, []byte("judgify/reward-profile-preview-command"))),
		RequestChecksum: sha256.Sum256([]byte("judgify/reward-profile-preview-request")),
		EffectiveAt:     previewEffectiveAt,
		Traits:          append([]RewardProfileTrait(nil), traits...),
	}
	if !normalized.validShape() {
		return RewardProfileAssignmentPreview{}, ErrInvalidRewardProfile
	}
	preview := RewardProfileAssignmentPreview{
		UserID:          current.UserID,
		CurrentRevision: current.Revision,
		NextRevision:    current.Revision + 1,
		Traits:          append([]RewardProfileTrait(nil), traits...),
	}
	preview.Checksum = RewardProfileAssignmentPreviewChecksum(preview)
	if preview.Checksum == ([sha256.Size]byte{}) {
		return RewardProfileAssignmentPreview{}, ErrInvalidRewardProfile
	}
	return preview, nil
}

func RewardProfileAssignmentPreviewChecksum(
	preview RewardProfileAssignmentPreview,
) [sha256.Size]byte {
	type assignment struct {
		TraitID          uuid.UUID `json:"trait_id"`
		Slot             string    `json:"slot"`
		SlotOrder        int       `json:"slot_order"`
		EffectRevisionID uuid.UUID `json:"effect_revision_id"`
		EffectRevision   int64     `json:"effect_revision"`
		EffectChecksum   string    `json:"effect_checksum"`
	}
	assignments := make([]assignment, len(preview.Traits))
	for index, current := range preview.Traits {
		assignments[index] = assignment{
			TraitID:          current.TraitID,
			Slot:             current.Slot,
			SlotOrder:        current.SlotOrder,
			EffectRevisionID: current.EffectRevision.ID,
			EffectRevision:   current.EffectRevision.Revision,
			EffectChecksum:   checksumHex(current.EffectRevision.Checksum),
		}
	}
	slices.SortFunc(assignments, func(left, right assignment) int {
		if left.Slot != right.Slot {
			if left.Slot == RewardProfileSlotRootBone {
				return -1
			}
			return 1
		}
		return left.SlotOrder - right.SlotOrder
	})
	document := struct {
		Schema          string       `json:"schema"`
		UserID          uuid.UUID    `json:"user_id"`
		CurrentRevision int64        `json:"current_revision"`
		NextRevision    int64        `json:"next_revision"`
		Assignments     []assignment `json:"assignments"`
	}{
		Schema:          "judgify.cultivation.reward-profile-assignment-preview.v1",
		UserID:          preview.UserID,
		CurrentRevision: preview.CurrentRevision,
		NextRevision:    preview.NextRevision,
		Assignments:     assignments,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func RewardProfileAssignmentRequestChecksum(
	command RewardProfileAssignmentCommand,
) [sha256.Size]byte {
	document := struct {
		Schema           string                 `json:"schema"`
		ID               uuid.UUID              `json:"id"`
		UserID           uuid.UUID              `json:"user_id"`
		ExpectedRevision int64                  `json:"expected_revision"`
		Selection        RewardProfileSelection `json:"selection"`
		PreviewChecksum  string                 `json:"preview_checksum"`
		Reason           string                 `json:"reason"`
	}{
		Schema:           "judgify.cultivation.reward-profile-assignment.v1",
		ID:               command.ID,
		UserID:           command.UserID,
		ExpectedRevision: command.ExpectedRevision,
		Selection:        command.Selection,
		PreviewChecksum:  checksumHex(command.PreviewChecksum),
		Reason:           command.Reason,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func RewardProfileOnboardingRequestChecksum(
	userID uuid.UUID,
	offer TraitOffer,
	selection RewardProfileSelection,
) [sha256.Size]byte {
	document := struct {
		Schema        string                 `json:"schema"`
		UserID        uuid.UUID              `json:"user_id"`
		OfferID       uuid.UUID              `json:"offer_id"`
		OfferChecksum string                 `json:"offer_checksum"`
		Selection     RewardProfileSelection `json:"selection"`
	}{
		Schema:        "judgify.cultivation.reward-profile-onboarding.v1",
		UserID:        userID,
		OfferID:       offer.ID,
		OfferChecksum: checksumHex(offer.OfferChecksum),
		Selection:     selection,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func NewUserRewardProfileRevision(
	id uuid.UUID,
	userID uuid.UUID,
	revision int64,
	source string,
	sourceOfferID *uuid.UUID,
	commandID *uuid.UUID,
	requestChecksum [sha256.Size]byte,
	effectiveAt time.Time,
	traits []RewardProfileTrait,
) (UserRewardProfileRevision, error) {
	profile := UserRewardProfileRevision{
		ID:              id,
		UserID:          userID,
		Revision:        revision,
		Source:          source,
		SourceOfferID:   cloneRewardProfileUUIDPointer(sourceOfferID),
		CommandID:       cloneRewardProfileUUIDPointer(commandID),
		RequestChecksum: requestChecksum,
		EffectiveAt:     effectiveAt.UTC(),
		Traits:          append([]RewardProfileTrait(nil), traits...),
	}
	checksum, err := profile.CalculateChecksum()
	if err != nil {
		return UserRewardProfileRevision{}, err
	}
	profile.AssignmentChecksum = checksum
	if !profile.Valid() {
		return UserRewardProfileRevision{}, ErrInvalidRewardProfile
	}
	return profile, nil
}

func (profile UserRewardProfileRevision) Valid() bool {
	if !profile.validShape() || profile.AssignmentChecksum == ([sha256.Size]byte{}) {
		return false
	}
	checksum, err := profile.CalculateChecksum()
	return err == nil && subtle.ConstantTimeCompare(
		checksum[:], profile.AssignmentChecksum[:],
	) == 1
}

func (profile UserRewardProfileRevision) validShape() bool {
	if profile.ID == uuid.Nil || profile.UserID == uuid.Nil || profile.Revision < 1 ||
		profile.RequestChecksum == ([sha256.Size]byte{}) || profile.EffectiveAt.IsZero() ||
		len(profile.Traits) != 4 {
		return false
	}
	switch profile.Source {
	case RewardProfileSourceOnboarding:
		if profile.SourceOfferID == nil || *profile.SourceOfferID == uuid.Nil || profile.CommandID != nil {
			return false
		}
	case RewardProfileSourceAdministrative:
		if profile.CommandID == nil || *profile.CommandID == uuid.Nil || profile.SourceOfferID != nil {
			return false
		}
	default:
		return false
	}
	seenTraits := make(map[uuid.UUID]struct{}, len(profile.Traits))
	rootCount := 0
	talentOrders := make([]int, 0, 3)
	for _, assignment := range profile.Traits {
		if assignment.TraitID == uuid.Nil || assignment.EffectRevision.TraitID != assignment.TraitID ||
			!assignment.EffectRevision.Valid() ||
			assignment.EffectRevision.EffectiveAt.After(profile.EffectiveAt) {
			return false
		}
		if _, duplicate := seenTraits[assignment.TraitID]; duplicate {
			return false
		}
		seenTraits[assignment.TraitID] = struct{}{}
		switch assignment.Slot {
		case RewardProfileSlotRootBone:
			if assignment.TraitType != TraitTypeRootBone || assignment.SlotOrder != 0 {
				return false
			}
			rootCount++
		case RewardProfileSlotTalent:
			if assignment.TraitType != TraitTypeTalent || assignment.SlotOrder < 0 || assignment.SlotOrder > 2 {
				return false
			}
			talentOrders = append(talentOrders, assignment.SlotOrder)
		default:
			return false
		}
	}
	slices.Sort(talentOrders)
	return rootCount == 1 && slices.Equal(talentOrders, []int{0, 1, 2})
}

func (profile UserRewardProfileRevision) CalculateChecksum() ([sha256.Size]byte, error) {
	if !profile.validShape() {
		return [sha256.Size]byte{}, ErrInvalidRewardProfile
	}
	type assignment struct {
		TraitID          uuid.UUID `json:"trait_id"`
		TraitType        string    `json:"trait_type"`
		Slot             string    `json:"slot"`
		SlotOrder        int       `json:"slot_order"`
		EffectRevisionID uuid.UUID `json:"effect_revision_id"`
		EffectRevision   int64     `json:"effect_revision"`
		EffectChecksum   string    `json:"effect_checksum"`
	}
	assignments := make([]assignment, len(profile.Traits))
	for index, current := range profile.Traits {
		assignments[index] = assignment{
			TraitID: current.TraitID, TraitType: current.TraitType,
			Slot: current.Slot, SlotOrder: current.SlotOrder,
			EffectRevisionID: current.EffectRevision.ID,
			EffectRevision:   current.EffectRevision.Revision,
			EffectChecksum:   checksumHex(current.EffectRevision.Checksum),
		}
	}
	slices.SortFunc(assignments, func(left, right assignment) int {
		if left.Slot != right.Slot {
			if left.Slot == RewardProfileSlotRootBone {
				return -1
			}
			return 1
		}
		return left.SlotOrder - right.SlotOrder
	})
	document := struct {
		Domain          string       `json:"domain"`
		ID              uuid.UUID    `json:"id"`
		UserID          uuid.UUID    `json:"user_id"`
		Revision        int64        `json:"revision"`
		Source          string       `json:"source"`
		SourceOfferID   *uuid.UUID   `json:"source_offer_id,omitempty"`
		CommandID       *uuid.UUID   `json:"command_id,omitempty"`
		RequestChecksum string       `json:"request_checksum"`
		EffectiveAt     time.Time    `json:"effective_at"`
		Assignments     []assignment `json:"assignments"`
	}{
		Domain: rewardProfileChecksumDomain, ID: profile.ID, UserID: profile.UserID,
		Revision: profile.Revision, Source: profile.Source,
		SourceOfferID: profile.SourceOfferID, CommandID: profile.CommandID,
		RequestChecksum: checksumHex(profile.RequestChecksum),
		EffectiveAt:     profile.EffectiveAt.UTC(), Assignments: assignments,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}, ErrInvalidRewardProfile
	}
	return sha256.Sum256(encoded), nil
}

func rewardProfileUUIDPointer(value uuid.UUID) *uuid.UUID {
	return &value
}

func cloneRewardProfileUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
