package db

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
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

func TestTaxonomyCatalogRejectsInvalidDependenciesAndContexts(t *testing.T) {
	t.Parallel()

	catalog, err := NewTaxonomyCatalog(nil)
	if catalog != nil || !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("NewTaxonomyCatalog(nil) = (%#v, %v), want nil, ErrInvalidTaxonomyCatalog", catalog, err)
	}

	catalog, err = NewTaxonomyCatalog(generate.NewClient())
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	if _, err := catalog.ResolveRewardFacts(missingContext, ports.RewardIntent{DifficultyID: uuid.New()}); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("ResolveRewardFacts(nil context) error = %v, want ErrInvalidTaxonomyCatalog", err)
	}
	if err := catalog.VerifyRewardSnapshot(missingContext, entity.RewardSnapshot{}); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("VerifyRewardSnapshot(nil context) error = %v, want ErrInvalidTaxonomyCatalog", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := catalog.ResolveRewardFacts(ctx, ports.RewardIntent{DifficultyID: uuid.New()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveRewardFacts(cancelled) error = %v, want context.Canceled", err)
	}
	if _, err := catalog.ResolveRewardFacts(context.Background(), ports.RewardIntent{DifficultyID: uuid.New()}); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("ResolveRewardFacts(outside transaction) error = %v, want ErrTransactionRequired", err)
	}

	var typedNil *TaxonomyCatalog
	if _, err := typedNil.ResolveRewardFacts(context.Background(), ports.RewardIntent{}); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("typed-nil ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
	}
	if err := typedNil.VerifyRewardSnapshot(context.Background(), entity.RewardSnapshot{}); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("typed-nil VerifyRewardSnapshot() error = %v, want ErrInvalidTaxonomyCatalog", err)
	}
	for name, intent := range map[string]ports.RewardIntent{
		"nil difficulty": {DifficultyID: uuid.Nil},
		"nil tag": {
			DifficultyID: uuid.New(),
			TagIDs:       []uuid.UUID{uuid.Nil},
		},
		"duplicate tag": {
			DifficultyID: uuid.New(),
			TagIDs: func() []uuid.UUID {
				id := uuid.New()
				return []uuid.UUID{id, id}
			}(),
		},
	} {
		if _, err := catalog.ResolveRewardFacts(context.Background(), intent); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("%s ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", name, err)
		}
	}
}

func TestTaxonomyCatalogResolveRewardFactsAllowsExactEmptyTagSet(t *testing.T) {
	client, mock := newTaxonomyCatalogMock(t)
	catalog, err := NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	fixture := newTaxonomyCatalogFixture()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulties".*"deleted_at" IS NULL.*FOR UPDATE`).
		WillReturnRows(difficultyRows(fixture.difficulty))
	mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).
		WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
	mock.ExpectCommit()

	var facts entity.RewardFacts
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		var resolveErr error
		facts, resolveErr = catalog.ResolveRewardFacts(ctx, ports.RewardIntent{
			DifficultyID: fixture.difficulty.ID,
			TagIDs:       []uuid.UUID{},
		})
		return resolveErr
	})
	if err != nil {
		t.Fatalf("ResolveRewardFacts(empty tags) error = %v", err)
	}
	if facts.Tags == nil || len(facts.Tags) != 0 {
		t.Fatalf("empty tag facts = %#v, want owned empty slice", facts.Tags)
	}
	if facts.Elements != nil {
		t.Fatalf("empty element union = %#v, want nil", facts.Elements)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestTaxonomyCatalogResolveRewardFactsUsesExactActiveTuplesAndCanonicalOrder(t *testing.T) {
	client, mock := newTaxonomyCatalogMock(t)
	catalog, err := NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	fixture := newTaxonomyCatalogFixture()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulties".*"deleted_at" IS NULL.*FOR UPDATE`).
		WillReturnRows(difficultyRows(fixture.difficulty))
	mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).
		WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
	mock.ExpectQuery(`SELECT .* FROM "tags".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).
		WillReturnRows(tagRows(fixture.tags[1], fixture.tags[0]))
	mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).
		WillReturnRows(tagRevisionRows(fixture.tagRevisions[1], fixture.tagRevisions[0]))
	mock.ExpectQuery(`SELECT .* FROM "tag_revision_elements"`).
		WillReturnRows(tagRevisionElementRows(fixture.links...))
	mock.ExpectQuery(`SELECT .* FROM "element_revisions"`).
		WillReturnRows(elementRevisionRows(fixture.elementRevisions[1], fixture.elementRevisions[0]))
	mock.ExpectCommit()

	var facts entity.RewardFacts
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		var resolveErr error
		facts, resolveErr = catalog.ResolveRewardFacts(ctx, ports.RewardIntent{
			DifficultyID: fixture.difficulty.ID,
			TagIDs:       []uuid.UUID{fixture.tags[1].ID, fixture.tags[0].ID},
		})
		return resolveErr
	})
	if err != nil {
		t.Fatalf("ResolveRewardFacts() error = %v", err)
	}
	if facts.Difficulty != (entity.DifficultyRewardFact{
		ID:             fixture.difficulty.ID,
		Revision:       fixture.difficulty.Version,
		BaseExperience: fixture.difficultyRevision.ExpReward,
	}) {
		t.Fatalf("difficulty facts = %#v", facts.Difficulty)
	}
	wantTags := []entity.TaxonomyRevisionFact{
		{ID: fixture.tags[0].ID, Revision: fixture.tags[0].Version},
		{ID: fixture.tags[1].ID, Revision: fixture.tags[1].Version},
	}
	wantElements := []entity.TaxonomyRevisionFact{
		{ID: fixture.elementRevisions[0].ElementID, Revision: fixture.elementRevisions[0].Revision},
		{ID: fixture.elementRevisions[1].ElementID, Revision: fixture.elementRevisions[1].Revision},
	}
	if !equalTaxonomyFacts(facts.Tags, wantTags) || !equalTaxonomyFacts(facts.Elements, wantElements) {
		t.Fatalf("facts = %#v, want tags=%#v elements=%#v", facts, wantTags, wantElements)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestTaxonomyCatalogResolveRewardFactsRejectsPartialActiveSetAndInconsistentElementUnion(t *testing.T) {
	t.Run("missing active tag fails closed", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT .* FROM "difficulties".*"deleted_at" IS NULL.*FOR UPDATE`).WillReturnRows(difficultyRows(fixture.difficulty))
		mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
		mock.ExpectQuery(`SELECT .* FROM "tags".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).WillReturnRows(tagRows(fixture.tags[0]))
		mock.ExpectRollback()

		err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
			_, resolveErr := catalog.ResolveRewardFacts(ctx, ports.RewardIntent{
				DifficultyID: fixture.difficulty.ID,
				TagIDs:       []uuid.UUID{fixture.tags[0].ID, fixture.tags[1].ID},
			})
			return resolveErr
		})
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})

	t.Run("same element at two revisions fails rather than inventing a fact", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		fixture.links[2].ElementRevision++
		fixture.tagRevisions[1].Checksum = taxonomyTagChecksum(
			fixture.tagRevisions[1],
			[]elementRevisionFact{{ID: fixture.links[2].ElementID, Revision: fixture.links[2].ElementRevision}},
		)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT .* FROM "difficulties".*"deleted_at" IS NULL.*FOR UPDATE`).WillReturnRows(difficultyRows(fixture.difficulty))
		mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
		mock.ExpectQuery(`SELECT .* FROM "tags".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).WillReturnRows(tagRows(fixture.tags...))
		mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).WillReturnRows(tagRevisionRows(fixture.tagRevisions...))
		mock.ExpectQuery(`SELECT .* FROM "tag_revision_elements"`).WillReturnRows(tagRevisionElementRows(fixture.links...))
		mock.ExpectRollback()

		err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
			_, resolveErr := catalog.ResolveRewardFacts(ctx, ports.RewardIntent{
				DifficultyID: fixture.difficulty.ID,
				TagIDs:       []uuid.UUID{fixture.tags[0].ID, fixture.tags[1].ID},
			})
			return resolveErr
		})
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})
}

func TestTaxonomyCatalogResolveRewardFactsRejectsMissingExactRevisions(t *testing.T) {
	t.Run("difficulty revision", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT .* FROM "difficulties".*"deleted_at" IS NULL.*FOR UPDATE`).
			WillReturnRows(difficultyRows(fixture.difficulty))
		mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).
			WillReturnRows(difficultyRevisionRows())
		mock.ExpectRollback()

		err = resolveTaxonomyFixture(client, catalog, fixture)
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})

	t.Run("tag revision", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		expectActiveTaxonomyPrefix(mock, fixture)
		mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).
			WillReturnRows(tagRevisionRows(fixture.tagRevisions[0]))
		mock.ExpectRollback()

		err = resolveTaxonomyFixture(client, catalog, fixture)
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})

	t.Run("element revision", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		expectActiveTaxonomyPrefix(mock, fixture)
		mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).
			WillReturnRows(tagRevisionRows(fixture.tagRevisions...))
		mock.ExpectQuery(`SELECT .* FROM "tag_revision_elements"`).
			WillReturnRows(tagRevisionElementRows(fixture.links...))
		mock.ExpectQuery(`SELECT .* FROM "element_revisions"`).
			WillReturnRows(elementRevisionRows(fixture.elementRevisions[0]))
		mock.ExpectRollback()

		err = resolveTaxonomyFixture(client, catalog, fixture)
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})
}

func TestTaxonomyCatalogRejectsMalformedRevisionChecksumsAndLinks(t *testing.T) {
	t.Run("difficulty checksum", func(t *testing.T) {
		fixture := newTaxonomyCatalogFixture()
		fixture.difficultyRevision.Checksum[0] ^= 0xff
		if err := validateDifficultyRevisionRecord(fixture.difficultyRevision); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("validateDifficultyRevisionRecord() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
	})

	t.Run("element checksum", func(t *testing.T) {
		fixture := newTaxonomyCatalogFixture()
		fixture.elementRevisions[0].Checksum[0] ^= 0xff
		if err := validateElementRevisionRecord(fixture.elementRevisions[0]); !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("validateElementRevisionRecord() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
	})

	t.Run("tag checksum", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		fixture.tagRevisions[0].Checksum[0] ^= 0xff
		expectActiveTaxonomyPrefix(mock, fixture)
		mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).
			WillReturnRows(tagRevisionRows(fixture.tagRevisions...))
		mock.ExpectQuery(`SELECT .* FROM "tag_revision_elements"`).
			WillReturnRows(tagRevisionElementRows(fixture.links...))
		mock.ExpectRollback()

		err = resolveTaxonomyFixture(client, catalog, fixture)
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})

	t.Run("duplicate link", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, err := NewTaxonomyCatalog(client)
		if err != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", err)
		}
		fixture := newTaxonomyCatalogFixture()
		duplicate := *fixture.links[0]
		duplicate.ID = taxonomyUUID(64)
		fixture.links = append(fixture.links, &duplicate)
		expectActiveTaxonomyPrefix(mock, fixture)
		mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).
			WillReturnRows(tagRevisionRows(fixture.tagRevisions...))
		mock.ExpectQuery(`SELECT .* FROM "tag_revision_elements"`).
			WillReturnRows(tagRevisionElementRows(fixture.links...))
		mock.ExpectRollback()

		err = resolveTaxonomyFixture(client, catalog, fixture)
		if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("ResolveRewardFacts() error = %v, want ErrInvalidTaxonomyCatalog", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})
}

func TestTaxonomyCatalogVerifyRewardSnapshotUsesHistoricalTuplesAndRejectsForgedFacts(t *testing.T) {
	client, mock := newTaxonomyCatalogMock(t)
	catalog, err := NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	fixture := newTaxonomyCatalogFixture()
	facts := fixture.facts()
	snapshot, err := entity.BuildRewardSnapshot(entity.RewardMappingIdentity{
		TagMappingID:     taxonomyUUID(201),
		ElementMappingID: taxonomyUUID(202),
	}, 7, facts)
	if err != nil {
		t.Fatalf("BuildRewardSnapshot() error = %v", err)
	}

	expectHistoricalTaxonomyRead(mock, fixture)
	mock.ExpectCommit()
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		return catalog.VerifyRewardSnapshot(ctx, snapshot)
	})
	if err != nil {
		t.Fatalf("VerifyRewardSnapshot() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}

	client, mock = newTaxonomyCatalogMock(t)
	catalog, err = NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	forged := cloneTaxonomyRewardSnapshot(snapshot)
	forged.BaseExperience++
	checksum, checksumErr := forged.ComputeChecksum()
	if checksumErr != nil {
		t.Fatalf("forged ComputeChecksum() error = %v", checksumErr)
	}
	forged.Checksum = checksum
	expectHistoricalTaxonomyRead(mock, fixture)
	mock.ExpectRollback()
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		return catalog.VerifyRewardSnapshot(ctx, forged)
	})
	if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("VerifyRewardSnapshot(forged) error = %v, want ErrInvalidTaxonomyCatalog", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}

	client, mock = newTaxonomyCatalogMock(t)
	catalog, err = NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	forged = cloneTaxonomyRewardSnapshot(snapshot)
	forged.Elements.Items[0].WeightBasisPoints--
	forged.Elements.Items[1].WeightBasisPoints++
	checksum, checksumErr = forged.ComputeChecksum()
	if checksumErr != nil {
		t.Fatalf("forged weight ComputeChecksum() error = %v", checksumErr)
	}
	forged.Checksum = checksum
	expectHistoricalTaxonomyRead(mock, fixture)
	mock.ExpectRollback()
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		return catalog.VerifyRewardSnapshot(ctx, forged)
	})
	if !errors.Is(err, ErrInvalidTaxonomyCatalog) {
		t.Fatalf("VerifyRewardSnapshot(forged weight) error = %v, want ErrInvalidTaxonomyCatalog", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}

	forged = cloneTaxonomyRewardSnapshot(snapshot)
	forged.Checksum[0] ^= 0xff
	if err := catalog.VerifyRewardSnapshot(context.Background(), forged); !errors.Is(err, entity.ErrRewardChecksum) {
		t.Fatalf("VerifyRewardSnapshot(forged checksum) error = %v, want ErrRewardChecksum", err)
	}
}

func TestTaxonomyCatalogVerifyRewardSnapshotRejectsForgedMembershipAndExactRevision(t *testing.T) {
	fixture := newTaxonomyCatalogFixture()
	snapshot, err := entity.BuildRewardSnapshot(entity.RewardMappingIdentity{
		TagMappingID:     taxonomyUUID(201),
		ElementMappingID: taxonomyUUID(202),
	}, 7, fixture.facts())
	if err != nil {
		t.Fatalf("BuildRewardSnapshot() error = %v", err)
	}

	t.Run("element membership with a valid recomputed checksum", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, catalogErr := NewTaxonomyCatalog(client)
		if catalogErr != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", catalogErr)
		}
		forged := cloneTaxonomyRewardSnapshot(snapshot)
		forged.Elements.Items[0].ElementID = taxonomyUUID(29)
		checksum, checksumErr := forged.ComputeChecksum()
		if checksumErr != nil {
			t.Fatalf("ComputeChecksum() error = %v", checksumErr)
		}
		forged.Checksum = checksum
		expectHistoricalTaxonomyRead(mock, fixture)
		mock.ExpectRollback()
		verifyErr := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
			return catalog.VerifyRewardSnapshot(ctx, forged)
		})
		if !errors.Is(verifyErr, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("VerifyRewardSnapshot() error = %v, want ErrInvalidTaxonomyCatalog", verifyErr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})

	t.Run("missing exact tag revision", func(t *testing.T) {
		client, mock := newTaxonomyCatalogMock(t)
		catalog, catalogErr := NewTaxonomyCatalog(client)
		if catalogErr != nil {
			t.Fatalf("NewTaxonomyCatalog() error = %v", catalogErr)
		}
		forged := cloneTaxonomyRewardSnapshot(snapshot)
		forged.Tags.Items[0].TagRevision++
		checksum, checksumErr := forged.ComputeChecksum()
		if checksumErr != nil {
			t.Fatalf("ComputeChecksum() error = %v", checksumErr)
		}
		forged.Checksum = checksum
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).
			WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
		mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).
			WillReturnRows(tagRevisionRows(fixture.tagRevisions[1]))
		mock.ExpectRollback()
		verifyErr := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
			return catalog.VerifyRewardSnapshot(ctx, forged)
		})
		if !errors.Is(verifyErr, ErrInvalidTaxonomyCatalog) {
			t.Fatalf("VerifyRewardSnapshot() error = %v, want ErrInvalidTaxonomyCatalog", verifyErr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("SQL expectations: %v", err)
		}
	})
}

func TestTaxonomyCatalogVerifyRewardSnapshotSupportsHistoricalNoElementUnion(t *testing.T) {
	client, mock := newTaxonomyCatalogMock(t)
	catalog, err := NewTaxonomyCatalog(client)
	if err != nil {
		t.Fatalf("NewTaxonomyCatalog() error = %v", err)
	}
	fixture := newTaxonomyCatalogFixture()
	facts := fixture.facts()
	facts.Tags = nil
	facts.Elements = nil
	snapshot, err := entity.BuildRewardSnapshot(entity.RewardMappingIdentity{
		TagMappingID:     taxonomyUUID(201),
		ElementMappingID: taxonomyUUID(202),
	}, 7, facts)
	if err != nil {
		t.Fatalf("BuildRewardSnapshot() error = %v", err)
	}
	if snapshot.Elements != nil {
		t.Fatalf("snapshot elements = %#v, want nil", snapshot.Elements)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).
		WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
	mock.ExpectCommit()
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		return catalog.VerifyRewardSnapshot(ctx, snapshot)
	})
	if err != nil {
		t.Fatalf("VerifyRewardSnapshot(no elements) error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

type taxonomyCatalogFixture struct {
	difficulty         *generate.Difficulty
	difficultyRevision *generate.DifficultyRevision
	tags               []*generate.Tag
	tagRevisions       []*generate.TagRevision
	links              []*generate.TagRevisionElement
	elementRevisions   []*generate.ElementRevision
}

func newTaxonomyCatalogFixture() taxonomyCatalogFixture {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	difficultyID := taxonomyUUID(1)
	difficulty := &generate.Difficulty{ID: difficultyID, CreatedAt: now, UpdatedAt: now, Name: "Advanced", Level: 3, ExpReward: 340, Version: 2}
	difficultyChecksum := difficultyRevisionChecksum(difficultyID, 2, difficulty.Name, difficulty.Level, difficulty.ExpReward, difficulty.Description)
	difficultyRevision := &generate.DifficultyRevision{ID: taxonomyUUID(11), CreatedAt: now, DifficultyID: difficultyID, Revision: 2, Name: difficulty.Name, Level: difficulty.Level, ExpReward: difficulty.ExpReward, Description: difficulty.Description, Checksum: difficultyChecksum[:]}

	tagA := &generate.Tag{ID: taxonomyUUID(20), CreatedAt: now, UpdatedAt: now, Name: "graphs", Version: 3}
	tagB := &generate.Tag{ID: taxonomyUUID(21), CreatedAt: now, UpdatedAt: now, Name: "trees", Version: 2}
	elementAID, elementBID := taxonomyUUID(30), taxonomyUUID(31)
	elementA := taxonomyElementRevision(now, taxonomyUUID(41), elementAID, 5, "Traversal", "traversal")
	elementB := taxonomyElementRevision(now, taxonomyUUID(42), elementBID, 4, "DP", "dp")
	tagRevisionA := &generate.TagRevision{ID: taxonomyUUID(51), CreatedAt: now, TagID: tagA.ID, Revision: tagA.Version, Name: tagA.Name}
	tagRevisionB := &generate.TagRevision{ID: taxonomyUUID(52), CreatedAt: now, TagID: tagB.ID, Revision: tagB.Version, Name: tagB.Name}
	links := []*generate.TagRevisionElement{
		{ID: taxonomyUUID(61), CreatedAt: now, TagRevisionID: tagRevisionA.ID, ElementID: elementAID, ElementRevision: elementA.Revision},
		{ID: taxonomyUUID(62), CreatedAt: now, TagRevisionID: tagRevisionA.ID, ElementID: elementBID, ElementRevision: elementB.Revision},
		{ID: taxonomyUUID(63), CreatedAt: now, TagRevisionID: tagRevisionB.ID, ElementID: elementAID, ElementRevision: elementA.Revision},
	}
	tagRevisionA.Checksum = taxonomyTagChecksum(tagRevisionA, []elementRevisionFact{{ID: elementAID, Revision: elementA.Revision}, {ID: elementBID, Revision: elementB.Revision}})
	tagRevisionB.Checksum = taxonomyTagChecksum(tagRevisionB, []elementRevisionFact{{ID: elementAID, Revision: elementA.Revision}})
	return taxonomyCatalogFixture{
		difficulty: difficulty, difficultyRevision: difficultyRevision,
		tags: []*generate.Tag{tagA, tagB}, tagRevisions: []*generate.TagRevision{tagRevisionA, tagRevisionB},
		links: links, elementRevisions: []*generate.ElementRevision{elementA, elementB},
	}
}

func (fixture taxonomyCatalogFixture) facts() entity.RewardFacts {
	return entity.RewardFacts{
		Difficulty: entity.DifficultyRewardFact{ID: fixture.difficulty.ID, Revision: fixture.difficulty.Version, BaseExperience: fixture.difficulty.ExpReward},
		Tags:       []entity.TaxonomyRevisionFact{{ID: fixture.tags[0].ID, Revision: fixture.tags[0].Version}, {ID: fixture.tags[1].ID, Revision: fixture.tags[1].Version}},
		Elements:   []entity.TaxonomyRevisionFact{{ID: fixture.elementRevisions[0].ElementID, Revision: fixture.elementRevisions[0].Revision}, {ID: fixture.elementRevisions[1].ElementID, Revision: fixture.elementRevisions[1].Revision}},
	}
}

func newTaxonomyCatalogMock(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database))), mock
}

func expectHistoricalTaxonomyRead(mock sqlmock.Sqlmock, fixture taxonomyCatalogFixture) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
	mock.ExpectQuery(`SELECT .* FROM "tag_revisions"`).WillReturnRows(tagRevisionRows(fixture.tagRevisions[1], fixture.tagRevisions[0]))
	mock.ExpectQuery(`SELECT .* FROM "tag_revision_elements"`).WillReturnRows(tagRevisionElementRows(fixture.links...))
	mock.ExpectQuery(`SELECT .* FROM "element_revisions"`).WillReturnRows(elementRevisionRows(fixture.elementRevisions[1], fixture.elementRevisions[0]))
}

func expectActiveTaxonomyPrefix(mock sqlmock.Sqlmock, fixture taxonomyCatalogFixture) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulties".*"deleted_at" IS NULL.*FOR UPDATE`).
		WillReturnRows(difficultyRows(fixture.difficulty))
	mock.ExpectQuery(`SELECT .* FROM "difficulty_revisions"`).
		WillReturnRows(difficultyRevisionRows(fixture.difficultyRevision))
	mock.ExpectQuery(`SELECT .* FROM "tags".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).
		WillReturnRows(tagRows(fixture.tags...))
}

func resolveTaxonomyFixture(
	client *generate.Client,
	catalog *TaxonomyCatalog,
	fixture taxonomyCatalogFixture,
) error {
	return platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(ctx context.Context) error {
		_, err := catalog.ResolveRewardFacts(ctx, ports.RewardIntent{
			DifficultyID: fixture.difficulty.ID,
			TagIDs:       []uuid.UUID{fixture.tags[1].ID, fixture.tags[0].ID},
		})
		return err
	})
}

func difficultyRows(records ...*generate.Difficulty) *sqlmock.Rows {
	rows := sqlmock.NewRows(difficulty.Columns)
	for _, record := range records {
		rows.AddRow(record.ID, record.CreatedAt, record.UpdatedAt, record.DeletedAt, record.DeletedBy, record.Name, record.Level, record.ExpReward, record.Description, record.Version)
	}
	return rows
}

func difficultyRevisionRows(records ...*generate.DifficultyRevision) *sqlmock.Rows {
	rows := sqlmock.NewRows(difficultyrevision.Columns)
	for _, record := range records {
		rows.AddRow(record.ID, record.CreatedAt, record.DifficultyID, record.Revision, record.Name, record.Level, record.ExpReward, record.Description, record.Checksum)
	}
	return rows
}

func tagRows(records ...*generate.Tag) *sqlmock.Rows {
	rows := sqlmock.NewRows(tag.Columns)
	for _, record := range records {
		rows.AddRow(record.ID, record.CreatedAt, record.UpdatedAt, record.DeletedAt, record.DeletedBy, record.Name, record.Version)
	}
	return rows
}

func tagRevisionRows(records ...*generate.TagRevision) *sqlmock.Rows {
	rows := sqlmock.NewRows(tagrevision.Columns)
	for _, record := range records {
		rows.AddRow(record.ID, record.CreatedAt, record.TagID, record.Revision, record.Name, record.Checksum)
	}
	return rows
}

func tagRevisionElementRows(records ...*generate.TagRevisionElement) *sqlmock.Rows {
	rows := sqlmock.NewRows(tagrevisionelement.Columns)
	for _, record := range records {
		rows.AddRow(record.ID, record.CreatedAt, record.TagRevisionID, record.ElementID, record.ElementRevision)
	}
	return rows
}

func elementRevisionRows(records ...*generate.ElementRevision) *sqlmock.Rows {
	rows := sqlmock.NewRows(elementrevision.Columns)
	for _, record := range records {
		rows.AddRow(record.ID, record.CreatedAt, record.ElementID, record.Revision, record.Name, record.Code, record.Description, record.Checksum)
	}
	return rows
}

func taxonomyElementRevision(now time.Time, id, elementID uuid.UUID, revision int64, name, code string) *generate.ElementRevision {
	encoder := problemcontract.NewChecksumEncoder("judgify.element-revision.v1")
	encoder.UUID(elementID)
	encoder.Int64(revision)
	encoder.String(name)
	encoder.String(code)
	encoder.String("")
	checksum := encoder.Sum()
	return &generate.ElementRevision{ID: id, CreatedAt: now, ElementID: elementID, Revision: revision, Name: name, Code: code, Checksum: checksum[:]}
}

func taxonomyTagChecksum(record *generate.TagRevision, elements []elementRevisionFact) []byte {
	checksum := tagRevisionChecksum(record.TagID, record.Revision, record.Name, elements)
	return checksum[:]
}

func taxonomyUUID(value byte) uuid.UUID {
	var id uuid.UUID
	id[15] = value
	return id
}

func equalTaxonomyFacts(got, want []entity.TaxonomyRevisionFact) bool {
	return len(got) == len(want) && bytes.Equal(flattenTaxonomyFacts(got), flattenTaxonomyFacts(want))
}

func flattenTaxonomyFacts(facts []entity.TaxonomyRevisionFact) []byte {
	result := make([]byte, 0, len(facts)*24)
	for _, fact := range facts {
		result = append(result, fact.ID[:]...)
		result = append(result, byte(fact.Revision>>56), byte(fact.Revision>>48), byte(fact.Revision>>40), byte(fact.Revision>>32), byte(fact.Revision>>24), byte(fact.Revision>>16), byte(fact.Revision>>8), byte(fact.Revision))
	}
	return result
}

func cloneTaxonomyRewardSnapshot(snapshot entity.RewardSnapshot) entity.RewardSnapshot {
	clone := snapshot
	clone.Tags.Items = append([]entity.TagSnapshotItem(nil), snapshot.Tags.Items...)
	if snapshot.Elements != nil {
		elements := *snapshot.Elements
		elements.Items = append([]entity.ElementSnapshotItem(nil), snapshot.Elements.Items...)
		clone.Elements = &elements
	}
	return clone
}
