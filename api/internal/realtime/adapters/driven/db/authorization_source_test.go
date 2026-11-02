package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/realtime"
)

func TestAuthorizationSourceReadsOnlyCommittedRevisionProjection(t *testing.T) {
	source, mock := newAuthorizationSource(t)
	changedAt := time.Date(2026, time.July, 22, 9, 30, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT .*desired_revision.*updated_at.* FROM "authorization_states" .*id.* LIMIT`).
		WillReturnRows(sqlmock.NewRows([]string{"desired_revision", "updated_at"}).AddRow(int64(17), changedAt))

	change, err := source.Current(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(17), change.Event.Revision)
	require.Equal(t, changedAt, change.Event.ChangedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthorizationSourceRejectsMalformedDurableFence(t *testing.T) {
	source, mock := newAuthorizationSource(t)
	mock.ExpectQuery(`SELECT .* FROM "authorization_states"`).
		WillReturnRows(sqlmock.NewRows([]string{"desired_revision", "updated_at"}).
			AddRow(int64(0), time.Now().UTC()))

	_, err := source.Current(context.Background())
	require.ErrorIs(t, err, realtime.ErrInvalidChange)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newAuthorizationSource(t *testing.T) (*AuthorizationSource, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	generated := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = generated.Close() })
	client := internalent.WrapClient(generated, zap.NewNop())
	source, err := NewAuthorizationSource(client)
	require.NoError(t, err)
	return source, mock
}
