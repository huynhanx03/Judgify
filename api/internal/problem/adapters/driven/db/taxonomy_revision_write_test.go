package db

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/difficulty"
	"github.com/huynhanx03/judgify/internal/ent/generate/element"
	generatedtag "github.com/huynhanx03/judgify/internal/ent/generate/tag"
	"github.com/huynhanx03/judgify/internal/ports"
	problemEntity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// This is intentionally the first taxonomy-writer contract: a stable version
// is part of the domain record that a transactional writer must compare and
// advance. The remaining repository probes are added with the implementation.
func TestDifficultyRevisionDomainCarriesStableVersion(t *testing.T) {
	difficulty := problemEntity.Difficulty{Version: 1}
	require.Equal(t, int64(1), difficulty.Version)
}

func TestTaxonomyRevisionChecksumsMatchGoldenVectors(t *testing.T) {
	difficultyID := uuid.MustParse("00112233-4455-6677-8899-aabbccddeeff")
	difficulty := difficultyRevisionChecksum(difficultyID, 7, "Hard", 9, 1234, "Café")
	require.Equal(t, "bc8bb5af866b27c2dcb33975621c4af30e54d0e0f94feab29c65d07f039db17f", hex.EncodeToString(difficulty[:]))

	tagID := uuid.MustParse("abcdefab-cdef-abcd-efab-cdefabcdefab")
	first := uuid.MustParse("ffeeddcc-bbaa-9988-7766-554433221100")
	second := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	tag := tagRevisionChecksum(tagID, 4, "graphs", []elementRevisionFact{{ID: first, Revision: 2}, {ID: second, Revision: 9}})
	require.Equal(t, "21258003655160f78116446db8517b70fc70cdd63047bca71b9fc34b845ae597", hex.EncodeToString(tag[:]))

	reversed := tagRevisionChecksum(tagID, 4, "graphs", []elementRevisionFact{{ID: second, Revision: 9}, {ID: first, Revision: 2}})
	require.Equal(t, tag, reversed, "tag memberships must hash in raw UUID order")
}

func TestDifficultyRevisionCreateRollsBackStableProjectionWhenRevisionWriteFails(t *testing.T) {
	repository, mock := newTaxonomyMockDifficultyRepository(t)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "difficulties"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "difficulty_revisions"`).WillReturnError(assertedRevisionWriteFailure{})
	mock.ExpectRollback()

	value := &problemEntity.Difficulty{Name: "Hard", Level: 9, ExpReward: 1234, Description: "deterministic"}
	err := repository.Create(context.Background(), value)
	require.Error(t, err)
	require.Equal(t, uuid.Nil, value.ID, "the caller is updated only after commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTaxonomyRevisionWritersUseTheTransactionScopedClient(t *testing.T) {
	for _, test := range []struct {
		name  string
		write func(*DifficultyRepository, context.Context) error
	}{
		{
			name: "difficulty",
			write: func(repository *DifficultyRepository, ctx context.Context) error {
				return repository.Create(ctx, &problemEntity.Difficulty{Name: "Hard", Level: 9, ExpReward: 1234})
			},
		},
		{
			name: "tag",
			write: func(repository *DifficultyRepository, ctx context.Context) error {
				return (&TagRepository{client: repository.client}).Create(ctx, &problemEntity.Tag{Name: "graphs"})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := &transactionProbeDriver{tx: &transactionProbeTx{}}
			client := generate.NewClient(generate.Driver(driver))
			t.Cleanup(func() { _ = client.Close() })
			repository := &DifficultyRepository{client: dbent.WrapClient(client, nil)}

			err := dbent.NewTxManager(repository.client).DoInTx(context.Background(), func(txCtx context.Context) error {
				return test.write(repository, txCtx)
			})
			require.Error(t, err)
			require.Zero(t, driver.rootWriteCount())
			require.Equal(t, 1, driver.tx.writeCount())
			require.Zero(t, driver.tx.commitCount())
			require.Equal(t, 1, driver.tx.rollbackCount())
		})
	}
}

func TestTagRevisionSnapshotsSortedExactElementVersions(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	first := uuid.MustParse("ffeeddcc-bbaa-9988-7766-554433221100")
	second := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT .* FROM "elements".*ORDER BY .*"id".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows(element.Columns).
		AddRow(first, now, now, nil, nil, "Fire", "fire", "", int64(2)).
		AddRow(second, now, now, nil, nil, "Water", "water", "", int64(9)))

	_, facts, err := resolveTagElements(context.Background(), client, []uuid.UUID{first, second})
	require.NoError(t, err)
	require.Equal(t, []elementRevisionFact{{ID: second, Revision: 9}, {ID: first, Revision: 2}}, facts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDifficultyRevisionStaleVersionReturnsTypedConflictWithoutAppend(t *testing.T) {
	repository, mock := newTaxonomyMockDifficultyRepository(t)
	id := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulties".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows(difficulty.Columns).
		AddRow(id, now, now, nil, nil, "Hard", 9, int64(1234), "", int64(2)))
	mock.ExpectRollback()

	err := repository.Update(context.Background(), &problemEntity.Difficulty{
		ID: id, Name: "Hard", Level: 9, ExpReward: 1234, Version: 1,
	})
	require.ErrorIs(t, err, ports.ErrDifficultyVersionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDifficultyRevisionUpdateAppendsThenAdvancesWithVersionCAS(t *testing.T) {
	repository, mock := newTaxonomyMockDifficultyRepository(t)
	id := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulties".*FOR UPDATE`).WillReturnRows(taxonomyDifficultyRows(
		id, now, "Hard", 9, 1234, "old", 1,
	))
	mock.ExpectExec(`INSERT INTO "difficulty_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "difficulties".*"version"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "difficulties" WHERE "id"`).WillReturnRows(taxonomyDifficultyRows(
		id, now, "Expert", 10, 1500, "new", 2,
	))
	mock.ExpectCommit()

	value := &problemEntity.Difficulty{
		ID: id, Name: "Expert", Level: 10, ExpReward: 1500, Description: "new", Version: 1,
	}
	require.NoError(t, repository.Update(context.Background(), value))
	require.Equal(t, int64(2), value.Version)
	require.Equal(t, "Expert", value.Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDifficultyRevisionCASMissAfterAppendRollsBackAndReturnsTypedConflict(t *testing.T) {
	repository, mock := newTaxonomyMockDifficultyRepository(t)
	id := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "difficulties".*FOR UPDATE`).WillReturnRows(taxonomyDifficultyRows(
		id, now, "Hard", 9, 1234, "old", 1,
	))
	mock.ExpectExec(`INSERT INTO "difficulty_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "difficulties".*"version"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT EXISTS .* FROM "difficulties".*"version"`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	value := &problemEntity.Difficulty{
		ID: id, Name: "Expert", Level: 10, ExpReward: 1500, Description: "new", Version: 1,
	}
	err := repository.Update(context.Background(), value)
	require.ErrorIs(t, err, ports.ErrDifficultyVersionConflict)
	require.Equal(t, int64(1), value.Version, "caller state changes only after commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRevisionRejectsDuplicateElementIDsBeforeStableMutation(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	repository := &TagRepository{client: dbent.WrapClient(client, nil)}
	elementID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectRollback()
	err := repository.Create(context.Background(), &problemEntity.Tag{
		Name:     "graphs",
		Elements: []problemEntity.TagElement{{ID: elementID}, {ID: elementID}},
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRevisionCreatePersistsStableRevisionAndExactLinkInOneTransaction(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	repository := &TagRepository{client: dbent.WrapClient(client, nil)}
	elementID := uuid.New()
	tagID := uuid.New()
	previousDefaultID := generatedtag.DefaultID
	generatedtag.DefaultID = func() uuid.UUID { return tagID }
	t.Cleanup(func() { generatedtag.DefaultID = previousDefaultID })
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "elements".*ORDER BY .*"id".*FOR UPDATE`).WillReturnRows(
		taxonomyElementRows(elementID, now, "Water", "water", 7),
	)
	mock.ExpectExec(`INSERT INTO "tags"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_revision_elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "tags" WHERE`).WillReturnRows(taxonomyTagRows(
		tagID, now, "graphs", 1,
	))
	mock.ExpectQuery(`SELECT .* FROM "elements".*JOIN "tag_elements"`).WillReturnRows(
		sqlmock.NewRows(append([]string{"tag_id"}, element.Columns...)).
			AddRow(tagID, elementID, now, now, nil, nil, "Water", "water", "", int64(7)),
	)
	mock.ExpectCommit()

	value := &problemEntity.Tag{Name: "graphs", Elements: []problemEntity.TagElement{{ID: elementID}}}
	require.NoError(t, repository.Create(context.Background(), value))
	require.Equal(t, tagID, value.ID)
	require.Equal(t, int64(1), value.Version)
	require.Len(t, value.Elements, 1)
	require.Equal(t, elementID, value.Elements[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRevisionCreateLinkFailureRollsBackStableProjectionAndHistory(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	repository := &TagRepository{client: dbent.WrapClient(client, nil)}
	elementID := uuid.New()
	tagID := uuid.New()
	previousDefaultID := generatedtag.DefaultID
	generatedtag.DefaultID = func() uuid.UUID { return tagID }
	t.Cleanup(func() { generatedtag.DefaultID = previousDefaultID })
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "elements".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).
		WillReturnRows(taxonomyElementRows(elementID, now, "Water", "water", 7))
	mock.ExpectExec(`INSERT INTO "tags"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_revision_elements"`).WillReturnError(errors.New("link write failed"))
	mock.ExpectRollback()

	value := &problemEntity.Tag{Name: "graphs", Elements: []problemEntity.TagElement{{ID: elementID}}}
	err := repository.Create(context.Background(), value)
	require.Error(t, err)
	require.Equal(t, uuid.Nil, value.ID, "caller state changes only after commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRevisionMissingOrDeletedElementRejectsBeforeStableMutation(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	repository := &TagRepository{client: dbent.WrapClient(client, nil)}
	elementID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "elements".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows(element.Columns))
	mock.ExpectRollback()

	value := &problemEntity.Tag{Name: "graphs", Elements: []problemEntity.TagElement{{ID: elementID}}}
	err := repository.Create(context.Background(), value)
	require.Error(t, err)
	require.Equal(t, uuid.Nil, value.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRevisionUpdateAppendsLinksAndReplacesMutableMembershipWithCAS(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	repository := &TagRepository{client: dbent.WrapClient(client, nil)}
	tagID := uuid.New()
	elementID := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "tags".*FOR UPDATE`).
		WillReturnRows(taxonomyTagRows(tagID, now, "old", 1))
	mock.ExpectQuery(`SELECT .* FROM "elements".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).
		WillReturnRows(taxonomyElementRows(elementID, now, "Water", "water", 7))
	mock.ExpectExec(`INSERT INTO "tag_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_revision_elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "tags".*"version"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "tag_elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "tags" WHERE "id"`).
		WillReturnRows(taxonomyTagRows(tagID, now, "new", 2))
	mock.ExpectQuery(`SELECT .* FROM "tags" WHERE`).
		WillReturnRows(taxonomyTagRows(tagID, now, "new", 2))
	mock.ExpectQuery(`SELECT .* FROM "elements".*JOIN "tag_elements"`).WillReturnRows(
		sqlmock.NewRows(append([]string{"tag_id"}, element.Columns...)).
			AddRow(tagID, elementID, now, now, nil, nil, "Water", "water", "", int64(7)),
	)
	mock.ExpectCommit()

	value := &problemEntity.Tag{
		ID: tagID, Name: "new", Version: 1,
		Elements: []problemEntity.TagElement{{ID: elementID}},
	}
	require.NoError(t, repository.Update(context.Background(), value))
	require.Equal(t, int64(2), value.Version)
	require.Equal(t, "new", value.Name)
	require.Equal(t, []uuid.UUID{elementID}, value.ElementIDs())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTagRevisionUpdateLinkFailureRollsBackBeforeMutableMembershipCAS(t *testing.T) {
	client, mock := newTaxonomyMockClient(t)
	repository := &TagRepository{client: dbent.WrapClient(client, nil)}
	tagID := uuid.New()
	elementID := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "tags".*FOR UPDATE`).
		WillReturnRows(taxonomyTagRows(tagID, now, "old", 1))
	mock.ExpectQuery(`SELECT .* FROM "elements".*"deleted_at" IS NULL.*ORDER BY .*"id".*FOR UPDATE`).
		WillReturnRows(taxonomyElementRows(elementID, now, "Water", "water", 7))
	mock.ExpectExec(`INSERT INTO "tag_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "tag_revision_elements"`).WillReturnError(errors.New("link write failed"))
	mock.ExpectRollback()

	value := &problemEntity.Tag{
		ID: tagID, Name: "new", Version: 1,
		Elements: []problemEntity.TagElement{{ID: elementID}},
	}
	err := repository.Update(context.Background(), value)
	require.Error(t, err)
	require.Equal(t, int64(1), value.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func taxonomyDifficultyRows(
	id uuid.UUID,
	now time.Time,
	name string,
	level int,
	experience int64,
	description string,
	version int64,
) *sqlmock.Rows {
	return sqlmock.NewRows(difficulty.Columns).
		AddRow(id, now, now, nil, nil, name, level, experience, description, version)
}

func taxonomyElementRows(
	id uuid.UUID,
	now time.Time,
	name string,
	code string,
	version int64,
) *sqlmock.Rows {
	return sqlmock.NewRows(element.Columns).
		AddRow(id, now, now, nil, nil, name, code, "", version)
}

func taxonomyTagRows(id uuid.UUID, now time.Time, name string, version int64) *sqlmock.Rows {
	return sqlmock.NewRows(generatedtag.Columns).
		AddRow(id, now, now, nil, nil, name, version)
}

func newTaxonomyMockDifficultyRepository(t *testing.T) (*DifficultyRepository, sqlmock.Sqlmock) {
	t.Helper()
	client, mock := newTaxonomyMockClient(t)
	return &DifficultyRepository{client: dbent.WrapClient(client, nil)}, mock
}

func newTaxonomyMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

type assertedRevisionWriteFailure struct{}

func (assertedRevisionWriteFailure) Error() string { return "revision write failure" }
