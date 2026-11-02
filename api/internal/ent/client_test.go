package ent

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

type testPostCommitRegistrar interface {
	RegisterAfterCommit(context.Context, func(context.Context) error) error
}

func TestTxManagerJoinsNestedWorkAndRunsPostCommitAfterOutermostCommit(t *testing.T) {
	client, sqlMock := newEntClientMock(t)
	sqlMock.ExpectBegin()
	sqlMock.ExpectCommit()

	wrapper := WrapClient(client, zap.NewNop())
	manager := NewTxManager(wrapper)
	registrar, ok := manager.(testPostCommitRegistrar)
	require.True(t, ok)

	var scoped context.Context
	postCommitCalls := 0
	err := manager.DoInTx(context.Background(), func(txCtx context.Context) error {
		scoped = txCtx
		outerClient := wrapper.DB(txCtx)
		require.NotSame(t, wrapper.Client(), outerClient)

		require.NoError(t, manager.DoInTx(txCtx, func(nestedCtx context.Context) error {
			require.Same(t, outerClient, wrapper.DB(nestedCtx))
			return registrar.RegisterAfterCommit(nestedCtx, func(context.Context) error {
				postCommitCalls++
				return nil
			})
		}))
		require.Zero(t, postCommitCalls, "nested work must not commit or invalidate early")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, postCommitCalls)
	require.PanicsWithValue(t, platformtx.ErrTransactionClosed, func() { wrapper.DB(scoped) })
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestReadSnapshotTxManagerUsesRepeatableReadOnlyTransaction(t *testing.T) {
	client, sqlMock := newEntClientMock(t)
	sqlMock.ExpectBegin()
	sqlMock.ExpectCommit()

	wrapper := WrapClient(client, zap.NewNop())
	manager := NewReadSnapshotTxManager(wrapper)
	configured, ok := manager.(*entTxManager)
	require.True(t, ok)
	require.NotNil(t, configured.options)
	require.Equal(t, sql.LevelRepeatableRead, configured.options.Isolation)
	require.True(t, configured.options.ReadOnly)

	require.NoError(t, manager.DoInTx(context.Background(), func(txCtx context.Context) error {
		require.NotSame(t, wrapper.Client(), wrapper.DB(txCtx))
		return nil
	}))
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestEntClientArchitectureHasNoPrivateTransactionContext(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	clientSource := filepath.Join(filepath.Dir(source), "client.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), clientSource, nil, 0)
	require.NoError(t, err)

	forbiddenIdentifiers := map[string]struct{}{
		strings.Join([]string{"TX", "Key"}, ""): {},
		strings.Join([]string{"Get", "Tx"}, ""): {},
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			if _, forbidden := forbiddenIdentifiers[value.Name]; forbidden {
				t.Errorf("legacy transaction context identifier %q must not return", value.Name)
			}
		case *ast.SelectorExpr:
			if value.Sel.Name == strings.Join([]string{"With", "Value"}, "") {
				t.Errorf("EntClient must use platform transaction scope, not context.WithValue")
			}
		}
		return true
	})
}

func newEntClientMock(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}
