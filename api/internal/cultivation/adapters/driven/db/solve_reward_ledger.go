package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	stdsql "database/sql"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"

	cultivationcontract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	cultivationentity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
)

const (
	progressionSolveSourceType    = "problem_solve"
	maximumSignedBigint           = int64(1<<63 - 1)
	maximumProgressionCount       = int64(1<<31 - 1)
	maximumProgressionVectorBytes = 64 << 10
)

type progressionElementDelta struct {
	ElementID uuid.UUID `json:"element_id"`
	ExpDelta  int64     `json:"exp_delta"`
}

type progressionDifficultyDelta struct {
	DifficultyID uuid.UUID `json:"difficulty_id"`
	SolvedDelta  int64     `json:"solved_delta"`
}

type progressionTagDelta struct {
	TagID       uuid.UUID `json:"tag_id"`
	SolvedDelta int64     `json:"solved_delta"`
}

type progressionRewardVector struct {
	TotalExperience         int64
	Elements                []progressionElementDelta
	Difficulties            []progressionDifficultyDelta
	Tags                    []progressionTagDelta
	SourceChecksum          [sha256Size]byte
	RewardRuleRevisionID    uuid.UUID
	RewardRuleVersion       int64
	RewardProfileRevisionID uuid.UUID
	RewardProfileVersion    int64
	Provenance              stdjson.RawMessage
}

type progressionBalance struct {
	Before  int64
	After   int64
	Version int64
}

type solveRewardProvenanceReward struct {
	DifficultyID       uuid.UUID                             `json:"difficulty_id"`
	DifficultyRevision int64                                 `json:"difficulty_revision"`
	BaseExperience     int64                                 `json:"base_exp"`
	Tags               problementity.TagMappingSnapshot      `json:"tags"`
	Elements           *problementity.ElementMappingSnapshot `json:"elements"`
}

type solveRewardProvenanceTrait struct {
	TraitID           uuid.UUID                         `json:"trait_id"`
	TraitCode         string                            `json:"trait_code"`
	TraitType         string                            `json:"trait_type"`
	Slot              string                            `json:"slot"`
	SlotOrder         int                               `json:"slot_order"`
	EffectRevisionID  uuid.UUID                         `json:"effect_revision_id"`
	EffectRevision    int64                             `json:"effect_revision"`
	EffectChecksum    string                            `json:"effect_checksum"`
	Effect            cultivationentity.TraitEffectSpec `json:"effect"`
	TargetElementIDs  []uuid.UUID                       `json:"target_element_ids,omitempty"`
	EffectEffectiveAt time.Time                         `json:"effect_effective_at"`
}

type storedProgressionGrant struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	SourceType         string
	SourceAggregateID  uuid.UUID
	SourceVersion      int64
	SourceTransitionID uuid.UUID
	SourceEventID      uuid.UUID
	Vector             progressionRewardVector
}

func (projector *SolveRewardProjector) applyContiguousSolveReward(
	ctx context.Context,
	tx *stdsql.Tx,
	cursor solveRewardCursor,
	transition cultivationcontract.SolveRewardTransition,
	now time.Time,
) (solveRewardApplication, error) {
	switch transition.Kind {
	case cultivationcontract.SolveRewardGrant:
		if cursor.ActiveLedgerEntryID != nil || transition.To.Reward == nil {
			return solveRewardApplication{}, permanentSolveReward(ErrSolveRewardConflict)
		}
		rule, err := loadEffectiveRewardRule(ctx, tx, transition.OccurredAt)
		if err != nil {
			return solveRewardApplication{}, permanentSolveReward(err)
		}
		profile, err := loadEffectiveRewardProfile(
			ctx, tx, transition.UserID, transition.OccurredAt,
		)
		if err != nil {
			return solveRewardApplication{}, permanentSolveReward(err)
		}
		vector, err := buildSolveRewardGrantVector(transition, rule, profile)
		if err != nil {
			return solveRewardApplication{}, err
		}
		balance, err := lockProgressionBalance(ctx, tx, transition.UserID, vector.TotalExperience)
		if err != nil {
			return solveRewardApplication{}, err
		}
		ledgerID, err := projector.insertProgressionLedgerEntry(
			ctx, tx, transition, "grant", nil, vector, balance, now,
		)
		if err != nil {
			return solveRewardApplication{}, err
		}
		if err := projector.applyProgressionRewardVector(ctx, tx, transition.UserID, vector, balance, now); err != nil {
			return solveRewardApplication{}, err
		}
		return solveRewardApplication{
			Outcome: ports.SolveRewardGranted, LedgerEntryID: &ledgerID,
			TotalExperienceDelta: vector.TotalExperience,
			ElementDeltaCount:    len(vector.Elements),
		}, nil

	case cultivationcontract.SolveRewardReversal:
		if cursor.ActiveLedgerEntryID == nil {
			return solveRewardApplication{}, permanentSolveReward(ErrSolveRewardConflict)
		}
		grant, err := loadActiveProgressionGrant(ctx, tx, *cursor.ActiveLedgerEntryID, cursor, transition)
		if err != nil {
			return solveRewardApplication{}, err
		}
		vector, err := buildSolveRewardReversalVector(transition, grant)
		if err != nil {
			return solveRewardApplication{}, err
		}
		balance, err := lockProgressionBalance(ctx, tx, transition.UserID, vector.TotalExperience)
		if err != nil {
			return solveRewardApplication{}, err
		}
		ledgerID, err := projector.insertProgressionLedgerEntry(
			ctx, tx, transition, "reversal", &grant.ID, vector, balance, now,
		)
		if err != nil {
			return solveRewardApplication{}, err
		}
		if err := projector.applyProgressionRewardVector(ctx, tx, transition.UserID, vector, balance, now); err != nil {
			return solveRewardApplication{}, err
		}
		return solveRewardApplication{
			Outcome: ports.SolveRewardReversed, LedgerEntryID: &ledgerID,
			TotalExperienceDelta: vector.TotalExperience,
			ElementDeltaCount:    len(vector.Elements),
		}, nil

	case cultivationcontract.SolveRewardPointerMove:
		if cursor.ActiveLedgerEntryID == nil {
			return solveRewardApplication{}, permanentSolveReward(ErrSolveRewardConflict)
		}
		if _, err := loadActiveProgressionGrant(
			ctx, tx, *cursor.ActiveLedgerEntryID, cursor, transition,
		); err != nil {
			return solveRewardApplication{}, err
		}
		return solveRewardApplication{Outcome: ports.SolveRewardPointerMoved}, nil

	default:
		return solveRewardApplication{}, permanentSolveReward(ErrSolveRewardConflict)
	}
}

func buildSolveRewardGrantVector(
	transition cultivationcontract.SolveRewardTransition,
	rule cultivationentity.RewardRuleRevision,
	profile cultivationentity.UserRewardProfileRevision,
) (progressionRewardVector, error) {
	if transition.Kind != cultivationcontract.SolveRewardGrant || transition.To.Reward == nil {
		return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	reward := *transition.To.Reward
	if reward.Validate() != nil || reward.BaseExperience < 0 || !rule.Valid() ||
		!profile.Valid() || profile.UserID != transition.UserID ||
		profile.EffectiveAt.After(transition.OccurredAt) {
		return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	elementInputs := make([]cultivationentity.RewardElementInput, 0)
	if reward.Elements != nil {
		elementInputs = make([]cultivationentity.RewardElementInput, len(reward.Elements.Items))
		for index, element := range reward.Elements.Items {
			elementInputs[index] = cultivationentity.RewardElementInput{
				ElementID:         element.ElementID,
				WeightBasisPoints: element.WeightBasisPoints,
			}
		}
	}
	calculated, err := cultivationentity.CalculateSolveReward(
		reward.BaseExperience, rule, profile, elementInputs,
	)
	if err != nil || calculated.TotalExperience <= 0 {
		return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	vector := progressionRewardVector{
		TotalExperience: calculated.TotalExperience,
		Elements:        make([]progressionElementDelta, 0),
		Difficulties: []progressionDifficultyDelta{{
			DifficultyID: reward.DifficultyID,
			SolvedDelta:  1,
		}},
		RewardRuleRevisionID:    rule.ID,
		RewardRuleVersion:       rule.Revision,
		RewardProfileRevisionID: profile.ID,
		RewardProfileVersion:    profile.Revision,
	}
	if len(calculated.Elements) > 0 {
		vector.Elements = make([]progressionElementDelta, 0, len(calculated.Elements))
		for _, element := range calculated.Elements {
			if element.Experience == 0 {
				continue
			}
			vector.Elements = append(vector.Elements, progressionElementDelta{
				ElementID: element.ElementID,
				ExpDelta:  element.Experience,
			})
		}
	}
	vector.Tags = make([]progressionTagDelta, len(reward.Tags.Items))
	for index, tag := range reward.Tags.Items {
		vector.Tags[index] = progressionTagDelta{TagID: tag.TagID, SolvedDelta: 1}
	}
	provenance, err := solveRewardLedgerProvenance(transition, rule, profile, nil, [sha256Size]byte{})
	if err != nil {
		return progressionRewardVector{}, err
	}
	vector.Provenance = provenance
	vector.SourceChecksum = solveRewardSourceChecksum(
		reward.Checksum, rule.Checksum, profile.AssignmentChecksum, vector,
	)
	return vector, nil
}

func buildSolveRewardReversalVector(
	transition cultivationcontract.SolveRewardTransition,
	grant storedProgressionGrant,
) (progressionRewardVector, error) {
	if transition.Kind != cultivationcontract.SolveRewardReversal ||
		grant.ID == uuid.Nil || grant.Vector.TotalExperience < 0 {
		return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardConflict)
	}
	vector := progressionRewardVector{
		TotalExperience:         -grant.Vector.TotalExperience,
		Elements:                make([]progressionElementDelta, len(grant.Vector.Elements)),
		Difficulties:            make([]progressionDifficultyDelta, len(grant.Vector.Difficulties)),
		Tags:                    make([]progressionTagDelta, len(grant.Vector.Tags)),
		SourceChecksum:          grant.Vector.SourceChecksum,
		RewardRuleRevisionID:    grant.Vector.RewardRuleRevisionID,
		RewardRuleVersion:       grant.Vector.RewardRuleVersion,
		RewardProfileRevisionID: grant.Vector.RewardProfileRevisionID,
		RewardProfileVersion:    grant.Vector.RewardProfileVersion,
	}
	for index, element := range grant.Vector.Elements {
		if element.ExpDelta < 0 {
			return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardInvariant)
		}
		vector.Elements[index] = progressionElementDelta{
			ElementID: element.ElementID,
			ExpDelta:  -element.ExpDelta,
		}
	}
	for index, difficulty := range grant.Vector.Difficulties {
		if difficulty.SolvedDelta != 1 {
			return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardInvariant)
		}
		vector.Difficulties[index] = progressionDifficultyDelta{
			DifficultyID: difficulty.DifficultyID,
			SolvedDelta:  -1,
		}
	}
	for index, tag := range grant.Vector.Tags {
		if tag.SolvedDelta != 1 {
			return progressionRewardVector{}, permanentSolveReward(ErrSolveRewardInvariant)
		}
		vector.Tags[index] = progressionTagDelta{TagID: tag.TagID, SolvedDelta: -1}
	}
	rule := cultivationentity.RewardRuleRevision{
		ID:       grant.Vector.RewardRuleRevisionID,
		Revision: grant.Vector.RewardRuleVersion,
	}
	profile := cultivationentity.UserRewardProfileRevision{
		ID:       grant.Vector.RewardProfileRevisionID,
		Revision: grant.Vector.RewardProfileVersion,
	}
	provenance, err := solveRewardLedgerProvenance(
		transition, rule, profile, &grant.ID, grant.Vector.SourceChecksum,
	)
	if err != nil {
		return progressionRewardVector{}, err
	}
	vector.Provenance = provenance
	return vector, nil
}

func solveRewardSourceChecksum(
	rewardChecksum [sha256.Size]byte,
	ruleChecksum [sha256.Size]byte,
	profileChecksum [sha256.Size]byte,
	vector progressionRewardVector,
) [sha256.Size]byte {
	document := struct {
		Schema            string                       `json:"schema"`
		RewardChecksum    string                       `json:"reward_checksum"`
		RuleChecksum      string                       `json:"rule_checksum"`
		RuleRevisionID    uuid.UUID                    `json:"rule_revision_id"`
		RuleVersion       int64                        `json:"rule_version"`
		ProfileChecksum   string                       `json:"profile_checksum"`
		ProfileRevisionID uuid.UUID                    `json:"profile_revision_id"`
		ProfileVersion    int64                        `json:"profile_version"`
		TotalExperience   int64                        `json:"total_exp"`
		Elements          []progressionElementDelta    `json:"elements"`
		Difficulties      []progressionDifficultyDelta `json:"difficulties"`
		Tags              []progressionTagDelta        `json:"tags"`
	}{
		Schema:            "judgify.cultivation.progression-source.solve.v2",
		RewardChecksum:    fmt.Sprintf("%x", rewardChecksum),
		RuleChecksum:      fmt.Sprintf("%x", ruleChecksum),
		RuleRevisionID:    vector.RewardRuleRevisionID,
		RuleVersion:       vector.RewardRuleVersion,
		ProfileChecksum:   fmt.Sprintf("%x", profileChecksum),
		ProfileRevisionID: vector.RewardProfileRevisionID,
		ProfileVersion:    vector.RewardProfileVersion,
		TotalExperience:   vector.TotalExperience,
		Elements:          vector.Elements,
		Difficulties:      vector.Difficulties,
		Tags:              vector.Tags,
	}
	encoded, err := stdjson.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func solveRewardLedgerProvenance(
	transition cultivationcontract.SolveRewardTransition,
	rule cultivationentity.RewardRuleRevision,
	profile cultivationentity.UserRewardProfileRevision,
	reversedEntryID *uuid.UUID,
	reversedSourceChecksum [sha256Size]byte,
) (stdjson.RawMessage, error) {
	state := transition.To
	if transition.Kind == cultivationcontract.SolveRewardReversal {
		state = transition.From
	}
	if state.Reward == nil || state.SubmissionID == nil || state.JudgementID == nil ||
		state.Generation == nil || state.ProblemRevisionID == nil ||
		state.RewardGrantSourceID == nil {
		return nil, permanentSolveReward(ErrSolveRewardInvariant)
	}
	document := struct {
		Schema                  string                       `json:"schema"`
		ProblemID               uuid.UUID                    `json:"problem_id"`
		TransitionID            uuid.UUID                    `json:"transition_id"`
		EventID                 uuid.UUID                    `json:"event_id"`
		AggregateVersion        int64                        `json:"aggregate_version"`
		SubmissionID            uuid.UUID                    `json:"submission_id"`
		JudgementID             uuid.UUID                    `json:"judgement_id"`
		Generation              int64                        `json:"generation"`
		ProblemRevisionID       uuid.UUID                    `json:"problem_revision_id"`
		RewardGrantSourceID     uuid.UUID                    `json:"reward_grant_source_id"`
		RewardRuleRevisionID    uuid.UUID                    `json:"reward_rule_revision_id"`
		RewardRuleVersion       int64                        `json:"reward_rule_version"`
		RewardRuleChecksum      string                       `json:"reward_rule_checksum,omitempty"`
		RewardProfileRevisionID uuid.UUID                    `json:"reward_profile_revision_id"`
		RewardProfileVersion    int64                        `json:"reward_profile_version"`
		RewardProfileChecksum   string                       `json:"reward_profile_checksum,omitempty"`
		RewardProfileTraits     []solveRewardProvenanceTrait `json:"reward_profile_traits,omitempty"`
		Reward                  *solveRewardProvenanceReward `json:"reward,omitempty"`
		ReversedEntryID         *uuid.UUID                   `json:"reversed_entry_id,omitempty"`
		ReversedSourceChecksum  string                       `json:"reversed_source_checksum,omitempty"`
	}{
		Schema:    "judgify.cultivation.progression-ledger.solve.v2",
		ProblemID: transition.ProblemID, TransitionID: transition.TransitionID,
		EventID: transition.EventID, AggregateVersion: transition.To.Version,
		SubmissionID: *state.SubmissionID, JudgementID: *state.JudgementID,
		Generation: *state.Generation, ProblemRevisionID: *state.ProblemRevisionID,
		RewardGrantSourceID:     *state.RewardGrantSourceID,
		RewardRuleRevisionID:    rule.ID,
		RewardRuleVersion:       rule.Revision,
		RewardProfileRevisionID: profile.ID,
		RewardProfileVersion:    profile.Revision,
		ReversedEntryID:         reversedEntryID,
	}
	if reversedEntryID == nil {
		if !profile.Valid() {
			return nil, permanentSolveReward(ErrSolveRewardInvariant)
		}
		reward := state.Reward
		document.Reward = &solveRewardProvenanceReward{
			DifficultyID:       reward.DifficultyID,
			DifficultyRevision: reward.DifficultyRevision,
			BaseExperience:     reward.BaseExperience,
			Tags:               reward.Tags,
			Elements:           reward.Elements,
		}
		document.RewardRuleChecksum = fmt.Sprintf("%x", rule.Checksum)
		document.RewardProfileChecksum = fmt.Sprintf("%x", profile.AssignmentChecksum)
		document.RewardProfileTraits = make(
			[]solveRewardProvenanceTrait, len(profile.Traits),
		)
		for index, assignment := range profile.Traits {
			effect := assignment.EffectRevision
			document.RewardProfileTraits[index] = solveRewardProvenanceTrait{
				TraitID:           assignment.TraitID,
				TraitCode:         assignment.TraitCode,
				TraitType:         assignment.TraitType,
				Slot:              assignment.Slot,
				SlotOrder:         assignment.SlotOrder,
				EffectRevisionID:  effect.ID,
				EffectRevision:    effect.Revision,
				EffectChecksum:    fmt.Sprintf("%x", effect.Checksum),
				Effect:            effect.Effect,
				TargetElementIDs:  append([]uuid.UUID(nil), effect.TargetElementIDs...),
				EffectEffectiveAt: effect.EffectiveAt,
			}
		}
	} else {
		if profile.ID == uuid.Nil || profile.Revision < 1 ||
			reversedSourceChecksum == ([sha256Size]byte{}) {
			return nil, permanentSolveReward(ErrSolveRewardInvariant)
		}
		document.ReversedSourceChecksum = fmt.Sprintf("%x", reversedSourceChecksum)
	}
	encoded, err := stdjson.Marshal(document)
	if err != nil || len(encoded) == 0 || len(encoded) > maximumProgressionVectorBytes {
		return nil, permanentSolveReward(ErrSolveRewardInvariant)
	}
	return encoded, nil
}

func (projector *SolveRewardProjector) insertProgressionLedgerEntry(
	ctx context.Context,
	tx *stdsql.Tx,
	transition cultivationcontract.SolveRewardTransition,
	kind string,
	reversedEntryID *uuid.UUID,
	vector progressionRewardVector,
	balance progressionBalance,
	now time.Time,
) (uuid.UUID, error) {
	id := projector.newID()
	if id == uuid.Nil || (kind != "grant" && kind != "reversal") ||
		(kind == "grant") != (reversedEntryID == nil) ||
		vector.SourceChecksum == ([sha256Size]byte{}) ||
		vector.RewardRuleRevisionID == uuid.Nil || vector.RewardRuleVersion < 1 ||
		vector.RewardProfileRevisionID == uuid.Nil || vector.RewardProfileVersion < 1 ||
		balance.Version < 1 || balance.After-balance.Before != vector.TotalExperience {
		return uuid.Nil, permanentSolveReward(ErrSolveRewardInvariant)
	}
	elements, err := stdjson.Marshal(vector.Elements)
	if err != nil {
		return uuid.Nil, permanentSolveReward(err)
	}
	difficulties, err := stdjson.Marshal(vector.Difficulties)
	if err != nil {
		return uuid.Nil, permanentSolveReward(err)
	}
	tags, err := stdjson.Marshal(vector.Tags)
	if err != nil {
		return uuid.Nil, permanentSolveReward(err)
	}
	if !validProgressionVectorJSON(elements, difficulties, tags, vector.Provenance) {
		return uuid.Nil, permanentSolveReward(ErrSolveRewardInvariant)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO progression_ledger_entries (
			id, user_id, kind, source_type, source_aggregate_id,
				source_aggregate_version, source_transition_id, source_event_id,
				reversed_entry_id, reward_rule_revision_id, reward_profile_revision_id,
				total_exp_delta,
				balance_before, balance_after, progression_version, element_deltas,
				difficulty_deltas, tag_deltas, source_checksum,
				provenance, correlation_id, occurred_at, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
		id, transition.UserID, kind, progressionSolveSourceType, transition.SolveID,
		transition.To.Version, transition.TransitionID, transition.EventID,
		reversedEntryID, vector.RewardRuleRevisionID, vector.RewardProfileRevisionID,
		vector.TotalExperience,
		balance.Before, balance.After, balance.Version, elements, difficulties, tags,
		vector.SourceChecksum[:], vector.Provenance, transition.CorrelationID,
		transition.OccurredAt, now,
	)
	if err != nil {
		return uuid.Nil, mapSolveRewardWriteError("insert progression ledger entry", err)
	}
	return id, nil
}

func loadActiveProgressionGrant(
	ctx context.Context,
	tx *stdsql.Tx,
	ledgerID uuid.UUID,
	cursor solveRewardCursor,
	transition cultivationcontract.SolveRewardTransition,
) (storedProgressionGrant, error) {
	var (
		grant                                              storedProgressionGrant
		kind                                               string
		elements, difficulties, tags, checksum, provenance []byte
	)
	// progression_ledger_entries, reward_rule_revisions and
	// user_reward_profile_revisions are all append-only; none can be updated or
	// deleted. FOR SHARE OF ledger, rule, profile therefore provides no
	// protection and requires UPDATE privilege that the runtime roles do not
	// hold.
	err := tx.QueryRowContext(ctx, `
		SELECT ledger.id, ledger.user_id, ledger.kind, ledger.source_type,
			ledger.source_aggregate_id, ledger.source_aggregate_version,
				ledger.source_transition_id, ledger.source_event_id,
				ledger.reward_rule_revision_id, rule.revision,
				ledger.reward_profile_revision_id, profile.revision,
				ledger.total_exp_delta, ledger.element_deltas,
			ledger.difficulty_deltas, ledger.tag_deltas,
			ledger.source_checksum, ledger.provenance
		FROM progression_ledger_entries AS ledger
			JOIN reward_rule_revisions AS rule
			  ON rule.id = ledger.reward_rule_revision_id
			JOIN user_reward_profile_revisions AS profile
			  ON profile.id = ledger.reward_profile_revision_id
			WHERE ledger.id = $1`, ledgerID).Scan(
		&grant.ID, &grant.UserID, &kind, &grant.SourceType,
		&grant.SourceAggregateID, &grant.SourceVersion,
		&grant.SourceTransitionID, &grant.SourceEventID,
		&grant.Vector.RewardRuleRevisionID, &grant.Vector.RewardRuleVersion,
		&grant.Vector.RewardProfileRevisionID, &grant.Vector.RewardProfileVersion,
		&grant.Vector.TotalExperience,
		&elements, &difficulties, &tags,
		&checksum, &provenance,
	)
	if errors.Is(err, stdsql.ErrNoRows) {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err != nil {
		return storedProgressionGrant{}, fmt.Errorf("load active progression grant: %w", err)
	}
	if kind != "grant" || grant.ID != ledgerID || grant.UserID != cursor.UserID ||
		grant.UserID != transition.UserID || grant.SourceType != progressionSolveSourceType ||
		grant.SourceAggregateID != cursor.SolveID || grant.SourceAggregateID != transition.SolveID ||
		transition.From.RewardGrantSourceID == nil ||
		grant.SourceTransitionID != *transition.From.RewardGrantSourceID ||
		grant.SourceEventID == uuid.Nil || grant.SourceVersion < 1 ||
		grant.SourceVersion > cursor.LastAppliedVersion || grant.Vector.TotalExperience < 0 ||
		len(checksum) != sha256Size || len(provenance) == 0 {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	copy(grant.Vector.SourceChecksum[:], checksum)
	// Pointer moves may change the currently selected submission and therefore
	// its reward snapshot. The live accounting vector stays bound to the
	// original RewardGrantSourceID until a reversal, so validate the immutable
	// grant itself rather than comparing it with the latest pointer snapshot.
	if !validProgressionVectorJSON(elements, difficulties, tags, provenance) {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	grant.Vector.Provenance = append(stdjson.RawMessage(nil), provenance...)
	if err := commonjson.UnmarshalStrictLimit(elements, &grant.Vector.Elements, maximumProgressionVectorBytes); err != nil {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err := commonjson.UnmarshalStrictLimit(difficulties, &grant.Vector.Difficulties, maximumProgressionVectorBytes); err != nil {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err := commonjson.UnmarshalStrictLimit(tags, &grant.Vector.Tags, maximumProgressionVectorBytes); err != nil {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	if !validStoredProgressionGrant(grant) {
		return storedProgressionGrant{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	return grant, nil
}

func validStoredProgressionGrant(grant storedProgressionGrant) bool {
	if grant.Vector.SourceChecksum == ([sha256Size]byte{}) ||
		grant.Vector.RewardRuleRevisionID == uuid.Nil ||
		grant.Vector.RewardRuleVersion < 1 ||
		grant.Vector.RewardProfileRevisionID == uuid.Nil ||
		grant.Vector.RewardProfileVersion < 1 ||
		len(grant.Vector.Difficulties) != 1 ||
		grant.Vector.Difficulties[0].DifficultyID == uuid.Nil ||
		grant.Vector.Difficulties[0].SolvedDelta != 1 {
		return false
	}
	elements := make(map[uuid.UUID]struct{}, len(grant.Vector.Elements))
	previousElement := uuid.Nil
	for index, element := range grant.Vector.Elements {
		if element.ElementID == uuid.Nil || element.ExpDelta <= 0 ||
			(index > 0 && bytes.Compare(previousElement[:], element.ElementID[:]) >= 0) {
			return false
		}
		if _, duplicate := elements[element.ElementID]; duplicate {
			return false
		}
		elements[element.ElementID] = struct{}{}
		previousElement = element.ElementID
	}
	tags := make(map[uuid.UUID]struct{}, len(grant.Vector.Tags))
	previousTag := uuid.Nil
	for index, tag := range grant.Vector.Tags {
		if tag.TagID == uuid.Nil || tag.SolvedDelta != 1 ||
			(index > 0 && bytes.Compare(previousTag[:], tag.TagID[:]) >= 0) {
			return false
		}
		if _, duplicate := tags[tag.TagID]; duplicate {
			return false
		}
		tags[tag.TagID] = struct{}{}
		previousTag = tag.TagID
	}
	return true
}

func validProgressionVectorJSON(values ...[]byte) bool {
	for _, value := range values {
		if len(value) == 0 || len(value) > maximumProgressionVectorBytes || !stdjson.Valid(value) {
			return false
		}
	}
	return true
}

func (projector *SolveRewardProjector) applyProgressionRewardVector(
	ctx context.Context,
	tx *stdsql.Tx,
	userID uuid.UUID,
	vector progressionRewardVector,
	balance progressionBalance,
	now time.Time,
) error {
	if userID == uuid.Nil || vector.TotalExperience == -maximumSignedBigint-1 {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err := updateProgressionTotalExperience(ctx, tx, userID, balance, now); err != nil {
		return err
	}
	for _, element := range vector.Elements {
		if err := projector.updateProgressionElementExperience(ctx, tx, userID, element, now); err != nil {
			return err
		}
	}
	for _, difficulty := range vector.Difficulties {
		if err := projector.updateProgressionDifficultyCount(ctx, tx, userID, difficulty); err != nil {
			return err
		}
	}
	for _, tag := range vector.Tags {
		if err := projector.updateProgressionTagCount(ctx, tx, userID, tag); err != nil {
			return err
		}
	}
	return nil
}

func updateProgressionTotalExperience(
	ctx context.Context,
	tx *stdsql.Tx,
	userID uuid.UUID,
	balance progressionBalance,
	now time.Time,
) error {
	if userID == uuid.Nil || balance.Version < 1 || balance.Before < 0 ||
		balance.After < 0 || balance.Before > maximumProgressionBalance ||
		balance.After > maximumProgressionBalance || now.IsZero() {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	var total, version int64
	err := tx.QueryRowContext(ctx, `
		UPDATE user_stats
		SET total_exp = $2, progression_version = $3,
			progression_updated_at = $4, updated_at = $4
		WHERE user_id = $1 AND deleted_at IS NULL
		  AND total_exp = $5 AND progression_version = $3 - 1
		RETURNING total_exp, progression_version`,
		userID, balance.After, balance.Version, now, balance.Before,
	).Scan(&total, &version)
	if errors.Is(err, stdsql.ErrNoRows) {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err != nil {
		return mapSolveRewardWriteError("update total experience", err)
	}
	if total != balance.After || version != balance.Version {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err := synchronizeProgressionRanking(ctx, tx, userID, now); err != nil {
		return permanentSolveReward(err)
	}
	return nil
}

const maximumProgressionBalance = int64(9_007_199_254_740_991)

func lockProgressionBalance(
	ctx context.Context,
	tx *stdsql.Tx,
	userID uuid.UUID,
	delta int64,
) (progressionBalance, error) {
	if ctx == nil || tx == nil || userID == uuid.Nil || delta == 0 ||
		delta == -maximumSignedBigint-1 {
		return progressionBalance{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	var before, version int64
	err := tx.QueryRowContext(ctx, `
		SELECT total_exp, progression_version
		FROM user_stats
		WHERE user_id = $1 AND deleted_at IS NULL
		FOR UPDATE`, userID).Scan(&before, &version)
	if errors.Is(err, stdsql.ErrNoRows) {
		return progressionBalance{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	if err != nil {
		return progressionBalance{}, fmt.Errorf("lock progression balance: %w", err)
	}
	if before < 0 || before > maximumProgressionBalance || version < 0 ||
		(delta > 0 && before > maximumProgressionBalance-delta) ||
		(delta < 0 && before < -delta) {
		return progressionBalance{}, permanentSolveReward(ErrSolveRewardInvariant)
	}
	return progressionBalance{
		Before:  before,
		After:   before + delta,
		Version: version + 1,
	}, nil
}

func (projector *SolveRewardProjector) updateProgressionElementExperience(
	ctx context.Context,
	tx *stdsql.Tx,
	userID uuid.UUID,
	delta progressionElementDelta,
	now time.Time,
) error {
	if delta.ElementID == uuid.Nil || delta.ExpDelta == 0 || delta.ExpDelta == -maximumSignedBigint-1 {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	var total int64
	if delta.ExpDelta > 0 {
		id := projector.newID()
		if id == uuid.Nil {
			return ErrInvalidSolveRewardProjector
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO user_element_exps (
				id, created_at, updated_at, user_id, element_id, exp
			) VALUES ($1,$2,$2,$3,$4,$5)
			ON CONFLICT (user_id, element_id) DO UPDATE
			SET exp = user_element_exps.exp + EXCLUDED.exp,
				updated_at = EXCLUDED.updated_at, deleted_at = NULL
			WHERE user_element_exps.exp <= $6 - EXCLUDED.exp
			RETURNING exp`,
			id, now, userID, delta.ElementID, delta.ExpDelta, maximumSignedBigint,
		).Scan(&total)
		if errors.Is(err, stdsql.ErrNoRows) {
			return permanentSolveReward(ErrSolveRewardInvariant)
		}
		if err != nil {
			return mapSolveRewardWriteError("grant element experience", err)
		}
	} else {
		required := -delta.ExpDelta
		err := tx.QueryRowContext(ctx, `
			UPDATE user_element_exps
			SET exp = exp - $3, updated_at = $4
			WHERE user_id = $1 AND element_id = $2 AND deleted_at IS NULL AND exp >= $3
			RETURNING exp`, userID, delta.ElementID, required, now).Scan(&total)
		if errors.Is(err, stdsql.ErrNoRows) {
			return permanentSolveReward(ErrSolveRewardInvariant)
		}
		if err != nil {
			return mapSolveRewardWriteError("reverse element experience", err)
		}
	}
	if total < 0 {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	return nil
}

func (projector *SolveRewardProjector) updateProgressionDifficultyCount(
	ctx context.Context,
	tx *stdsql.Tx,
	userID uuid.UUID,
	delta progressionDifficultyDelta,
) error {
	if delta.DifficultyID == uuid.Nil || (delta.SolvedDelta != 1 && delta.SolvedDelta != -1) {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	var count int64
	if delta.SolvedDelta > 0 {
		id := projector.newID()
		if id == uuid.Nil {
			return ErrInvalidSolveRewardProjector
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO user_difficulty_stats (id, user_id, difficulty_id, solved_count)
			VALUES ($1,$2,$3,1)
			ON CONFLICT (user_id, difficulty_id) DO UPDATE
			SET solved_count = user_difficulty_stats.solved_count + 1
			WHERE user_difficulty_stats.solved_count < $4
			RETURNING solved_count`, id, userID, delta.DifficultyID, maximumProgressionCount).Scan(&count)
		if errors.Is(err, stdsql.ErrNoRows) {
			return permanentSolveReward(ErrSolveRewardInvariant)
		}
		if err != nil {
			return mapSolveRewardWriteError("grant difficulty solve", err)
		}
	} else {
		err := tx.QueryRowContext(ctx, `
			UPDATE user_difficulty_stats
			SET solved_count = solved_count - 1
			WHERE user_id = $1 AND difficulty_id = $2 AND solved_count >= 1
			RETURNING solved_count`, userID, delta.DifficultyID).Scan(&count)
		if errors.Is(err, stdsql.ErrNoRows) {
			return permanentSolveReward(ErrSolveRewardInvariant)
		}
		if err != nil {
			return mapSolveRewardWriteError("reverse difficulty solve", err)
		}
	}
	if count < 0 {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	return nil
}

func (projector *SolveRewardProjector) updateProgressionTagCount(
	ctx context.Context,
	tx *stdsql.Tx,
	userID uuid.UUID,
	delta progressionTagDelta,
) error {
	if delta.TagID == uuid.Nil || (delta.SolvedDelta != 1 && delta.SolvedDelta != -1) {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	var count int64
	if delta.SolvedDelta > 0 {
		id := projector.newID()
		if id == uuid.Nil {
			return ErrInvalidSolveRewardProjector
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO user_tag_stats (id, user_id, tag_id, solved_count)
			VALUES ($1,$2,$3,1)
			ON CONFLICT (user_id, tag_id) DO UPDATE
			SET solved_count = user_tag_stats.solved_count + 1
			WHERE user_tag_stats.solved_count < $4
			RETURNING solved_count`, id, userID, delta.TagID, maximumProgressionCount).Scan(&count)
		if errors.Is(err, stdsql.ErrNoRows) {
			return permanentSolveReward(ErrSolveRewardInvariant)
		}
		if err != nil {
			return mapSolveRewardWriteError("grant tag solve", err)
		}
	} else {
		err := tx.QueryRowContext(ctx, `
			UPDATE user_tag_stats
			SET solved_count = solved_count - 1
			WHERE user_id = $1 AND tag_id = $2 AND solved_count >= 1
			RETURNING solved_count`, userID, delta.TagID).Scan(&count)
		if errors.Is(err, stdsql.ErrNoRows) {
			return permanentSolveReward(ErrSolveRewardInvariant)
		}
		if err != nil {
			return mapSolveRewardWriteError("reverse tag solve", err)
		}
	}
	if count < 0 {
		return permanentSolveReward(ErrSolveRewardInvariant)
	}
	return nil
}
