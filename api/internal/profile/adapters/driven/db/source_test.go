package db

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func TestSourceReturnsUserStatsStorageFailure(t *testing.T) {
	source, mock := newProfileSource(t)
	storageErr := errors.New("profile database unavailable")
	mock.ExpectQuery(`SELECT .* FROM "user_stats"`).WillReturnError(storageErr)

	projection, err := source.Load(context.Background(), uuid.New())

	require.Zero(t, projection)
	require.ErrorIs(t, err, storageErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRejectsInvalidConstructionAndRequest(t *testing.T) {
	source, err := NewSource(nil)
	require.Nil(t, source)
	require.ErrorIs(t, err, ErrInvalidSource)

	valid, mock := newProfileSource(t)
	_, err = valid.Load(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, ErrInvalidSource)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newProfileSource(t *testing.T) (*Source, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	generated := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = generated.Close() })
	source, err := NewSource(internalent.WrapClient(generated, zap.NewNop()))
	require.NoError(t, err)
	return source, mock
}
