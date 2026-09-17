package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	entschema "github.com/huynhanx03/judgify/internal/ent/schema"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

func TestRepositoryRejectsIncompleteAuditEntries(t *testing.T) {
	repository := NewRepository()
	tests := map[string]func(*Entry){
		"action":   func(entry *Entry) { entry.Action = " " },
		"resource": func(entry *Entry) { entry.Resource = "" },
		"reason":   func(entry *Entry) { entry.Reason = "\t" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			entry := validEntry()
			mutate(&entry)
			_, err := repository.prepareEntry(context.Background(), entry)
			require.ErrorIs(t, err, ErrInvalidEntry)
		})
	}
}

func TestRepositoryRedactsDefaultAndConfiguredKeysRecursively(t *testing.T) {
	repository := NewRepository(WithRedactedKeys("api_key"))
	entry := validEntry()
	entry.Before = map[string]any{
		"username":         "alice",
		"password":         "never-store-me",
		"actor_session_id": "session-must-stay-internal",
		"nested": map[string]any{
			"apiKey":            "also-secret",
			"refresh_token":     "another-secret",
			"session_id":        "nested-session-must-stay-internal",
			"target_session_id": "target-session-is-the-audited-resource",
		},
	}

	prepared, err := repository.prepareEntry(context.Background(), entry)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"nested":{"apiKey":"[REDACTED]","refresh_token":"[REDACTED]","session_id":"[REDACTED]","target_session_id":"target-session-is-the-audited-resource"},
		"actor_session_id":"[REDACTED]",
		"password":"[REDACTED]",
		"username":"alice"
	}`, string(prepared.beforeJSON))
	require.NotContains(t, string(prepared.beforeJSON), "never-store-me")
	require.NotContains(t, string(prepared.beforeJSON), "also-secret")
	require.NotContains(t, string(prepared.beforeJSON), "session-must-stay-internal")
	require.Contains(t, string(prepared.beforeJSON), "target-session-is-the-audited-resource")
}

func TestRepositoryCapsAuditPayloadDepthAndBytes(t *testing.T) {
	repository := NewRepository()

	deep := map[string]any{"leaf": true}
	for range maxAuditDepth + 1 {
		deep = map[string]any{"next": deep}
	}
	entry := validEntry()
	entry.After = deep
	_, err := repository.prepareEntry(context.Background(), entry)
	require.ErrorIs(t, err, ErrEntryTooDeep)

	entry = validEntry()
	entry.After = map[string]any{"value": strings.Repeat("x", maxAuditDocumentBytes+1)}
	_, err = repository.prepareEntry(context.Background(), entry)
	require.ErrorIs(t, err, ErrEntryTooLarge)
}

func TestRepositoryRequiresTransactionContext(t *testing.T) {
	_, err := NewRepository().Append(context.Background(), validEntry())
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
}

func TestAuditInsertRollsBackWithEnclosingMutation(t *testing.T) {
	client, mock := newAuditMockClient(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "casbin_rules"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	rollbackCause := errors.New("reject mutation")
	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		txClient, resolveErr := platformtx.Client(txCtx, nil)
		require.NoError(t, resolveErr)
		_, createErr := txClient.CasbinRule.Create().SetPtype("p").SetV0("role").Save(txCtx)
		require.NoError(t, createErr)
		_, appendErr := NewRepository().Append(txCtx, validEntry())
		require.NoError(t, appendErr)
		return rollbackCause
	})
	require.ErrorIs(t, err, rollbackCause)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditSchemaIsAppendOnlyAtTheEntFieldBoundary(t *testing.T) {
	mixins := (entschema.AuditLog{}).Mixin()
	require.Len(t, mixins, 1)
	_, ok := mixins[0].(commonent.AppendOnlyMixin)
	require.True(t, ok, "audit schema must use go-common AppendOnlyMixin")

	for _, auditField := range (entschema.AuditLog{}).Fields() {
		descriptor := auditField.Descriptor()
		require.Truef(t, descriptor.Immutable, "audit field %q must be immutable", descriptor.Name)
	}
}

func TestGeneratedAuditClientRejectsMutableOperationsBeforeSQL(t *testing.T) {
	t.Parallel()

	client, mock := newAuditMockClient(t)
	_, err := client.AuditLog.Update().Save(context.Background())
	require.ErrorIs(t, err, commonent.ErrAppendOnlyMutation)

	_, err = client.AuditLog.Delete().Exec(context.Background())
	require.ErrorIs(t, err, commonent.ErrAppendOnlyMutation)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditSchemaIndexesResourceIDKeysetReads(t *testing.T) {
	t.Parallel()

	expected := []string{"resource_id", "occurred_at", "id"}
	for _, auditIndex := range (entschema.AuditLog{}).Indexes() {
		if slices.Equal(auditIndex.Descriptor().Fields, expected) {
			return
		}
	}
	require.Failf(t, "missing audit resource keyset index", "expected fields %v", expected)
}

func validEntry() Entry {
	return Entry{
		ID:            uuid.MustParse("00000000-0000-7000-8000-000000000001"),
		Action:        "identity.role.replace",
		Resource:      "authorization_policy",
		ResourceID:    "default",
		Reason:        "approved access change",
		Before:        map[string]any{"revision": int64(1)},
		After:         map[string]any{"revision": int64(2)},
		CorrelationID: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
		OccurredAt:    time.Date(2026, time.July, 19, 8, 0, 0, 0, time.UTC),
	}
}

func newAuditMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}
