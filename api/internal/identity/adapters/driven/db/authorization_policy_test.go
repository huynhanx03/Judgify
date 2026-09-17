package db

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	adaptercasbinrule "github.com/casbin/ent-adapter/ent/casbinrule"
	"github.com/google/uuid"
	authcasbin "github.com/huynhanx03/go-common/pkg/security/authorization/casbin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/authorizationstate"
	"github.com/huynhanx03/judgify/internal/ent/generate/casbinrule"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestAuthorizationPolicyRuleNormalization(t *testing.T) {
	roleSubject := "role:teacher"
	input := []authcasbin.PolicyRule{
		{Type: "p", V0: roleSubject, V1: "submission", V2: "read"},
		{Type: "p", V0: roleSubject, V1: "problem", V2: "update"},
		{Type: "p", V0: roleSubject, V1: "problem", V2: "read"},
	}
	original := append([]authcasbin.PolicyRule(nil), input...)

	normalized, err := normalizeRolePolicyRules(roleSubject, input)
	require.NoError(t, err)
	require.Equal(t, []authcasbin.PolicyRule{
		{Type: "p", V0: roleSubject, V1: "problem", V2: "read"},
		{Type: "p", V0: roleSubject, V1: "problem", V2: "update"},
		{Type: "p", V0: roleSubject, V1: "submission", V2: "read"},
	}, normalized)
	require.Equal(t, original, input, "normalization must not mutate caller-owned input")

	invalid := []struct {
		name        string
		roleSubject string
		rules       []authcasbin.PolicyRule
	}{
		{name: "missing prefix", roleSubject: "teacher", rules: input},
		{name: "uppercase role", roleSubject: "role:Teacher", rules: input},
		{name: "wrong owner", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{{Type: "p", V0: "role:admin", V1: "problem", V2: "read"}}},
		{name: "wrong type", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{{Type: "g", V0: roleSubject, V1: "problem", V2: "read"}}},
		{name: "trim required", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{{Type: "p", V0: roleSubject, V1: " problem", V2: "read"}}},
		{name: "control character", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{{Type: "p", V0: roleSubject, V1: "problem", V2: "read\nforged"}}},
		{name: "unused value", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{{Type: "p", V0: roleSubject, V1: "problem", V2: "read", V3: "unexpected"}}},
		{name: "duplicate", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{
			{Type: "p", V0: roleSubject, V1: "problem", V2: "read"},
			{Type: "p", V0: roleSubject, V1: "problem", V2: "read"},
		}},
		{name: "value bound", roleSubject: roleSubject, rules: []authcasbin.PolicyRule{{Type: "p", V0: roleSubject, V1: strings.Repeat("x", maxAuthorizationPolicyValueBytes+1), V2: "read"}}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			_, err := normalizeRolePolicyRules(test.roleSubject, test.rules)
			require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
		})
	}

	tooMany := make([]authcasbin.PolicyRule, maxAuthorizationRulesPerMutation+1)
	_, err = normalizeRolePolicyRules(roleSubject, tooMany)
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
}

func TestAuthorizationUserRoleNormalization(t *testing.T) {
	userID := uuid.New()
	userSubject := "user:" + userID.String()
	rules, err := normalizeUserRoleRules(userSubject, []string{
		"role:teacher",
		"role:admin",
	})
	require.NoError(t, err)
	require.Equal(t, []authcasbin.PolicyRule{
		{Type: "g", V0: userSubject, V1: "role:admin"},
		{Type: "g", V0: userSubject, V1: "role:teacher"},
	}, rules)

	_, err = normalizeUserRoleRules(userSubject, []string{"role:admin", "role:admin"})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
	_, err = normalizeUserRoleRules("user:"+strings.ToUpper(userID.String()), []string{"role:admin"})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
	_, err = normalizeUserRoleRules("user:"+uuid.Nil.String(), []string{"role:admin"})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
	_, err = normalizeUserRoleRules(userSubject, []string{"admin"})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
}

func TestAuthorizationPolicyCanonicalRecordConversion(t *testing.T) {
	userSubject := "user:" + uuid.NewString()
	tests := []struct {
		name   string
		record *generate.CasbinRule
		want   authcasbin.PolicyRule
		valid  bool
	}{
		{
			name:   "policy",
			record: &generate.CasbinRule{Ptype: "p", V0: "role:admin", V1: "problem", V2: "read"},
			want:   authcasbin.PolicyRule{Type: "p", V0: "role:admin", V1: "problem", V2: "read"},
			valid:  true,
		},
		{
			name:   "grouping",
			record: &generate.CasbinRule{Ptype: "g", V0: userSubject, V1: "role:admin"},
			want:   authcasbin.PolicyRule{Type: "g", V0: userSubject, V1: "role:admin"},
			valid:  true,
		},
		{name: "nil", record: nil},
		{name: "unknown type", record: &generate.CasbinRule{Ptype: "p2", V0: "role:admin", V1: "problem", V2: "read"}},
		{name: "raw user id", record: &generate.CasbinRule{Ptype: "g", V0: uuid.NewString(), V1: "role:admin"}},
		{name: "noncanonical tail", record: &generate.CasbinRule{Ptype: "g", V0: userSubject, V1: "role:admin", V2: "extra"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := authorizationPolicyRuleFromRecord(test.record)
			if !test.valid {
				require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestAuthorizationStateConversionOwnsUpdater(t *testing.T) {
	updater := uuid.New()
	originalUpdater := updater
	record := &generate.AuthorizationState{
		ID:              authorizationStateID,
		DesiredRevision: 7,
		UpdatedBy:       &updater,
	}
	state, err := authorizationStateFromRecord(record)
	require.NoError(t, err)
	require.NotNil(t, state.UpdatedBy)
	require.NotSame(t, record.UpdatedBy, state.UpdatedBy)
	require.Equal(t, originalUpdater, *state.UpdatedBy)

	*record.UpdatedBy = uuid.New()
	require.Equal(t, originalUpdater, *state.UpdatedBy)

	_, err = authorizationStateFromRecord(&generate.AuthorizationState{
		ID:              authorizationStateID,
		DesiredRevision: 0,
	})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
}

func TestAuthorizationLockStateRequiresTransactionAndUsesForUpdate(t *testing.T) {
	repository, generated, sqlMock, cleanup := newAuthorizationPolicySQLMock(t)
	defer cleanup()

	_, err := repository.LockState(context.Background())
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
	require.NoError(t, sqlMock.ExpectationsWereMet(), "ordinary context must execute no SQL")

	now := time.Now().UTC()
	updater := uuid.New()
	sqlMock.ExpectBegin()
	sqlMock.ExpectQuery(`SELECT .* FROM "authorization_states".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows(authorizationstate.Columns).AddRow(
			authorizationStateID,
			now,
			now,
			int64(9),
			updater,
		))
	sqlMock.ExpectCommit()

	var state ports.AuthorizationState
	err = platformtx.NewEntUnitOfWork(generated).Within(context.Background(), func(txCtx context.Context) error {
		var lockErr error
		state, lockErr = repository.LockState(txCtx)
		return lockErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(9), state.DesiredRevision)
	require.NotNil(t, state.UpdatedBy)
	require.Equal(t, updater, *state.UpdatedBy)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestAuthorizationStateMissingIsNotInitialized(t *testing.T) {
	repository, _, sqlMock, cleanup := newAuthorizationPolicySQLMock(t)
	defer cleanup()

	sqlMock.ExpectQuery(`SELECT .* FROM "authorization_states"`).
		WillReturnRows(sqlmock.NewRows(authorizationstate.Columns))
	_, err := repository.State(context.Background())
	require.ErrorIs(t, err, ports.ErrAuthorizationStateNotInitialized)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestAuthorizationListRulesReturnsCanonicalDeterministicCopy(t *testing.T) {
	repository, _, sqlMock, cleanup := newAuthorizationPolicySQLMock(t)
	defer cleanup()
	userSubject := "user:" + uuid.NewString()

	sqlMock.ExpectQuery(`SELECT .* FROM "casbin_rules".*ORDER BY`).
		WillReturnRows(sqlmock.NewRows(casbinrule.Columns).
			AddRow(1, "p", "role:teacher", "submission", "read", "", "", "").
			AddRow(2, "g", userSubject, "role:teacher", "", "", "", "").
			AddRow(3, "p", "role:teacher", "problem", "read", "", "", ""))
	rules, err := repository.ListRules(context.Background())
	require.NoError(t, err)
	require.Equal(t, []authcasbin.PolicyRule{
		{Type: "g", V0: userSubject, V1: "role:teacher"},
		{Type: "p", V0: "role:teacher", V1: "problem", V2: "read"},
		{Type: "p", V0: "role:teacher", V1: "submission", V2: "read"},
	}, rules)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestAuthorizationEmptyActiveLookupsDoNotQuery(t *testing.T) {
	repository, _, sqlMock, cleanup := newAuthorizationPolicySQLMock(t)
	defer cleanup()

	roles, err := repository.ActiveRoleKeys(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, roles)
	require.Empty(t, roles)
	users, err := repository.ActiveUserIDs(context.Background(), []uuid.UUID{})
	require.NoError(t, err)
	require.NotNil(t, users)
	require.Empty(t, users)
	eligibleUsers, err := repository.AuthorizationEligibleUserIDs(
		context.Background(),
		[]uuid.UUID{},
	)
	require.NoError(t, err)
	require.NotNil(t, eligibleUsers)
	require.Empty(t, eligibleUsers)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestAuthorizationPolicyInputGuardsRunBeforeSQL(t *testing.T) {
	repository, _, sqlMock, cleanup := newAuthorizationPolicySQLMock(t)
	defer cleanup()
	ctx := context.Background()

	require.ErrorIs(t, repository.EnsureState(ctx, 0), ports.ErrInvalidAuthorizationPolicy)
	require.ErrorIs(t, repository.ReplaceRolePolicies(ctx, "admin", nil), ports.ErrInvalidAuthorizationPolicy)
	require.ErrorIs(t, repository.ReplaceUserRoles(ctx, "user:invalid", nil), ports.ErrInvalidAuthorizationPolicy)
	require.ErrorIs(t, repository.DeleteRoleReferences(ctx, "role:Admin"), ports.ErrInvalidAuthorizationPolicy)
	require.ErrorIs(t, repository.DeleteUserReferences(ctx, "user:"+uuid.Nil.String()), ports.ErrInvalidAuthorizationPolicy)
	require.ErrorIs(t, repository.AdvanceRevision(ctx, 1, 1, nil), ports.ErrInvalidAuthorizationPolicy)
	nilUpdater := uuid.Nil
	require.ErrorIs(t, repository.AdvanceRevision(ctx, 1, 2, &nilUpdater), ports.ErrInvalidAuthorizationPolicy)
	_, err := repository.ActiveRoleKeys(ctx, []string{"Admin"})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
	_, err = repository.ActiveUserIDs(ctx, []uuid.UUID{uuid.Nil})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
	_, err = repository.AuthorizationEligibleUserIDs(ctx, []uuid.UUID{uuid.Nil})
	require.ErrorIs(t, err, ports.ErrInvalidAuthorizationPolicy)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestAuthorizationPolicyProductionContractGuard(t *testing.T) {
	const sourceFile = "authorization_policy.go"
	parsed, err := parser.ParseFile(token.NewFileSet(), sourceFile, nil, 0)
	require.NoError(t, err)
	expectedMethods := map[string]bool{
		"EnsureState":          false,
		"State":                false,
		"LockState":            false,
		"ListRules":            false,
		"ReplaceRolePolicies":  false,
		"ReplaceUserRoles":     false,
		"DeleteRoleReferences": false,
		"DeleteUserReferences": false,
		"AdvanceRevision":      false,
		"ActiveRoleKeys":       false,
		"authorizationUserIDs": false,
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || function.Body == nil {
			continue
		}
		if _, expected := expectedMethods[function.Name.Name]; expected {
			expectedMethods[function.Name.Name] = authorizationMethodCallsContextDatabase(function.Body)
		}
	}
	for method, resolved := range expectedMethods {
		require.Truef(t, resolved, "%s must resolve repository.client.DB(ctx)", method)
	}

	source, err := os.ReadFile(sourceFile)
	require.NoError(t, err)
	text := string(source)
	assert.Contains(t, text, "ForUpdate()", "LockState must issue SELECT FOR UPDATE")
	assert.Contains(t, text, "platformtx.Client(ctx, nil)", "LockState must require a platform transaction")
	assert.Contains(t, text, "CreateBulk", "replacement writes must use bounded bulk creation")
	for _, forbidden := range []string{
		"Schema.Create(",
		"EnableAutoSave(",
		"sql.Open(",
		"ent.Open(",
		"zap.",
		"slog.",
	} {
		assert.NotContains(t, text, forbidden)
	}
}

func TestAuthorizationPersistenceErrorDoesNotRenderPolicyOrPII(t *testing.T) {
	cause := errors.New("duplicate key (v0)=(user:raw-sensitive-id) (v1)=(role:admin)")
	err := authorizationPersistenceError("bulk create rules", cause)
	require.Error(t, err)
	require.ErrorIs(t, err, cause)
	assert.NotContains(t, err.Error(), "raw-sensitive-id")
	assert.NotContains(t, err.Error(), "role:admin")
}

func authorizationMethodCallsContextDatabase(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DB" {
			return true
		}
		argument, ok := call.Args[0].(*ast.Ident)
		if !ok || argument.Name != "ctx" {
			return true
		}
		client, ok := selector.X.(*ast.SelectorExpr)
		if !ok || client.Sel.Name != "client" {
			return true
		}
		receiver, ok := client.X.(*ast.Ident)
		found = ok && receiver.Name == "repository"
		return !found
	})
	return found
}

func newAuthorizationPolicySQLMock(
	t *testing.T,
) (*AuthorizationPolicyRepository, *generate.Client, sqlmock.Sqlmock, func()) {
	t.Helper()
	database, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	generated := generate.NewClient(generate.Driver(driver))
	repository := &AuthorizationPolicyRepository{
		client: dbent.WrapClient(generated, zap.NewNop()),
	}
	return repository, generated, sqlMock, func() {
		_ = generated.Close()
	}
}

func TestAuthorizationPolicyRepositorySatisfiesPort(t *testing.T) {
	require.Implements(t, (*ports.AuthorizationPolicyRepository)(nil), (*AuthorizationPolicyRepository)(nil))
}

func TestAuthorizationSchemaMatchesPinnedEntAdapter(t *testing.T) {
	// Production deliberately avoids ent-adapter constructors because v1.4.0
	// performs runtime Schema.Create. This compatibility fence keeps Judgify's
	// migration-owned table wire-compatible with the pinned adapter contract.
	require.Equal(t, adaptercasbinrule.Table, casbinrule.Table)
	require.Equal(t, adaptercasbinrule.Columns, casbinrule.Columns)
}
