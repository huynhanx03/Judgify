package db

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedelement "github.com/huynhanx03/judgify/internal/ent/generate/element"
	"github.com/huynhanx03/judgify/internal/ports"
)

// Element writers use the same stable-version concurrency contract as the
// Problem taxonomy writers.
func TestElementRevisionDomainCarriesStableVersion(t *testing.T) {
	element := entity.Element{Version: 1}
	require.Equal(t, int64(1), element.Version)
}

func TestElementRevisionChecksumMatchesGoldenVector(t *testing.T) {
	id := uuid.MustParse("10213243-5465-7687-98a9-bacbdcedfe0f")
	checksum := elementRevisionChecksum(id, 3, "Water", "water", "flow")
	require.Equal(t, "16149cd28204212ca26ef035d78f8fbc5a5b2a96d2a5aca4153dec7a8415e114", hex.EncodeToString(checksum[:]))
}

func TestElementRevisionCreateRollsBackStableProjectionOnRevisionFailure(t *testing.T) {
	repository, mock := newElementWriterMockRepository(t)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "elements"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "element_revisions"`).WillReturnError(errors.New("revision write failed"))
	mock.ExpectRollback()

	value := &entity.Element{Name: "Water", Code: "water", Description: "flow"}
	err := repository.Create(context.Background(), value)
	require.Error(t, err)
	require.Equal(t, uuid.Nil, value.ID, "caller state changes only after commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestElementRevisionUpdateAppendsThenAdvancesWithVersionCAS(t *testing.T) {
	repository, mock := newElementWriterMockRepository(t)
	id := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "elements".*FOR UPDATE`).WillReturnRows(elementWriterRows(
		id, now, "Water", "water", "old", 1,
	))
	mock.ExpectExec(`INSERT INTO "element_revisions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "elements".*"version"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "elements" WHERE "id"`).WillReturnRows(elementWriterRows(
		id, now, "Ice", "ice", "new", 2,
	))
	mock.ExpectCommit()

	value := &entity.Element{
		ID: id, Name: "Ice", Code: "ice", Description: "new", Version: 1,
	}
	require.NoError(t, repository.Update(context.Background(), value))
	require.Equal(t, int64(2), value.Version)
	require.Equal(t, "ice", value.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestElementRevisionStaleUpdateReturnsTypedConflictWithoutAppend(t *testing.T) {
	repository, mock := newElementWriterMockRepository(t)
	id := uuid.New()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "elements".*FOR UPDATE`).WillReturnRows(elementWriterRows(
		id, now, "Water", "water", "old", 2,
	))
	mock.ExpectRollback()

	err := repository.Update(context.Background(), &entity.Element{
		ID: id, Name: "Ice", Code: "ice", Description: "new", Version: 1,
	})
	require.ErrorIs(t, err, ports.ErrElementVersionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newElementWriterMockRepository(t *testing.T) (*ElementRepository, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	t.Cleanup(func() { _ = client.Close() })
	return &ElementRepository{client: dbent.WrapClient(client, nil)}, mock
}

func elementWriterRows(
	id uuid.UUID,
	now time.Time,
	name string,
	code string,
	description string,
	version int64,
) *sqlmock.Rows {
	return sqlmock.NewRows(generatedelement.Columns).
		AddRow(id, now, now, nil, nil, name, code, description, version)
}
