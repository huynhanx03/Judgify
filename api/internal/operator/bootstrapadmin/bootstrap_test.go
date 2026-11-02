package bootstrapadmin

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	identityconstant "github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/attributedefinition"
	"github.com/huynhanx03/judgify/internal/ent/generate/role"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
)

func TestValidateFreshStateAcceptsOnlyEmptyDatabaseOrCanonicalStudentSeed(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		state databaseState
		want  bool
	}{
		{name: "empty", want: true},
		{
			name: "canonical student seed",
			state: databaseState{roles: []roleState{{
				key: "student", system: true, active: true, version: 1,
			}}},
			want: true,
		},
		{
			name:  "authorization initialized",
			state: databaseState{authorizationStates: 1},
		},
		{
			name:  "existing user",
			state: databaseState{users: 1},
		},
		{
			name:  "existing credential",
			state: databaseState{credentials: 1},
		},
		{
			name:  "existing casbin rule",
			state: databaseState{casbinRules: 1},
		},
		{
			name:  "existing audit",
			state: databaseState{audits: 1},
		},
		{
			name:  "superadmin role already exists",
			state: databaseState{roles: []roleState{{key: "superadmin", system: true, active: true, version: 1}}},
		},
		{
			name:  "student seed is mutable",
			state: databaseState{roles: []roleState{{key: "student", system: false, active: true, version: 1}}},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := validateFreshState(testCase.state)
			if (err == nil) != testCase.want {
				t.Fatalf("validateFreshState() error = %v, want valid=%t", err, testCase.want)
			}
		})
	}
}

func TestBootstrapRejectsInitializedDatabaseBeforeAnyWrite(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "authorization_states"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = Bootstrap(context.Background(), client, Input{
		Username: "operator",
		Password: []byte("correct horse battery staple"),
	})
	require.ErrorIs(t, err, ErrAlreadyInitialized)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBootstrapCreatesCanonicalAuthorizationStateAtomically(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := generate.NewClient(generate.Driver(entsql.OpenDB(dialect.Postgres, database)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	for _, table := range []string{
		"authorization_states",
		"users",
		"credentials",
		"casbin_rules",
		"audit_logs",
		"attribute_definitions",
		"attribute_definition_revisions",
		"user_attribute_values",
		"profile_schema_states",
	} {
		mock.ExpectQuery(`SELECT COUNT\(.* FROM "` + table + `"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	mock.ExpectQuery(`SELECT .* FROM "roles"`).WillReturnRows(sqlmock.NewRows(role.Columns))
	for range 2 {
		mock.ExpectExec(`INSERT INTO "roles"`).WillReturnResult(sqlmock.NewResult(1, 1))
	}
	stubRevisionID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	stubNow := time.Now().UTC()
	for _, spec := range identityconstant.RequiredSystemAttributeCatalog() {
		stubDefID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
		mock.ExpectExec(`INSERT INTO "attribute_definitions"`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`INSERT INTO "attribute_definition_revisions"`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`UPDATE "attribute_definitions"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`SELECT .* FROM "attribute_definitions"`).
			WillReturnRows(sqlmock.NewRows(attributedefinition.Columns).AddRow(
				stubDefID,
				stubNow,
				stubNow,
				nil,
				nil,
				spec.Key,
				string(attributedefinition.KindSystem),
				true,
				string(attributedefinition.StatusActive),
				stubRevisionID,
				stubRevisionID,
				int64(1),
			))
	}
	mock.ExpectExec(`INSERT INTO "profile_schema_states"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO "users"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO "credentials"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`INSERT INTO "casbin_rules"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO "casbin_rules"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	mock.ExpectExec(`INSERT INTO "authorization_states"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	administratorID, err := Bootstrap(context.Background(), client, Input{
		Username: "operator",
		Password: []byte("correct horse battery staple"),
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, administratorID)
	require.NoError(t, mock.ExpectationsWereMet())
}
