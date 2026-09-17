package domain

import (
	"bytes"
	"sort"

	"github.com/google/uuid"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

type RewardFacts struct {
	Difficulty DifficultyRewardFact
	Tags       []TaxonomyRevisionFact
	Elements   []TaxonomyRevisionFact
}

type DifficultyRewardFact struct {
	ID             uuid.UUID
	Revision       int64
	BaseExperience int64
}

type TaxonomyRevisionFact struct {
	ID       uuid.UUID
	Revision int64
}

type RewardMappingIdentity struct {
	TagMappingID     uuid.UUID
	ElementMappingID uuid.UUID
}

func BuildRewardSnapshot(
	identity RewardMappingIdentity,
	mappingRevision int64,
	facts RewardFacts,
) (RewardSnapshot, error) {
	if identity.TagMappingID == uuid.Nil || identity.ElementMappingID == uuid.Nil ||
		mappingRevision < 1 ||
		mappingRevision > problemcontract.MaximumRewardMappingRevision {
		return RewardSnapshot{}, invalidRevisionField("reward_mapping", "invalid_identity")
	}
	if facts.Difficulty.ID == uuid.Nil ||
		facts.Difficulty.Revision < 1 ||
		facts.Difficulty.Revision > problemcontract.MaximumDifficultyRevision ||
		facts.Difficulty.BaseExperience < 0 ||
		facts.Difficulty.BaseExperience > problemcontract.MaximumBaseExperience {
		return RewardSnapshot{}, invalidRevisionField("difficulty", "invalid_fact")
	}
	if len(facts.Tags) > problemcontract.MaximumTags {
		return RewardSnapshot{}, invalidRevisionField("tag_snapshot", "too_many_items")
	}
	if len(facts.Elements) > problemcontract.MaximumElements {
		return RewardSnapshot{}, invalidRevisionField("element_snapshot", "too_many_items")
	}

	tags, err := canonicalTaxonomyFacts(facts.Tags, "tag_snapshot")
	if err != nil {
		return RewardSnapshot{}, err
	}
	elements, err := canonicalTaxonomyFacts(facts.Elements, "element_snapshot")
	if err != nil {
		return RewardSnapshot{}, err
	}

	snapshot := RewardSnapshot{
		DifficultyID:       facts.Difficulty.ID,
		DifficultyRevision: facts.Difficulty.Revision,
		BaseExperience:     facts.Difficulty.BaseExperience,
		Tags: TagMappingSnapshot{
			MappingID: identity.TagMappingID,
			Revision:  mappingRevision,
			Items:     make([]TagSnapshotItem, len(tags)),
		},
	}
	for index, fact := range tags {
		snapshot.Tags.Items[index] = TagSnapshotItem{
			TagID:       fact.ID,
			TagRevision: fact.Revision,
		}
	}
	if len(elements) > 0 {
		base := problemcontract.MaximumRewardWeightBasisPoints / len(elements)
		remainder := problemcontract.MaximumRewardWeightBasisPoints % len(elements)
		snapshot.Elements = &ElementMappingSnapshot{
			MappingID: identity.ElementMappingID,
			Revision:  mappingRevision,
			Items:     make([]ElementSnapshotItem, len(elements)),
		}
		for index, fact := range elements {
			weight := base
			if index < remainder {
				weight++
			}
			snapshot.Elements.Items[index] = ElementSnapshotItem{
				ElementID:         fact.ID,
				ElementRevision:   fact.Revision,
				WeightBasisPoints: weight,
			}
		}
	}

	checksum, err := snapshot.ComputeChecksum()
	if err != nil {
		return RewardSnapshot{}, err
	}
	snapshot.Checksum = checksum
	if err := snapshot.Validate(); err != nil {
		return RewardSnapshot{}, err
	}
	return snapshot, nil
}

func canonicalTaxonomyFacts(
	facts []TaxonomyRevisionFact,
	field string,
) ([]TaxonomyRevisionFact, error) {
	result := append([]TaxonomyRevisionFact(nil), facts...)
	sort.Slice(result, func(first, second int) bool {
		return bytes.Compare(result[first].ID[:], result[second].ID[:]) < 0
	})
	for index, fact := range result {
		if fact.ID == uuid.Nil || fact.Revision < 1 ||
			fact.Revision > problemcontract.MaximumRewardMappingRevision {
			return nil, invalidRevisionField(field, "invalid_fact")
		}
		if index > 0 && result[index-1].ID == fact.ID {
			return nil, invalidRevisionField(field, "duplicate_fact")
		}
	}
	return result, nil
}
