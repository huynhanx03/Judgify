package db

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	casbinmodel "github.com/casbin/casbin/v3/model"
	adapter "github.com/casbin/ent-adapter"
	adapterent "github.com/casbin/ent-adapter/ent"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

const adapterTestModel = `
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

func TestPinnedEntAdapterCanStartWithoutRuntimeDDL(t *testing.T) {
	t.Parallel()
	database, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	driver := entsql.OpenDB(dialect.Postgres, database)
	client := adapterent.NewClient(adapterent.Driver(driver))
	loaded, err := adapter.NewAdapterWithClient(
		client,
		adapter.SkipSchemaCreate(),
		adapter.WithContext(context.Background()),
	)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.NoError(t, sqlMock.ExpectationsWereMet(), "constructor must execute no schema SQL")
}

func TestPinnedEntAdapterRejectsPolicySnapshotsOverConfiguredLimit(t *testing.T) {
	t.Parallel()
	database, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	driver := entsql.OpenDB(dialect.Postgres, database)
	client := adapterent.NewClient(adapterent.Driver(driver))
	loaded, err := adapter.NewAdapterWithClient(
		client,
		adapter.SkipSchemaCreate(),
		adapter.WithContext(context.Background()),
		adapter.WithPolicyLimit(1),
	)
	require.NoError(t, err)

	parsed, err := casbinmodel.NewModelFromString(adapterTestModel)
	require.NoError(t, err)
	sqlMock.ExpectQuery(`SELECT .* FROM "casbin_rules"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5"}).
			AddRow(1, "p", "role:teacher", "problem", "read", "", "", "").
			AddRow(2, "p", "role:teacher", "problem", "update", "", "", ""))

	require.ErrorIs(t, loaded.LoadPolicy(parsed), adapter.ErrPolicyLimitExceeded)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestAuthorizationRepositoryLoadsStartupSnapshotThroughPinnedAdapter(t *testing.T) {
	t.Parallel()
	database, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	driver := entsql.OpenDB(dialect.Postgres, database)
	generated := generate.NewClient(generate.Driver(driver))
	client := dbent.WrapClientWithDriver(generated, driver, nil)
	repository := NewAuthorizationPolicyRepository(client)

	sqlMock.ExpectQuery(`SELECT .* FROM "casbin_rules"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5"}).
			AddRow(1, "p", "role:teacher", "problem", "read", "", "", "").
			AddRow(2, "g", "user:018f6abb-8dd5-7581-8449-2b9ad77873e5", "role:teacher", "", "", "", ""))

	rules, err := repository.LoadRules(context.Background(), adapterTestModel)
	require.NoError(t, err)
	require.Equal(t, []authcasbin.PolicyRule{
		{Type: "g", V0: "user:018f6abb-8dd5-7581-8449-2b9ad77873e5", V1: "role:teacher"},
		{Type: "p", V0: "role:teacher", V1: "problem", V2: "read"},
	}, rules)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}
