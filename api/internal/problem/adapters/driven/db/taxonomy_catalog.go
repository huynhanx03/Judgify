package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/difficulty"
	"github.com/huynhanx03/judgify/internal/ent/generate/difficultyrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/elementrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/tag"
	"github.com/huynhanx03/judgify/internal/ent/generate/tagrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/tagrevisionelement"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var ErrInvalidTaxonomyCatalog = errors.New("problem taxonomy catalog: invalid dependency or facts")

// TaxonomyCatalog resolves exact, immutable reward facts. It deliberately
// requires an already active transaction: draft/publication services decide
// the transaction boundary and this adapter must observe the same snapshot.
type TaxonomyCatalog struct {
	client *generate.Client
}

func NewTaxonomyCatalog(client *generate.Client) (*TaxonomyCatalog, error) {
	if client == nil {
		return nil, ErrInvalidTaxonomyCatalog
	}
	return &TaxonomyCatalog{client: client}, nil
}

var _ ports.TaxonomyCatalog = (*TaxonomyCatalog)(nil)

func (catalog *TaxonomyCatalog) ResolveRewardFacts(
	ctx context.Context,
	intent ports.RewardIntent,
) (entity.RewardFacts, error) {
	if catalog == nil || catalog.client == nil || ctx == nil {
		return entity.RewardFacts{}, ErrInvalidTaxonomyCatalog
	}
	if err := ctx.Err(); err != nil {
		return entity.RewardFacts{}, err
	}
	if intent.DifficultyID == uuid.Nil {
		return entity.RewardFacts{}, ErrInvalidTaxonomyCatalog
	}
	tagIDs, err := canonicalUUIDs(intent.TagIDs, problemcontract.MaximumTags)
	if err != nil {
		return entity.RewardFacts{}, err
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return entity.RewardFacts{}, err
	}

	difficultyRecord, err := client.Difficulty.Query().
		Where(difficulty.ID(intent.DifficultyID), difficulty.DeletedAtIsNil()).
		Modify(func(selector *entsql.Selector) { selector.ForUpdate() }).
		Only(ctx)
	if err != nil {
		return entity.RewardFacts{}, catalogReadError(err)
	}
	if difficultyRecord.ID != intent.DifficultyID ||
		difficultyRecord.Version < 1 ||
		difficultyRecord.Version > problemcontract.MaximumDifficultyRevision {
		return entity.RewardFacts{}, ErrInvalidTaxonomyCatalog
	}
	difficultyRevisionRecord, err := client.DifficultyRevision.Query().
		Where(
			difficultyrevision.DifficultyID(difficultyRecord.ID),
			difficultyrevision.Revision(difficultyRecord.Version),
		).
		Only(ctx)
	if err != nil {
		return entity.RewardFacts{}, catalogReadError(err)
	}
	if err := validateDifficultyRevisionRecord(difficultyRevisionRecord); err != nil {
		return entity.RewardFacts{}, err
	}

	tagRecords, err := resolveActiveTags(ctx, client, tagIDs)
	if err != nil {
		return entity.RewardFacts{}, err
	}
	tagRevisionRecords, err := resolveExactCurrentTagRevisions(ctx, client, tagRecords)
	if err != nil {
		return entity.RewardFacts{}, err
	}
	elementFacts, err := resolveTagRevisionElementFacts(ctx, client, tagRevisionRecords)
	if err != nil {
		return entity.RewardFacts{}, err
	}

	tagFacts := make([]entity.TaxonomyRevisionFact, len(tagRevisionRecords))
	for index, record := range tagRevisionRecords {
		tagFacts[index] = entity.TaxonomyRevisionFact{ID: record.TagID, Revision: record.Revision}
	}
	return entity.RewardFacts{
		Difficulty: entity.DifficultyRewardFact{
			ID:             difficultyRevisionRecord.DifficultyID,
			Revision:       difficultyRevisionRecord.Revision,
			BaseExperience: difficultyRevisionRecord.ExpReward,
		},
		Tags:     tagFacts,
		Elements: elementFacts,
	}, nil
}

func (catalog *TaxonomyCatalog) VerifyRewardSnapshot(
	ctx context.Context,
	snapshot entity.RewardSnapshot,
) error {
	if catalog == nil || catalog.client == nil || ctx == nil {
		return ErrInvalidTaxonomyCatalog
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return err
	}
	facts, err := resolveHistoricalRewardFacts(ctx, client, snapshot)
	if err != nil {
		return err
	}
	identity := entity.RewardMappingIdentity{
		TagMappingID: snapshot.Tags.MappingID,
		// Empty element mappings are intentionally omitted from immutable problem
		// revisions. The identity is not part of the serialized/checksummed shape
		// in that case, so use the already validated tag identity solely to satisfy
		// the pure factory's non-nil input contract.
		ElementMappingID: snapshot.Tags.MappingID,
	}
	if snapshot.Elements != nil {
		identity.ElementMappingID = snapshot.Elements.MappingID
	}
	expected, err := entity.BuildRewardSnapshot(identity, snapshot.Tags.Revision, facts)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(expected, snapshot) {
		return ErrInvalidTaxonomyCatalog
	}
	return nil
}

func resolveActiveTags(
	ctx context.Context,
	client *generate.Client,
	ids []uuid.UUID,
) ([]*generate.Tag, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	records, err := client.Tag.Query().
		Where(tag.IDIn(ids...), tag.DeletedAtIsNil()).
		Order(tag.ByID()).
		Modify(func(selector *entsql.Selector) { selector.ForUpdate() }).
		All(ctx)
	if err != nil {
		return nil, catalogReadError(err)
	}
	if len(records) != len(ids) {
		return nil, ErrInvalidTaxonomyCatalog
	}
	sort.Slice(records, func(i, j int) bool { return uuidLess(records[i].ID, records[j].ID) })
	for index, record := range records {
		if record == nil || record.ID != ids[index] ||
			record.Version < 1 || record.Version > problemcontract.MaximumRewardMappingRevision {
			return nil, ErrInvalidTaxonomyCatalog
		}
	}
	return records, nil
}

func resolveExactCurrentTagRevisions(
	ctx context.Context,
	client *generate.Client,
	tags []*generate.Tag,
) ([]*generate.TagRevision, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	predicates := make([]*entsql.Predicate, 0, len(tags))
	for _, stable := range tags {
		predicates = append(predicates, entsql.And(
			entsql.EQ(tagrevision.FieldTagID, stable.ID),
			entsql.EQ(tagrevision.FieldRevision, stable.Version),
		))
	}
	records, err := client.TagRevision.Query().
		Where(func(selector *entsql.Selector) { selector.Where(entsql.Or(predicates...)) }).
		All(ctx)
	if err != nil {
		return nil, catalogReadError(err)
	}
	if len(records) != len(tags) {
		return nil, ErrInvalidTaxonomyCatalog
	}
	byTagID := make(map[uuid.UUID]*generate.TagRevision, len(records))
	for _, record := range records {
		if err := validateTagRevisionRecord(record); err != nil {
			return nil, err
		}
		if _, duplicate := byTagID[record.TagID]; duplicate {
			return nil, ErrInvalidTaxonomyCatalog
		}
		byTagID[record.TagID] = record
	}
	canonical := make([]*generate.TagRevision, len(tags))
	for index, stable := range tags {
		record, ok := byTagID[stable.ID]
		if !ok || record.Revision != stable.Version {
			return nil, ErrInvalidTaxonomyCatalog
		}
		canonical[index] = record
	}
	return canonical, nil
}

func resolveTagRevisionElementFacts(
	ctx context.Context,
	client *generate.Client,
	tagRevisions []*generate.TagRevision,
) ([]entity.TaxonomyRevisionFact, error) {
	if len(tagRevisions) == 0 {
		return nil, nil
	}
	revisionIDs := make([]uuid.UUID, len(tagRevisions))
	for index, record := range tagRevisions {
		if record == nil || record.ID == uuid.Nil {
			return nil, ErrInvalidTaxonomyCatalog
		}
		revisionIDs[index] = record.ID
	}
	links, err := client.TagRevisionElement.Query().
		Where(tagrevisionelement.TagRevisionIDIn(revisionIDs...)).
		All(ctx)
	if err != nil {
		return nil, catalogReadError(err)
	}
	knownTagRevisionIDs := make(map[uuid.UUID]struct{}, len(revisionIDs))
	tagRevisionByID := make(map[uuid.UUID]*generate.TagRevision, len(tagRevisions))
	for _, id := range revisionIDs {
		knownTagRevisionIDs[id] = struct{}{}
	}
	for _, record := range tagRevisions {
		tagRevisionByID[record.ID] = record
	}
	tupleByElementID := make(map[uuid.UUID]int64, len(links))
	seenPerTag := make(map[uuid.UUID]map[uuid.UUID]struct{}, len(revisionIDs))
	elementsPerTag := make(map[uuid.UUID][]elementRevisionFact, len(revisionIDs))
	for _, link := range links {
		if link == nil || link.ID == uuid.Nil || link.TagRevisionID == uuid.Nil ||
			link.ElementID == uuid.Nil || link.ElementRevision < 1 ||
			link.ElementRevision > problemcontract.MaximumRewardMappingRevision {
			return nil, ErrInvalidTaxonomyCatalog
		}
		if _, ok := knownTagRevisionIDs[link.TagRevisionID]; !ok {
			return nil, ErrInvalidTaxonomyCatalog
		}
		if seenPerTag[link.TagRevisionID] == nil {
			seenPerTag[link.TagRevisionID] = make(map[uuid.UUID]struct{})
		}
		if _, duplicate := seenPerTag[link.TagRevisionID][link.ElementID]; duplicate {
			return nil, ErrInvalidTaxonomyCatalog
		}
		seenPerTag[link.TagRevisionID][link.ElementID] = struct{}{}
		if revision, exists := tupleByElementID[link.ElementID]; exists && revision != link.ElementRevision {
			// A single reward element snapshot cannot truthfully represent two
			// different immutable revisions of the same stable element.
			return nil, ErrInvalidTaxonomyCatalog
		}
		tupleByElementID[link.ElementID] = link.ElementRevision
		elementsPerTag[link.TagRevisionID] = append(elementsPerTag[link.TagRevisionID], elementRevisionFact{
			ID:       link.ElementID,
			Revision: link.ElementRevision,
		})
	}
	for _, tagRevision := range tagRevisions {
		expected := tagRevisionChecksum(
			tagRevision.TagID,
			tagRevision.Revision,
			tagRevision.Name,
			elementsPerTag[tagRevision.ID],
		)
		if !bytes.Equal(tagRevision.Checksum, expected[:]) {
			return nil, ErrInvalidTaxonomyCatalog
		}
	}
	if len(tupleByElementID) == 0 {
		return nil, nil
	}
	return resolveExactElementFacts(ctx, client, tupleByElementID)
}

func resolveHistoricalRewardFacts(
	ctx context.Context,
	client *generate.Client,
	snapshot entity.RewardSnapshot,
) (entity.RewardFacts, error) {
	difficultyRecord, err := client.DifficultyRevision.Query().
		Where(
			difficultyrevision.DifficultyID(snapshot.DifficultyID),
			difficultyrevision.Revision(snapshot.DifficultyRevision),
		).
		Only(ctx)
	if err != nil {
		return entity.RewardFacts{}, catalogReadError(err)
	}
	if err := validateDifficultyRevisionRecord(difficultyRecord); err != nil {
		return entity.RewardFacts{}, err
	}

	tagRecords, err := resolveExactHistoricalTagRevisions(ctx, client, snapshot.Tags.Items)
	if err != nil {
		return entity.RewardFacts{}, err
	}
	elementFacts, err := resolveTagRevisionElementFacts(ctx, client, tagRecords)
	if err != nil {
		return entity.RewardFacts{}, err
	}
	tagFacts := make([]entity.TaxonomyRevisionFact, len(tagRecords))
	for index, record := range tagRecords {
		tagFacts[index] = entity.TaxonomyRevisionFact{ID: record.TagID, Revision: record.Revision}
	}
	return entity.RewardFacts{
		Difficulty: entity.DifficultyRewardFact{
			ID:             difficultyRecord.DifficultyID,
			Revision:       difficultyRecord.Revision,
			BaseExperience: difficultyRecord.ExpReward,
		},
		Tags:     tagFacts,
		Elements: elementFacts,
	}, nil
}

func resolveExactHistoricalTagRevisions(
	ctx context.Context,
	client *generate.Client,
	items []entity.TagSnapshotItem,
) ([]*generate.TagRevision, error) {
	if len(items) == 0 {
		return nil, nil
	}
	predicates := make([]*entsql.Predicate, 0, len(items))
	for _, item := range items {
		predicates = append(predicates, entsql.And(
			entsql.EQ(tagrevision.FieldTagID, item.TagID),
			entsql.EQ(tagrevision.FieldRevision, item.TagRevision),
		))
	}
	records, err := client.TagRevision.Query().
		Where(func(selector *entsql.Selector) { selector.Where(entsql.Or(predicates...)) }).
		All(ctx)
	if err != nil {
		return nil, catalogReadError(err)
	}
	if len(records) != len(items) {
		return nil, ErrInvalidTaxonomyCatalog
	}
	byID := make(map[uuid.UUID]*generate.TagRevision, len(records))
	for _, record := range records {
		if err := validateTagRevisionRecord(record); err != nil {
			return nil, err
		}
		if _, duplicate := byID[record.TagID]; duplicate {
			return nil, ErrInvalidTaxonomyCatalog
		}
		byID[record.TagID] = record
	}
	canonical := make([]*generate.TagRevision, len(items))
	for index, item := range items {
		record, ok := byID[item.TagID]
		if !ok || record.Revision != item.TagRevision {
			return nil, ErrInvalidTaxonomyCatalog
		}
		canonical[index] = record
	}
	return canonical, nil
}

func resolveExactElementFacts(
	ctx context.Context,
	client *generate.Client,
	tuples map[uuid.UUID]int64,
) ([]entity.TaxonomyRevisionFact, error) {
	predicates := make([]*entsql.Predicate, 0, len(tuples))
	for id, revision := range tuples {
		predicates = append(predicates, entsql.And(
			entsql.EQ(elementrevision.FieldElementID, id),
			entsql.EQ(elementrevision.FieldRevision, revision),
		))
	}
	records, err := client.ElementRevision.Query().
		Where(func(selector *entsql.Selector) { selector.Where(entsql.Or(predicates...)) }).
		All(ctx)
	if err != nil {
		return nil, catalogReadError(err)
	}
	if len(records) != len(tuples) {
		return nil, ErrInvalidTaxonomyCatalog
	}
	facts := make([]entity.TaxonomyRevisionFact, 0, len(records))
	for _, record := range records {
		if err := validateElementRevisionRecord(record); err != nil {
			return nil, err
		}
		if wanted, ok := tuples[record.ElementID]; !ok || wanted != record.Revision {
			return nil, ErrInvalidTaxonomyCatalog
		}
		facts = append(facts, entity.TaxonomyRevisionFact{ID: record.ElementID, Revision: record.Revision})
	}
	sort.Slice(facts, func(i, j int) bool { return uuidLess(facts[i].ID, facts[j].ID) })
	for index, fact := range facts {
		if index > 0 && facts[index-1].ID == fact.ID {
			return nil, ErrInvalidTaxonomyCatalog
		}
	}
	return facts, nil
}

func validateDifficultyRevisionRecord(record *generate.DifficultyRevision) error {
	if record == nil || record.ID == uuid.Nil || record.DifficultyID == uuid.Nil ||
		record.Revision < 1 || record.Revision > problemcontract.MaximumDifficultyRevision ||
		record.ExpReward < 0 || record.ExpReward > problemcontract.MaximumBaseExperience {
		return ErrInvalidTaxonomyCatalog
	}
	expected := difficultyRevisionChecksum(
		record.DifficultyID,
		record.Revision,
		record.Name,
		record.Level,
		record.ExpReward,
		record.Description,
	)
	if !bytes.Equal(record.Checksum, expected[:]) {
		return ErrInvalidTaxonomyCatalog
	}
	return nil
}

func validateTagRevisionRecord(record *generate.TagRevision) error {
	if record == nil || record.ID == uuid.Nil || record.TagID == uuid.Nil ||
		record.Revision < 1 || record.Revision > problemcontract.MaximumRewardMappingRevision ||
		len(record.Checksum) != 32 {
		return ErrInvalidTaxonomyCatalog
	}
	return nil
}

func validateElementRevisionRecord(record *generate.ElementRevision) error {
	if record == nil || record.ID == uuid.Nil || record.ElementID == uuid.Nil ||
		record.Revision < 1 || record.Revision > problemcontract.MaximumRewardMappingRevision ||
		len(record.Checksum) != 32 {
		return ErrInvalidTaxonomyCatalog
	}
	encoder := problemcontract.NewChecksumEncoder("judgify.element-revision.v1")
	encoder.UUID(record.ElementID)
	encoder.Int64(record.Revision)
	encoder.String(record.Name)
	encoder.String(record.Code)
	encoder.String(record.Description)
	expected := encoder.Sum()
	if !bytes.Equal(record.Checksum, expected[:]) {
		return ErrInvalidTaxonomyCatalog
	}
	return nil
}

func canonicalUUIDs(ids []uuid.UUID, maximum int) ([]uuid.UUID, error) {
	if len(ids) > maximum {
		return nil, ErrInvalidTaxonomyCatalog
	}
	canonical := append([]uuid.UUID(nil), ids...)
	sort.Slice(canonical, func(i, j int) bool { return uuidLess(canonical[i], canonical[j]) })
	for index, id := range canonical {
		if id == uuid.Nil || (index > 0 && canonical[index-1] == id) {
			return nil, ErrInvalidTaxonomyCatalog
		}
	}
	return canonical, nil
}

func uuidLess(left, right uuid.UUID) bool { return bytes.Compare(left[:], right[:]) < 0 }

func catalogReadError(err error) error {
	if err == nil {
		return nil
	}
	if generate.IsNotFound(err) {
		return ErrInvalidTaxonomyCatalog
	}
	return fmt.Errorf("read exact taxonomy revision: %w", err)
}
