package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/problem"
	"github.com/huynhanx03/judgify/internal/ent/generate/problemrevision"
	"github.com/huynhanx03/judgify/internal/ent/generate/testasset"
	"github.com/huynhanx03/judgify/internal/ent/generate/testcase"
	"github.com/huynhanx03/judgify/internal/ent/generate/testcaserevision"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestAggregateFromRecordRequiresAtomicRenderSelections(t *testing.T) {
	t.Parallel()

	t.Run("all null is a valid draft without a selection", func(t *testing.T) {
		record := validAggregateRecord()
		aggregate, err := aggregateFromRecord(record)
		if err != nil {
			t.Fatalf("aggregateFromRecord() error = %v", err)
		}
		if aggregate.Draft != nil || aggregate.Published != nil {
			t.Fatalf("selections = draft:%#v published:%#v, want both nil", aggregate.Draft, aggregate.Published)
		}
	})

	t.Run("Ent SQL NULL byte slice is still an all-null selection", func(t *testing.T) {
		record := validAggregateRecord()
		var scannedNull []byte
		record.DraftRenderSourceChecksum = &scannedNull

		aggregate, err := aggregateFromRecord(record)
		if err != nil {
			t.Fatalf("aggregateFromRecord() error = %v", err)
		}
		if aggregate.Draft != nil {
			t.Fatalf("draft = %#v, want nil", aggregate.Draft)
		}
	})

	t.Run("all present is mapped and does not alias checksum bytes", func(t *testing.T) {
		record := validAggregateRecord()
		selection := validDBSelection()
		setDraftSelection(record, selection)

		aggregate, err := aggregateFromRecord(record)
		if err != nil {
			t.Fatalf("aggregateFromRecord() error = %v", err)
		}
		if aggregate.Draft == nil || *aggregate.Draft != selection {
			t.Fatalf("draft = %#v, want %#v", aggregate.Draft, selection)
		}
		(*record.DraftRenderSourceChecksum)[0] ^= 0xff
		if aggregate.Draft.SourceChecksum != selection.SourceChecksum {
			t.Fatal("mapped draft checksum aliases the Ent record")
		}
	})

	t.Run("every partial tuple is rejected", func(t *testing.T) {
		mutations := []struct {
			name   string
			mutate func(*generate.Problem)
		}{
			{"missing revision", func(record *generate.Problem) { record.DraftRevisionID = nil }},
			{"missing source", func(record *generate.Problem) { record.DraftRenderSourceID = nil }},
			{"missing kind", func(record *generate.Problem) { record.DraftRenderSourceKind = nil }},
			{"missing checksum", func(record *generate.Problem) { record.DraftRenderSourceChecksum = nil }},
			{"missing artifact", func(record *generate.Problem) { record.DraftRenderArtifactID = nil }},
		}
		for _, test := range mutations {
			test := test
			t.Run(test.name, func(t *testing.T) {
				record := validAggregateRecord()
				setDraftSelection(record, validDBSelection())
				test.mutate(record)
				_, err := aggregateFromRecord(record)
				if !errors.Is(err, entity.ErrInvalidRenderSelection) {
					t.Fatalf("aggregateFromRecord() error = %v, want ErrInvalidRenderSelection", err)
				}
			})
		}
	})
}

func TestAggregateFromRecordClonesLifecycleTimestamps(t *testing.T) {
	t.Parallel()

	record := validAggregateRecord()
	selection := validDBSelection()
	setDraftSelection(record, selection)
	setPublishedSelection(record, selection)
	publishedAt := task2DBTime().Add(time.Hour)
	wantPublishedAt := publishedAt
	record.Lifecycle = problem.LifecyclePublished
	record.PublishedAt = &publishedAt
	record.IsPublished = true

	aggregate, err := aggregateFromRecord(record)
	if err != nil {
		t.Fatalf("aggregateFromRecord() error = %v", err)
	}
	*record.PublishedAt = record.PublishedAt.Add(24 * time.Hour)
	if aggregate.PublishedAt == nil || !aggregate.PublishedAt.Equal(wantPublishedAt) {
		t.Fatalf("published_at = %v, want owned value %v", aggregate.PublishedAt, wantPublishedAt)
	}
}

func TestAggregateFromRecordMapsStableProblemMappingIdentities(t *testing.T) {
	t.Parallel()

	record := validAggregateRecord()
	aggregate, err := aggregateFromRecord(record)
	if err != nil {
		t.Fatalf("aggregateFromRecord() error = %v", err)
	}
	if aggregate.TagMappingID != record.TagMappingID {
		t.Fatalf("tag mapping ID = %s, want %s", aggregate.TagMappingID, record.TagMappingID)
	}
	if aggregate.ElementMappingID != record.ElementMappingID {
		t.Fatalf("element mapping ID = %s, want %s", aggregate.ElementMappingID, record.ElementMappingID)
	}
}

func TestRevisionFromRecordRejectsCorruptJSONTaxonomyAndChecksums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*generate.ProblemRevision)
		want   error
	}{
		{
			name: "tag json has unknown field",
			mutate: func(record *generate.ProblemRevision) {
				record.TagSnapshot = json.RawMessage(
					`{"mapping_id":"00000000-0000-0000-0000-000000000065","revision":1,"items":[],"unexpected":true}`,
				)
			},
			want: entity.ErrInvalidProblemRevision,
		},
		{
			name: "tag json has trailing value",
			mutate: func(record *generate.ProblemRevision) {
				record.TagSnapshot = append(record.TagSnapshot, []byte(` {}`)...)
			},
			want: entity.ErrInvalidProblemRevision,
		},
		{
			name: "tag mapping identity disagrees with columns",
			mutate: func(record *generate.ProblemRevision) {
				record.TagMappingID = task2DBUUID(9_001)
			},
			want: entity.ErrInvalidProblemRevision,
		},
		{
			name: "element mapping identity disagrees with columns",
			mutate: func(record *generate.ProblemRevision) {
				other := task2DBUUID(9_002)
				record.ElementMappingID = &other
			},
			want: entity.ErrInvalidProblemRevision,
		},
		{
			name: "element tuple is only partially populated",
			mutate: func(record *generate.ProblemRevision) {
				record.ElementMappingRevision = nil
			},
			want: entity.ErrInvalidProblemRevision,
		},
		{
			name: "content checksum has invalid width",
			mutate: func(record *generate.ProblemRevision) {
				record.ContentChecksum = record.ContentChecksum[:31]
			},
			want: entity.ErrInvalidProblemRevision,
		},
		{
			name: "content checksum disagrees with immutable facts",
			mutate: func(record *generate.ProblemRevision) {
				record.ContentChecksum[0] ^= 0xff
			},
			want: entity.ErrRevisionChecksum,
		},
		{
			name: "reward checksum disagrees with taxonomy",
			mutate: func(record *generate.ProblemRevision) {
				record.RewardSnapshotChecksum[0] ^= 0xff
			},
			want: entity.ErrRewardChecksum,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			record := validRevisionRecord(t)
			test.mutate(record)
			_, err := revisionFromRecord(record)
			if !errors.Is(err, test.want) {
				t.Fatalf("revisionFromRecord() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRevisionFromRecordReturnsOwnedMutableValues(t *testing.T) {
	t.Parallel()

	record := validRevisionRecord(t)
	revision, err := revisionFromRecord(record)
	if err != nil {
		t.Fatalf("revisionFromRecord() error = %v", err)
	}
	wantConfig := append([]byte(nil), revision.CheckerConfig...)
	wantRuntime := revision.AllowedRuntimeKeys[0]
	wantTagID := revision.Reward.Tags.Items[0].TagID

	record.CheckerConfig[0] = '['
	record.AllowedRuntimeKeys[0] = "corrupted"
	record.TagSnapshot[0] = '['

	if string(revision.CheckerConfig) != string(wantConfig) {
		t.Fatal("checker config aliases the Ent record")
	}
	if revision.AllowedRuntimeKeys[0] != wantRuntime {
		t.Fatal("runtime keys alias the Ent record")
	}
	if revision.Reward.Tags.Items[0].TagID != wantTagID {
		t.Fatal("decoded taxonomy unexpectedly aliases encoded JSON")
	}
}

func TestRevisionFromRecordCanonicalizesSemanticallyEquivalentJSONB(t *testing.T) {
	t.Parallel()

	record := validRevisionRecord(t)
	// PostgreSQL JSONB may reformat object whitespace while preserving its
	// semantic value. The persistence boundary must restore the canonical byte
	// representation required by Revision.Validate and its content checksum.
	record.CheckerConfig = json.RawMessage(`{"mode":"exact", "trim":true}`)
	revision, err := revisionFromRecord(record)
	if err != nil {
		t.Fatalf("revisionFromRecord() error = %v", err)
	}
	if got, want := string(revision.CheckerConfig), `{"mode":"exact","trim":true}`; got != want {
		t.Fatalf("canonical checker config = %q, want %q", got, want)
	}
}

func TestTestsetFromRecordRequiresCompleteOrderedOwnedEdges(t *testing.T) {
	t.Parallel()

	t.Run("maps hidden assets and owns their bytes", func(t *testing.T) {
		record := validTestsetRecord(t)
		testset, err := testsetFromRecord(record)
		if err != nil {
			t.Fatalf("testsetFromRecord() error = %v", err)
		}
		hidden := testset.Groups[1].Cases[0]
		if hidden.Kind != problemcontract.TestKindHidden ||
			string(hidden.InputAsset.Content) != "99 1\n" ||
			string(hidden.ExpectedAsset.Content) != "100\n" {
			t.Fatalf("hidden testcase mapped incorrectly: %#v", hidden)
		}
		record.Edges.Groups[1].Edges.TestcaseRevisions[0].Edges.InputAsset.Content[0] = 'X'
		if string(hidden.InputAsset.Content) != "99 1\n" {
			t.Fatal("mapped hidden input aliases the Ent edge")
		}
	})

	tests := []struct {
		name   string
		mutate func(*generate.TestsetRevision)
		want   error
	}{
		{
			name: "nil group edge",
			mutate: func(record *generate.TestsetRevision) {
				record.Edges.Groups[0] = nil
			},
			want: entity.ErrInvalidTestsetRevision,
		},
		{
			name: "groups are not eagerly ordered",
			mutate: func(record *generate.TestsetRevision) {
				record.Edges.Groups[0], record.Edges.Groups[1] =
					record.Edges.Groups[1], record.Edges.Groups[0]
			},
			want: entity.ErrNonContiguousTests,
		},
		{
			name: "cases are not eagerly ordered",
			mutate: func(record *generate.TestsetRevision) {
				first := record.Edges.Groups[0].Edges.TestcaseRevisions[0]
				second := cloneGeneratedCase(first)
				second.ID = task2DBUUID(8_101)
				second.TestCaseID = task2DBUUID(8_102)
				second.OrderIndex = 1
				record.Edges.Groups[0].Edges.TestcaseRevisions =
					[]*generate.TestcaseRevision{second, first}
				record.TestCount++
				record.TotalUncompressedBytes +=
					second.Edges.InputAsset.UncompressedSize +
						second.Edges.ExpectedAsset.UncompressedSize
			},
			want: entity.ErrNonContiguousTests,
		},
		{
			name: "input asset edge is absent",
			mutate: func(record *generate.TestsetRevision) {
				record.Edges.Groups[0].Edges.TestcaseRevisions[0].Edges.InputAsset = nil
			},
			want: entity.ErrInvalidTestsetRevision,
		},
		{
			name: "expected asset edge is absent",
			mutate: func(record *generate.TestsetRevision) {
				record.Edges.Groups[0].Edges.TestcaseRevisions[0].Edges.ExpectedAsset = nil
			},
			want: entity.ErrInvalidTestsetRevision,
		},
		{
			name: "hidden kind is lost",
			mutate: func(record *generate.TestsetRevision) {
				record.Edges.Groups[1].Edges.TestcaseRevisions[0].Kind =
					testcaserevision.KindSample
			},
			want: entity.ErrHiddenTestRequired,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			record := validTestsetRecord(t)
			test.mutate(record)
			_, err := testsetFromRecord(record)
			if !errors.Is(err, test.want) {
				t.Fatalf("testsetFromRecord() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestTestsetFromRecordRejectsMismatchedEdgeIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*generate.TestcaseRevision)
	}{
		{
			name: "case points at another group",
			mutate: func(record *generate.TestcaseRevision) {
				record.GroupID = task2DBUUID(8_201)
			},
		},
		{
			name: "input edge is not the declared input asset",
			mutate: func(record *generate.TestcaseRevision) {
				record.InputAssetID = task2DBUUID(8_202)
			},
		},
		{
			name: "expected edge is not the declared expected asset",
			mutate: func(record *generate.TestcaseRevision) {
				record.ExpectedAssetID = task2DBUUID(8_203)
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			record := validTestsetRecord(t)
			test.mutate(record.Edges.Groups[0].Edges.TestcaseRevisions[0])
			_, err := testsetFromRecord(record)
			if !errors.Is(err, entity.ErrInvalidTestsetRevision) {
				t.Fatalf("testsetFromRecord() error = %v, want ErrInvalidTestsetRevision", err)
			}
		})
	}
}

func TestRevisionRepositoryOperationsRequireTransactionScope(t *testing.T) {
	t.Parallel()

	repository := &RevisionRepository{
		client:        generate.NewClient(),
		content:       inertContentRepository{},
		sampleDecoder: entity.IdentityAssetDecoder{},
	}
	ctx := context.Background()
	problemID := task2DBUUID(7_001)

	if _, err := repository.LockAggregate(ctx, problemID); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("LockAggregate() error = %v, want ErrTransactionRequired", err)
	}
	if _, err := repository.NextRevisionNumbers(ctx, problemID); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("NextRevisionNumbers() error = %v, want ErrTransactionRequired", err)
	}
	if _, err := repository.ResolveAssets(ctx, []entity.TestAsset{
		identityAsset(7_002, []byte("1 2\n")),
	}); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("ResolveAssets() error = %v, want ErrTransactionRequired", err)
	}
	if err := repository.StoreTestset(ctx, testsetEntityFromRecord(t, validTestsetRecord(t))); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("StoreTestset() error = %v, want ErrTransactionRequired", err)
	}
	if err := repository.StoreRevision(ctx, revisionEntityFromRecord(t, validRevisionRecord(t))); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("StoreRevision() error = %v, want ErrTransactionRequired", err)
	}
	if _, err := repository.AdvanceDraft(
		ctx,
		problemID,
		1,
		validDBSelection(),
		task2DBTime(),
	); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("AdvanceDraft() error = %v, want ErrTransactionRequired", err)
	}
	if _, err := repository.AdvancePublished(
		ctx,
		problemID,
		1,
		validDBSelection(),
		validPublishedProblemProjection(),
		task2DBTime(),
	); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("AdvancePublished() error = %v, want ErrTransactionRequired", err)
	}
	if _, err := repository.Archive(
		ctx,
		problemID,
		1,
		task2DBTime(),
	); !errors.Is(err, platformtx.ErrTransactionRequired) {
		t.Fatalf("Archive() error = %v, want ErrTransactionRequired", err)
	}
}

func TestNewRevisionRepositoryRejectsTypedNilContentRepository(t *testing.T) {
	t.Parallel()

	var content *contentrender.EntRepository
	repository, err := NewRevisionRepository(generate.NewClient(), content)
	if repository != nil {
		t.Fatalf("NewRevisionRepository() repository = %#v, want nil", repository)
	}
	if !errors.Is(err, ErrInvalidRevisionRepository) {
		t.Fatalf(
			"NewRevisionRepository() error = %v, want ErrInvalidRevisionRepository",
			err,
		)
	}
}

func revisionEntityFromRecord(
	t *testing.T,
	record *generate.ProblemRevision,
) entity.Revision {
	t.Helper()
	value, err := revisionFromRecord(record)
	if err != nil {
		t.Fatalf("revisionFromRecord() error = %v", err)
	}
	return value
}

func testsetEntityFromRecord(
	t *testing.T,
	record *generate.TestsetRevision,
) entity.TestsetRevision {
	t.Helper()
	value, err := testsetFromRecord(record)
	if err != nil {
		t.Fatalf("testsetFromRecord() error = %v", err)
	}
	return value
}

func TestRevisionRepositoryLockAggregateUsesForUpdate(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	problemID := task2DBUUID(7_101)
	record := validAggregateRecordWithID(problemID)
	selection := validDBSelection()
	setDraftSelection(record, selection)
	setPublishedSelection(record, selection)
	publishedAt := task2DBTime().Add(time.Hour)
	record.Lifecycle = problem.LifecyclePublished
	record.PublishedAt = &publishedAt
	record.IsPublished = true

	mock.ExpectBegin()
	mock.ExpectQuery("lock").
		WillReturnRows(problemSQLRows(record))
	mock.ExpectCommit()

	var aggregate entity.ProblemAggregate
	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			var loadErr error
			aggregate, loadErr = repository.LockAggregate(ctx, problemID)
			return loadErr
		},
	)
	if err != nil {
		t.Fatalf("LockAggregate() error = %v", err)
	}
	if aggregate.ID != problemID {
		t.Fatalf("aggregate id = %s, want %s", aggregate.ID, problemID)
	}
	if !strings.Contains(strings.ToUpper(capture.statement("lock")), "FOR UPDATE") {
		t.Fatalf("lock query does not contain FOR UPDATE: %s", capture.statement("lock"))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryAdvancePublishedUsesSingleCompareAndSwapUpdate(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	problemID := task2DBUUID(7_201)
	selection := validDBSelection()
	publishedAt := task2DBTime().Add(time.Hour)
	const expectedVersion int64 = 4
	reloaded := validAggregateRecordWithID(problemID)
	setDraftSelection(reloaded, selection)
	setPublishedSelection(reloaded, selection)
	reloaded.Lifecycle = problem.LifecyclePublished
	reloaded.Version = expectedVersion + 1
	reloaded.PublishedAt = &publishedAt
	reloaded.IsPublished = true

	mock.ExpectBegin()
	mock.ExpectQuery("advance_selector").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(problemID.String()),
	)
	mock.ExpectExec("advance_update").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("advance_tags_clear").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("advance_tags_add").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("advance_reload").WillReturnRows(problemSQLRows(reloaded))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			_, advanceErr := repository.AdvancePublished(
				ctx,
				problemID,
				expectedVersion,
				selection,
				validPublishedProblemProjection(),
				publishedAt,
			)
			return advanceErr
		},
	)
	if err != nil {
		t.Fatalf("AdvancePublished() error = %v", err)
	}
	where := sqlWhereClause(t, capture.statement("advance_selector"))
	for _, predicate := range []string{
		`"id"`,
		`"version"`,
		`"lifecycle"`,
		`"draft_revision_id"`,
		`"draft_render_source_id"`,
		`"draft_render_source_kind"`,
		`"draft_render_source_checksum"`,
		`"draft_render_artifact_id"`,
	} {
		if !strings.Contains(where, predicate) {
			t.Errorf("advance CAS WHERE is missing %s: %s", predicate, where)
		}
	}
	if strings.Count(strings.ToUpper(capture.statement("advance_update")), "UPDATE ") != 1 {
		t.Fatalf("advance emitted an unexpected update shape: %s", capture.statement("advance_update"))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryAdvanceDraftUsesSingleFiveColumnCompareAndSwap(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	problemID := task2DBUUID(7_251)
	selection := validDBSelection()
	savedAt := task2DBTime().Add(time.Hour)
	const expectedVersion int64 = 4
	reloaded := validAggregateRecordWithID(problemID)
	setDraftSelection(reloaded, selection)
	reloaded.Version = expectedVersion + 1
	reloaded.UpdatedAt = savedAt

	mock.ExpectBegin()
	mock.ExpectExec("draft_advance").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("draft_reload").WillReturnRows(problemSQLRows(reloaded))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			_, advanceErr := repository.AdvanceDraft(
				ctx,
				problemID,
				expectedVersion,
				selection,
				savedAt,
			)
			return advanceErr
		},
	)
	if err != nil {
		t.Fatalf("AdvanceDraft() error = %v", err)
	}
	statement := capture.statement("draft_advance")
	where := sqlWhereClause(t, statement)
	for _, predicate := range []string{
		`"id"`,
		`"version"`,
		`"lifecycle"`,
	} {
		if !strings.Contains(where, predicate) {
			t.Errorf("draft CAS WHERE is missing %s: %s", predicate, where)
		}
	}
	for _, column := range []string{
		`"draft_revision_id"`,
		`"draft_render_source_id"`,
		`"draft_render_source_kind"`,
		`"draft_render_source_checksum"`,
		`"draft_render_artifact_id"`,
		`"version"`,
	} {
		if !strings.Contains(statement, column) {
			t.Errorf("draft CAS update is missing %s: %s", column, statement)
		}
	}
	for _, forbidden := range []string{
		`"published_revision_id" =`,
		`"published_render_artifact_id" =`,
		`"lifecycle" =`,
	} {
		if strings.Contains(statement, forbidden) {
			t.Errorf("draft save mutates publication state through %s: %s", forbidden, statement)
		}
	}
	if strings.Count(strings.ToUpper(statement), "UPDATE ") != 1 {
		t.Fatalf("draft advance emitted an unexpected update shape: %s", statement)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryNextRevisionNumbersCountsAppendOnlyHistoryUnderLock(t *testing.T) {
	client, mock, _ := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	problemID := task2DBUUID(7_261)

	mock.ExpectBegin()
	mock.ExpectQuery("problem_revision_count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))
	mock.ExpectQuery("testset_revision_count").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	mock.ExpectCommit()

	var numbers ports.DraftRevisionNumbers
	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			var numberErr error
			numbers, numberErr = repository.NextRevisionNumbers(ctx, problemID)
			return numberErr
		},
	)
	if err != nil {
		t.Fatalf("NextRevisionNumbers() error = %v", err)
	}
	if numbers != (ports.DraftRevisionNumbers{
		ProblemRevision: 9,
		TestsetRevision: 12,
	}) {
		t.Fatalf("revision numbers = %#v, want problem=9 testset=12", numbers)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryResolveAssetsUsesInsertDoNothingThenVerifiesCanonicalRow(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	candidate := identityAsset(7_271, []byte("shared input\n"))
	persisted := candidate
	persisted.ID = task2DBUUID(7_272)
	persisted.CreatedAt = candidate.CreatedAt.Add(-time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery("asset_insert").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("asset_load").
		WillReturnRows(testAssetSQLRows(persisted))
	mock.ExpectCommit()

	var resolved []entity.TestAsset
	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			var resolveErr error
			resolved, resolveErr = repository.ResolveAssets(
				ctx,
				[]entity.TestAsset{candidate},
			)
			return resolveErr
		},
	)
	if err != nil {
		t.Fatalf("ResolveAssets() error = %v", err)
	}
	if len(resolved) != 1 || resolved[0].ID != persisted.ID ||
		resolved[0].SHA256 != candidate.SHA256 ||
		!bytes.Equal(resolved[0].Content, candidate.Content) {
		t.Fatalf("resolved assets = %#v, want canonical persisted row %#v", resolved, persisted)
	}
	resolved[0].Content[0] = 'X'
	if candidate.Content[0] == 'X' {
		t.Fatal("resolved asset aliases candidate content")
	}
	insert := strings.ToUpper(capture.statement("asset_insert"))
	if !strings.Contains(insert, "ON CONFLICT") ||
		!strings.Contains(insert, "DO NOTHING") ||
		strings.Contains(insert, "DO UPDATE") {
		t.Fatalf("asset insert is not append-only conflict-safe SQL: %s", insert)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryStoreRevisionInsertsCompleteImmutableSnapshot(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	revision := revisionEntityFromRecord(t, validRevisionRecord(t))
	owner := validAggregateRecordWithID(revision.ProblemID)
	owner.TagMappingID = revision.Reward.Tags.MappingID
	owner.ElementMappingID = revision.Reward.Elements.MappingID

	mock.ExpectBegin()
	mock.ExpectQuery("revision_mapping_owner").
		WillReturnRows(problemSQLRows(owner))
	mock.ExpectExec("revision_insert").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			return repository.StoreRevision(ctx, revision)
		},
	)
	if err != nil {
		t.Fatalf("StoreRevision() error = %v", err)
	}
	statement := capture.statement("revision_insert")
	if !strings.Contains(strings.ToUpper(statement), `INSERT INTO "PROBLEM_REVISIONS"`) {
		t.Fatalf("revision persistence is not an insert: %s", statement)
	}
	for _, column := range []string{
		`"problem_id"`,
		`"revision_number"`,
		`"statement_markdown"`,
		`"render_source_id"`,
		`"render_source_checksum"`,
		`"reward_snapshot_checksum"`,
		`"checker_config"`,
		`"testset_revision_id"`,
		`"allowed_runtime_keys"`,
		`"content_checksum"`,
	} {
		if !strings.Contains(statement, column) {
			t.Errorf("immutable revision insert is missing %s: %s", column, statement)
		}
	}
	if strings.Contains(strings.ToUpper(statement), "UPDATE ") {
		t.Fatalf("revision persistence emitted an update: %s", statement)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestVerifyAggregateRewardMappingOwnershipRejectsForgedProblemMapping(t *testing.T) {
	t.Parallel()

	revision := revisionEntityFromRecord(t, validRevisionRecord(t))
	aggregate := entity.ProblemAggregate{
		ID:               revision.ProblemID,
		TagMappingID:     revision.Reward.Tags.MappingID,
		ElementMappingID: revision.Reward.Elements.MappingID,
	}
	if err := verifyAggregateRewardMappingOwnership(aggregate, revision.Reward); err != nil {
		t.Fatalf("verifyAggregateRewardMappingOwnership() error = %v", err)
	}
	for _, mutate := range []struct {
		name string
		fn   func(*entity.RewardSnapshot)
	}{
		{
			name: "tag mapping",
			fn: func(reward *entity.RewardSnapshot) {
				reward.Tags.MappingID = task2DBUUID(99_001)
			},
		},
		{
			name: "element mapping",
			fn: func(reward *entity.RewardSnapshot) {
				reward.Elements.MappingID = task2DBUUID(99_002)
			},
		},
	} {
		mutation := mutate
		t.Run(mutation.name, func(t *testing.T) {
			reward := revision.Reward
			mutation.fn(&reward)
			if err := verifyAggregateRewardMappingOwnership(aggregate, reward); !errors.Is(err, entity.ErrInvalidProblemRevision) {
				t.Fatalf("verifyAggregateRewardMappingOwnership() error = %v, want ErrInvalidProblemRevision", err)
			}
		})
	}
}

func TestVerifyRevisionRewardMappingOwnershipRejectsForgedMappingRevisionAndDefinesNoElementCase(t *testing.T) {
	t.Parallel()

	revision := revisionEntityFromRecord(t, validRevisionRecord(t))
	aggregate := entity.ProblemAggregate{
		ID:               revision.ProblemID,
		TagMappingID:     revision.Reward.Tags.MappingID,
		ElementMappingID: revision.Reward.Elements.MappingID,
	}
	if err := verifyRevisionRewardMappingOwnership(aggregate, revision); err != nil {
		t.Fatalf("verifyRevisionRewardMappingOwnership() error = %v", err)
	}

	for _, mutate := range []struct {
		name string
		fn   func(*entity.Revision)
	}{
		{
			name: "tag mapping revision",
			fn: func(value *entity.Revision) {
				value.Reward.Tags.Revision++
			},
		},
		{
			name: "element mapping revision",
			fn: func(value *entity.Revision) {
				value.Reward.Elements.Revision++
			},
		},
	} {
		mutation := mutate
		t.Run(mutation.name, func(t *testing.T) {
			forged := revision
			elements := *revision.Reward.Elements
			elements.Items = append([]entity.ElementSnapshotItem(nil), revision.Reward.Elements.Items...)
			forged.Reward.Elements = &elements
			mutation.fn(&forged)
			if err := verifyRevisionRewardMappingOwnership(aggregate, forged); !errors.Is(err, entity.ErrInvalidProblemRevision) {
				t.Fatalf("verifyRevisionRewardMappingOwnership() error = %v, want ErrInvalidProblemRevision", err)
			}
		})
	}

	withoutElements := revision
	withoutElements.Reward.Elements = nil
	// An empty element union serializes no element mapping identity. There is
	// therefore nothing caller-controlled to compare; stable tag ownership and
	// mapping revision remain mandatory.
	aggregate.ElementMappingID = task2DBUUID(99_003)
	if err := verifyRevisionRewardMappingOwnership(aggregate, withoutElements); err != nil {
		t.Fatalf("verifyRevisionRewardMappingOwnership(no elements) error = %v", err)
	}
	withoutElements.Reward.Tags.MappingID = task2DBUUID(99_004)
	if err := verifyRevisionRewardMappingOwnership(aggregate, withoutElements); !errors.Is(err, entity.ErrInvalidProblemRevision) {
		t.Fatalf("verifyRevisionRewardMappingOwnership(no elements, forged tag) error = %v, want ErrInvalidProblemRevision", err)
	}
}

func TestRevisionRepositoryStoreTestsetInsertsLineageAndImmutableGraphInFKOrder(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	testset := testsetEntityFromRecord(t, validTestsetRecord(t))

	mock.ExpectBegin()
	for index, group := range testset.Groups {
		for caseIndex, item := range group.Cases {
			label := fmt.Sprintf("lineage_insert_%d_%d", index, caseIndex)
			mock.ExpectQuery(label).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(item.TestCaseID.String()))
			mock.ExpectQuery(fmt.Sprintf("lineage_load_%d_%d", index, caseIndex)).
				WillReturnRows(testCaseSQLRows(item.TestCaseID, testset.ProblemID, testset.CreatedAt))
		}
	}
	mock.ExpectExec("testset_insert").
		WillReturnResult(sqlmock.NewResult(0, 1))
	groupRows := sqlmock.NewRows([]string{"id"})
	for _, group := range testset.Groups {
		groupRows.AddRow(group.ID.String())
	}
	mock.ExpectQuery("group_bulk_insert").WillReturnRows(groupRows)
	caseRows := sqlmock.NewRows([]string{"id"})
	for _, group := range testset.Groups {
		for _, item := range group.Cases {
			caseRows.AddRow(item.ID.String())
		}
	}
	mock.ExpectQuery("case_bulk_insert").WillReturnRows(caseRows)
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			return repository.StoreTestset(ctx, testset)
		},
	)
	if err != nil {
		t.Fatalf("StoreTestset() error = %v", err)
	}
	for label, statement := range capture.statements {
		upper := strings.ToUpper(statement)
		if strings.Contains(upper, "UPDATE ") {
			t.Errorf("%s unexpectedly mutates historical state: %s", label, statement)
		}
		if strings.HasPrefix(label, "lineage_insert_") &&
			(!strings.Contains(upper, "ON CONFLICT") ||
				!strings.Contains(upper, "DO NOTHING")) {
			t.Errorf("%s is not collision-safe: %s", label, statement)
		}
	}
	if !strings.Contains(
		strings.ToUpper(capture.statement("testset_insert")),
		`INSERT INTO "TESTSET_REVISIONS"`,
	) {
		t.Fatalf("testset revision was not inserted: %s", capture.statement("testset_insert"))
	}
	if !strings.Contains(
		strings.ToUpper(capture.statement("group_bulk_insert")),
		`INSERT INTO "TESTCASE_GROUPS"`,
	) {
		t.Fatalf("testcase groups were not inserted in bulk: %s", capture.statement("group_bulk_insert"))
	}
	if !strings.Contains(
		strings.ToUpper(capture.statement("case_bulk_insert")),
		`INSERT INTO "TESTCASE_REVISIONS"`,
	) {
		t.Fatalf("testcase revisions were not inserted in bulk: %s", capture.statement("case_bulk_insert"))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryArchiveUsesSingleCompareAndSwapUpdate(t *testing.T) {
	client, mock, capture := newRevisionSQLMockClient(t)
	repository := &RevisionRepository{client: client, content: inertContentRepository{}, sampleDecoder: entity.IdentityAssetDecoder{}}
	problemID := task2DBUUID(7_301)
	selection := validDBSelection()
	publishedAt := task2DBTime().Add(time.Hour)
	archivedAt := publishedAt.Add(time.Hour)
	const expectedVersion int64 = 8
	reloaded := validAggregateRecordWithID(problemID)
	setDraftSelection(reloaded, selection)
	setPublishedSelection(reloaded, selection)
	reloaded.Lifecycle = problem.LifecycleArchived
	reloaded.Version = expectedVersion + 1
	reloaded.PublishedAt = &publishedAt
	reloaded.ArchivedAt = &archivedAt

	mock.ExpectBegin()
	mock.ExpectExec("archive").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("archive_reload").WillReturnRows(problemSQLRows(reloaded))
	mock.ExpectCommit()

	err := platformtx.NewEntUnitOfWork(client).Within(
		context.Background(),
		func(ctx context.Context) error {
			_, archiveErr := repository.Archive(
				ctx,
				problemID,
				expectedVersion,
				archivedAt,
			)
			return archiveErr
		},
	)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	where := sqlWhereClause(t, capture.statement("archive"))
	for _, predicate := range []string{
		`"id"`,
		`"version"`,
		`"lifecycle"`,
		`"published_revision_id" IS NOT NULL`,
	} {
		if !strings.Contains(where, predicate) {
			t.Errorf("archive CAS WHERE is missing %s: %s", predicate, where)
		}
	}
	if strings.Count(strings.ToUpper(capture.statement("archive")), "UPDATE ") != 1 {
		t.Fatalf("archive emitted an unexpected update shape: %s", capture.statement("archive"))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRevisionRepositoryCompareAndSwapMissReturnsConflictWithoutReload(t *testing.T) {
	tests := []struct {
		name           string
		wantError      error
		usesEdgeSelect bool
		run            func(context.Context, *RevisionRepository) error
	}{
		{
			name:      "advance draft",
			wantError: ports.ErrDraftConflict,
			run: func(ctx context.Context, repository *RevisionRepository) error {
				_, err := repository.AdvanceDraft(
					ctx,
					task2DBUUID(7_400),
					3,
					validDBSelection(),
					task2DBTime(),
				)
				return err
			},
		},
		{
			name:           "advance published",
			wantError:      ports.ErrPublicationConflict,
			usesEdgeSelect: true,
			run: func(ctx context.Context, repository *RevisionRepository) error {
				_, err := repository.AdvancePublished(
					ctx,
					task2DBUUID(7_401),
					3,
					validDBSelection(),
					validPublishedProblemProjection(),
					task2DBTime(),
				)
				return err
			},
		},
		{
			name:      "archive",
			wantError: ports.ErrPublicationConflict,
			run: func(ctx context.Context, repository *RevisionRepository) error {
				_, err := repository.Archive(
					ctx,
					task2DBUUID(7_402),
					3,
					task2DBTime(),
				)
				return err
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			client, mock, capture := newRevisionSQLMockClient(t)
			repository := &RevisionRepository{
				client:        client,
				content:       inertContentRepository{},
				sampleDecoder: entity.IdentityAssetDecoder{},
			}
			mock.ExpectBegin()
			if test.usesEdgeSelect {
				mock.ExpectQuery(test.name).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			} else {
				mock.ExpectExec(test.name).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			mock.ExpectRollback()

			err := platformtx.NewEntUnitOfWork(client).Within(
				context.Background(),
				func(ctx context.Context) error {
					return test.run(ctx, repository)
				},
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("%s error = %v, want %v", test.name, err, test.wantError)
			}
			statement := strings.ToUpper(capture.statement(test.name))
			if test.usesEdgeSelect {
				if strings.Count(statement, "SELECT ") != 1 {
					t.Fatalf("%s emitted an unexpected selector shape: %s", test.name, capture.statement(test.name))
				}
			} else if strings.Count(statement, "UPDATE ") != 1 {
				t.Fatalf("%s emitted an unexpected update shape: %s", test.name, capture.statement(test.name))
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("SQL expectations: %v", err)
			}
		})
	}
}

func validPublishedProblemProjection() ports.PublishedProblemProjection {
	return ports.PublishedProblemProjection{
		RevisionID:        validDBSelection().RevisionID,
		Title:             "Two Sum",
		StatementMarkdown: "# Two Sum",
		DifficultyID:      task2DBUUID(7_499),
		TagIDs:            []uuid.UUID{task2DBUUID(7_498)},
		CPUTimeMs:         problemcontract.DefaultCPUTimeLimitMilliseconds,
		MemoryLimitKb:     problemcontract.DefaultMemoryLimitKilobytes,
	}
}

func validAggregateRecord() *generate.Problem {
	return validAggregateRecordWithID(task2DBUUID(6_001))
}

func validAggregateRecordWithID(id uuid.UUID) *generate.Problem {
	createdAt := task2DBTime()
	return &generate.Problem{
		ID:               id,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
		Slug:             "sum-two-numbers",
		Lifecycle:        problem.LifecycleDraft,
		Version:          1,
		TagMappingID:     task2DBUUID(6_004),
		ElementMappingID: task2DBUUID(6_005),
		Title:            "legacy compatibility",
		Description:      "legacy compatibility",
		DifficultyID:     task2DBUUID(6_002),
		TimeLimitMs:      1_000,
		MemoryLimitKB:    256 << 10,
		AuthorID:         task2DBUUID(6_003),
		SubmissionCount:  0,
		AcceptedCount:    0,
	}
}

func validDBSelection() entity.RenderSelection {
	return entity.RenderSelection{
		RevisionID:     task2DBUUID(6_101),
		SourceID:       task2DBUUID(6_102),
		SourceKind:     problemcontract.RenderSourceKindProblemStatement,
		SourceChecksum: sha256.Sum256([]byte("render-source")),
		ArtifactID:     task2DBUUID(6_103),
	}
}

func setDraftSelection(record *generate.Problem, selection entity.RenderSelection) {
	kind := problem.DraftRenderSourceKind(selection.SourceKind)
	checksum := append([]byte(nil), selection.SourceChecksum[:]...)
	record.DraftRevisionID = uuidPointer(selection.RevisionID)
	record.DraftRenderSourceID = uuidPointer(selection.SourceID)
	record.DraftRenderSourceKind = &kind
	record.DraftRenderSourceChecksum = &checksum
	record.DraftRenderArtifactID = uuidPointer(selection.ArtifactID)
}

func setPublishedSelection(record *generate.Problem, selection entity.RenderSelection) {
	kind := problem.PublishedRenderSourceKind(selection.SourceKind)
	checksum := append([]byte(nil), selection.SourceChecksum[:]...)
	record.PublishedRevisionID = uuidPointer(selection.RevisionID)
	record.PublishedRenderSourceID = uuidPointer(selection.SourceID)
	record.PublishedRenderSourceKind = &kind
	record.PublishedRenderSourceChecksum = &checksum
	record.PublishedRenderArtifactID = uuidPointer(selection.ArtifactID)
}

func validRevisionRecord(t *testing.T) *generate.ProblemRevision {
	t.Helper()

	statement := []byte("# Sum\n\nCompute the sum.\n")
	normalized, err := contentrender.NormalizeMarkdown(statement)
	if err != nil {
		t.Fatalf("NormalizeMarkdown() error = %v", err)
	}
	tags := entity.TagMappingSnapshot{
		MappingID: task2DBUUID(101),
		Revision:  1,
		Items: []entity.TagSnapshotItem{
			{TagID: task2DBUUID(102), TagRevision: 1},
			{TagID: task2DBUUID(103), TagRevision: 2},
		},
	}
	elements := entity.ElementMappingSnapshot{
		MappingID: task2DBUUID(104),
		Revision:  1,
		Items: []entity.ElementSnapshotItem{
			{ElementID: task2DBUUID(105), ElementRevision: 1, WeightBasisPoints: 4_000},
			{ElementID: task2DBUUID(106), ElementRevision: 1, WeightBasisPoints: 6_000},
		},
	}
	reward := entity.RewardSnapshot{
		DifficultyID:       task2DBUUID(100),
		DifficultyRevision: 1,
		BaseExperience:     100,
		Tags:               tags,
		Elements:           &elements,
	}
	rewardChecksum, err := reward.ComputeChecksum()
	if err != nil {
		t.Fatalf("reward ComputeChecksum() error = %v", err)
	}
	reward.Checksum = rewardChecksum
	checkerConfig, err := entity.CanonicalizeCheckerConfig(
		json.RawMessage(`{"mode":"exact","trim":true}`),
	)
	if err != nil {
		t.Fatalf("CanonicalizeCheckerConfig() error = %v", err)
	}
	revision := entity.Revision{
		ID:                   task2DBUUID(1),
		ProblemID:            task2DBUUID(2),
		RevisionNumber:       1,
		Title:                "Sum",
		StatementMarkdown:    append([]byte(nil), normalized.Bytes...),
		RenderSourceID:       task2DBUUID(3),
		RenderSourceKind:     problemcontract.RenderSourceKindProblemStatement,
		RenderSourceChecksum: normalized.Checksum,
		Reward:               reward,
		Limits: entity.Limits{
			CPUTimeMilliseconds:  1_000,
			WallTimeMilliseconds: 2_000,
			MemoryKilobytes:      256 << 10,
			OutputBytes:          64 << 10,
			ProcessCount:         1,
		},
		CheckerKey:         "exact",
		CheckerVersion:     "v1",
		CheckerConfig:      checkerConfig,
		TestsetRevisionID:  task2DBUUID(4),
		AllowedRuntimeKeys: []string{"go1.25", "python3.13"},
		CreatedBy:          task2DBUUID(5),
		CreatedAt:          task2DBTime(),
	}
	contentChecksum, err := revision.ComputeContentChecksum()
	if err != nil {
		t.Fatalf("ComputeContentChecksum() error = %v", err)
	}
	tagJSON, err := json.Marshal(tags)
	if err != nil {
		t.Fatalf("json.Marshal(tags) error = %v", err)
	}
	elementJSON, err := json.Marshal(elements)
	if err != nil {
		t.Fatalf("json.Marshal(elements) error = %v", err)
	}
	elementMappingRevision := elements.Revision
	createdBy := revision.CreatedBy
	return &generate.ProblemRevision{
		ID:                     revision.ID,
		CreatedAt:              revision.CreatedAt,
		ProblemID:              revision.ProblemID,
		RevisionNumber:         revision.RevisionNumber,
		Title:                  revision.Title,
		StatementMarkdown:      string(revision.StatementMarkdown),
		RenderSourceID:         revision.RenderSourceID,
		RenderSourceKind:       problemrevision.RenderSourceKind(revision.RenderSourceKind),
		RenderSourceChecksum:   append([]byte(nil), revision.RenderSourceChecksum[:]...),
		DifficultyID:           reward.DifficultyID,
		DifficultyRevision:     reward.DifficultyRevision,
		BaseExp:                reward.BaseExperience,
		TagMappingID:           tags.MappingID,
		TagMappingRevision:     tags.Revision,
		TagSnapshot:            tagJSON,
		ElementMappingID:       uuidPointer(elements.MappingID),
		ElementMappingRevision: &elementMappingRevision,
		ElementSnapshot:        elementJSON,
		RewardSnapshotChecksum: append([]byte(nil), reward.Checksum[:]...),
		CPUTimeLimitMs:         revision.Limits.CPUTimeMilliseconds,
		WallTimeLimitMs:        revision.Limits.WallTimeMilliseconds,
		MemoryLimitKB:          revision.Limits.MemoryKilobytes,
		OutputLimitBytes:       revision.Limits.OutputBytes,
		ProcessLimit:           revision.Limits.ProcessCount,
		CheckerKey:             revision.CheckerKey,
		CheckerVersion:         revision.CheckerVersion,
		CheckerConfig:          append(json.RawMessage(nil), revision.CheckerConfig...),
		TestsetRevisionID:      revision.TestsetRevisionID,
		AllowedRuntimeKeys:     append([]string(nil), revision.AllowedRuntimeKeys...),
		CreatedBy:              &createdBy,
		ContentChecksum:        append([]byte(nil), contentChecksum[:]...),
	}
}

func validTestsetRecord(t *testing.T) *generate.TestsetRevision {
	t.Helper()

	createdAt := task2DBTime()
	testset := entity.TestsetRevision{
		ID:             task2DBUUID(10_000),
		ProblemID:      task2DBUUID(10_001),
		RevisionNumber: 1,
		CreatedBy:      task2DBUUID(10_002),
		CreatedAt:      createdAt,
		Groups: []entity.TestcaseGroup{
			{
				ID:                task2DBUUID(10_003),
				TestsetRevisionID: task2DBUUID(10_000),
				GroupIndex:        0,
				CreatedAt:         createdAt,
				Cases: []entity.TestcaseRevision{{
					ID:                task2DBUUID(10_004),
					TestCaseID:        task2DBUUID(10_005),
					TestsetRevisionID: task2DBUUID(10_000),
					ProblemID:         task2DBUUID(10_001),
					Kind:              problemcontract.TestKindSample,
					GroupIndex:        0,
					OrderIndex:        0,
					InputAsset:        identityAsset(10_006, []byte("1 2\n")),
					ExpectedAsset:     identityAsset(10_007, []byte("3\n")),
					Weight:            1,
					CreatedAt:         createdAt,
				}},
			},
			{
				ID:                task2DBUUID(10_008),
				TestsetRevisionID: task2DBUUID(10_000),
				GroupIndex:        1,
				StopOnFailure:     true,
				CreatedAt:         createdAt,
				Cases: []entity.TestcaseRevision{{
					ID:                task2DBUUID(10_009),
					TestCaseID:        task2DBUUID(10_010),
					TestsetRevisionID: task2DBUUID(10_000),
					ProblemID:         task2DBUUID(10_001),
					Kind:              problemcontract.TestKindHidden,
					GroupIndex:        1,
					OrderIndex:        0,
					InputAsset:        identityAsset(10_011, []byte("99 1\n")),
					ExpectedAsset:     identityAsset(10_012, []byte("100\n")),
					Weight:            1,
					StopOnFailure:     true,
					CreatedAt:         createdAt,
				}},
			},
		},
	}
	for _, group := range testset.Groups {
		for _, testcase := range group.Cases {
			testset.TestCount++
			testset.TotalUncompressedBytes +=
				testcase.InputAsset.UncompressedSize +
					testcase.ExpectedAsset.UncompressedSize
		}
	}
	checksum, err := testset.ComputeChecksum()
	if err != nil {
		t.Fatalf("ComputeChecksum() error = %v", err)
	}
	testset.Checksum = checksum

	createdBy := testset.CreatedBy
	record := &generate.TestsetRevision{
		ID:                     testset.ID,
		CreatedAt:              testset.CreatedAt,
		ProblemID:              testset.ProblemID,
		RevisionNumber:         testset.RevisionNumber,
		Checksum:               append([]byte(nil), testset.Checksum[:]...),
		TestCount:              testset.TestCount,
		TotalUncompressedBytes: testset.TotalUncompressedBytes,
		CreatedBy:              &createdBy,
		Edges: generate.TestsetRevisionEdges{
			Groups: make([]*generate.TestcaseGroup, len(testset.Groups)),
		},
	}
	for groupIndex, group := range testset.Groups {
		groupRecord := &generate.TestcaseGroup{
			ID:                group.ID,
			CreatedAt:         group.CreatedAt,
			TestsetRevisionID: group.TestsetRevisionID,
			GroupIndex:        group.GroupIndex,
			StopOnFailure:     group.StopOnFailure,
			Edges: generate.TestcaseGroupEdges{
				TestcaseRevisions: make([]*generate.TestcaseRevision, len(group.Cases)),
			},
		}
		for caseIndex, testcase := range group.Cases {
			groupRecord.Edges.TestcaseRevisions[caseIndex] =
				generatedCase(groupRecord.ID, testcase)
		}
		record.Edges.Groups[groupIndex] = groupRecord
	}
	return record
}

func generatedCase(groupID uuid.UUID, testcase entity.TestcaseRevision) *generate.TestcaseRevision {
	input := generatedAsset(testcase.InputAsset)
	expected := generatedAsset(testcase.ExpectedAsset)
	return &generate.TestcaseRevision{
		ID:                testcase.ID,
		CreatedAt:         testcase.CreatedAt,
		TestCaseID:        testcase.TestCaseID,
		ProblemID:         testcase.ProblemID,
		TestsetRevisionID: testcase.TestsetRevisionID,
		GroupID:           groupID,
		Kind:              testcaserevision.Kind(testcase.Kind),
		GroupIndex:        testcase.GroupIndex,
		OrderIndex:        testcase.OrderIndex,
		InputAssetID:      input.ID,
		ExpectedAssetID:   expected.ID,
		Weight:            testcase.Weight,
		StopOnFailure:     testcase.StopOnFailure,
		Edges: generate.TestcaseRevisionEdges{
			InputAsset:    input,
			ExpectedAsset: expected,
		},
	}
}

func cloneGeneratedCase(value *generate.TestcaseRevision) *generate.TestcaseRevision {
	cloned := *value
	input := *value.Edges.InputAsset
	input.Sha256 = append([]byte(nil), input.Sha256...)
	input.Content = append([]byte(nil), input.Content...)
	expected := *value.Edges.ExpectedAsset
	expected.Sha256 = append([]byte(nil), expected.Sha256...)
	expected.Content = append([]byte(nil), expected.Content...)
	cloned.Edges.InputAsset = &input
	cloned.Edges.ExpectedAsset = &expected
	return &cloned
}

func generatedAsset(asset entity.TestAsset) *generate.TestAsset {
	return &generate.TestAsset{
		ID:               asset.ID,
		CreatedAt:        asset.CreatedAt,
		Sha256:           append([]byte(nil), asset.SHA256[:]...),
		Encoding:         testasset.Encoding(asset.Encoding),
		Content:          append([]byte(nil), asset.Content...),
		UncompressedSize: asset.UncompressedSize,
	}
}

func identityAsset(id uint64, content []byte) entity.TestAsset {
	owned := append([]byte(nil), content...)
	return entity.TestAsset{
		ID:               task2DBUUID(id),
		SHA256:           sha256.Sum256(owned),
		Encoding:         problemcontract.AssetEncodingIdentity,
		Content:          owned,
		UncompressedSize: int64(len(owned)),
		CreatedAt:        task2DBTime(),
	}
}

type inertContentRepository struct{}

func (inertContentRepository) RegisterSource(
	context.Context,
	contentrender.Source,
) (contentrender.Source, error) {
	return contentrender.Source{}, errors.New("unexpected content repository call")
}

func (inertContentRepository) GetSource(
	context.Context,
	uuid.UUID,
) (contentrender.Source, error) {
	return contentrender.Source{}, errors.New("unexpected content repository call")
}

func (inertContentRepository) StoreArtifact(
	context.Context,
	contentrender.Source,
	contentrender.Artifact,
) (contentrender.Artifact, error) {
	return contentrender.Artifact{}, errors.New("unexpected content repository call")
}

func (inertContentRepository) GetArtifact(
	context.Context,
	uuid.UUID,
) (contentrender.Artifact, error) {
	return contentrender.Artifact{}, errors.New("unexpected content repository call")
}

func (inertContentRepository) FindArtifact(
	context.Context,
	uuid.UUID,
	string,
	string,
) (contentrender.Artifact, error) {
	return contentrender.Artifact{}, errors.New("unexpected content repository call")
}

type statementCapture struct {
	statements map[string]string
}

func (capture *statementCapture) Match(expectedSQL, actualSQL string) error {
	if capture.statements == nil {
		capture.statements = make(map[string]string)
	}
	capture.statements[expectedSQL] = actualSQL
	return nil
}

func (capture *statementCapture) statement(label string) string {
	if capture == nil {
		return ""
	}
	return capture.statements[label]
}

func newRevisionSQLMockClient(
	t *testing.T,
) (*generate.Client, sqlmock.Sqlmock, *statementCapture) {
	t.Helper()
	capture := &statementCapture{statements: make(map[string]string)}
	database, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(capture),
	)
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client, mock, capture
}

func problemSQLRows(record *generate.Problem) *sqlmock.Rows {
	values := []driver.Value{
		record.ID.String(),
		record.CreatedAt,
		record.UpdatedAt,
		nullableTime(record.DeletedAt),
		nullableString(record.DeletedBy),
		record.Slug,
		string(record.Lifecycle),
		record.Version,
		record.TagMappingID.String(),
		record.ElementMappingID.String(),
		nullableUUID(record.DraftRevisionID),
		nullableUUID(record.DraftRenderSourceID),
		nullableDraftKind(record.DraftRenderSourceKind),
		nullableBytes(record.DraftRenderSourceChecksum),
		nullableUUID(record.DraftRenderArtifactID),
		nullableUUID(record.PublishedRevisionID),
		nullableUUID(record.PublishedRenderSourceID),
		nullablePublishedKind(record.PublishedRenderSourceKind),
		nullableBytes(record.PublishedRenderSourceChecksum),
		nullableUUID(record.PublishedRenderArtifactID),
		nullableTime(record.PublishedAt),
		nullableTime(record.ArchivedAt),
		record.Title,
		record.Description,
		record.DifficultyID.String(),
		record.TimeLimitMs,
		record.MemoryLimitKB,
		record.AuthorID.String(),
		record.IsPublished,
		record.SubmissionCount,
		record.AcceptedCount,
	}
	return sqlmock.NewRows(problem.Columns).AddRow(values...)
}

func testAssetSQLRows(asset entity.TestAsset) *sqlmock.Rows {
	return sqlmock.NewRows(testasset.Columns).AddRow(
		asset.ID.String(),
		asset.CreatedAt,
		append([]byte(nil), asset.SHA256[:]...),
		asset.Encoding,
		append([]byte(nil), asset.Content...),
		asset.UncompressedSize,
	)
}

func testCaseSQLRows(
	id uuid.UUID,
	problemID uuid.UUID,
	createdAt time.Time,
) *sqlmock.Rows {
	return sqlmock.NewRows(testcase.Columns).AddRow(
		id.String(),
		createdAt,
		createdAt,
		nil,
		nil,
		problemID.String(),
	)
}

func nullableUUID(value *uuid.UUID) driver.Value {
	if value == nil {
		return nil
	}
	return value.String()
}

func nullableTime(value *time.Time) driver.Value {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value *string) driver.Value {
	if value == nil {
		return nil
	}
	return *value
}

func nullableBytes(value *[]byte) driver.Value {
	if value == nil {
		return nil
	}
	return append([]byte(nil), (*value)...)
}

func nullableDraftKind(value *problem.DraftRenderSourceKind) driver.Value {
	if value == nil {
		return nil
	}
	return string(*value)
}

func nullablePublishedKind(value *problem.PublishedRenderSourceKind) driver.Value {
	if value == nil {
		return nil
	}
	return string(*value)
}

func sqlWhereClause(t *testing.T, statement string) string {
	t.Helper()
	index := strings.Index(strings.ToUpper(statement), " WHERE ")
	if index < 0 {
		t.Fatalf("SQL has no WHERE clause: %s", statement)
	}
	return statement[index:]
}

func uuidPointer(value uuid.UUID) *uuid.UUID {
	copy := value
	return &copy
}

func task2DBUUID(value uint64) uuid.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], value)
	return id
}

func task2DBTime() time.Time {
	return time.Unix(1_700_000_000, 0).UTC()
}

var _ contentrender.Repository = inertContentRepository{}
var _ sqlmock.QueryMatcher = (*statementCapture)(nil)
