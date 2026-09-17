package bootstrap

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
)

func TestDatabaseHealthCheckerFailsClosedWithoutOpenClient(t *testing.T) {
	t.Parallel()

	checker := (&Process{}).DatabaseHealthChecker()
	require.Error(t, checker.Check(context.Background()))
}

func TestDatabaseHealthCheckerNormalizesQueryAndCloseFailures(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name string
		rows *sqlmock.Rows
		err  error
	}{
		{name: "query", err: errors.New("driver query details")},
		{
			name: "close",
			rows: sqlmock.NewRows([]string{"value"}).
				AddRow(1).
				CloseError(errors.New("driver close details")),
		},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = database.Close() })

			generated := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
			process := &Process{database: internalent.WrapClient(generated, nil)}
			expectation := mock.ExpectQuery(`SELECT 1`)
			if testCase.err != nil {
				expectation.WillReturnError(testCase.err)
			} else {
				expectation.WillReturnRows(testCase.rows)
			}

			checkErr := process.DatabaseHealthChecker().Check(context.Background())
			require.ErrorIs(t, checkErr, ErrDatabaseHealth)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
