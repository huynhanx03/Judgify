package eventing

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	entschema "github.com/huynhanx03/judgify/internal/ent/schema"
	identitycontract "github.com/huynhanx03/judgify/internal/identity/contract"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

func TestOutboxRejectsInvalidClosedMetadata(t *testing.T) {
	tests := map[string]func(*Event){
		"id":                func(event *Event) { event.ID = uuid.Nil },
		"type version":      func(event *Event) { event.Type = "identity.user.created" },
		"aggregate type":    func(event *Event) { event.AggregateType = "Identity/User" },
		"aggregate id":      func(event *Event) { event.AggregateID = uuid.Nil },
		"aggregate version": func(event *Event) { event.AggregateVersion = 0 },
		"correlation":       func(event *Event) { event.CorrelationID = "unsafe\nvalue" },
		"causation":         func(event *Event) { zero := uuid.Nil; event.CausationID = &zero },
		"occurred at":       func(event *Event) { event.OccurredAt = time.Time{} },
		"object payload":    func(event *Event) { event.Payload = []byte(`[1,2,3]`) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			event := validEvent()
			mutate(&event)
			_, err := prepareEvents(context.Background(), []Event{event})
			require.ErrorIs(t, err, ErrInvalidEvent)
		})
	}
}

func TestOutboxAcceptsProfilePrivacyAggregateType(t *testing.T) {
	event := validEvent()
	event.Type = identitycontract.EventProfileVisibilityChangedV1
	event.AggregateType = identitycontract.AggregateUserProfilePrivacy

	_, err := prepareEvents(context.Background(), []Event{event})
	require.NoError(t, err)
}

func TestOutboxCanonicalizesPayloadDeterministically(t *testing.T) {
	first := validEvent()
	first.Payload = []byte(`{ "z": 1, "nested": {"b":true,"a":false}, "a": 2 }`)
	second := validEvent()
	second.ID = uuid.MustParse("00000000-0000-7000-8000-000000000012")
	second.AggregateVersion = 2
	second.Payload = []byte(`{"a":2,"nested":{"a":false,"b":true},"z":1}`)

	preparedFirst, err := prepareEvents(context.Background(), []Event{first})
	require.NoError(t, err)
	preparedSecond, err := prepareEvents(context.Background(), []Event{second})
	require.NoError(t, err)
	require.True(t, bytes.Equal(preparedFirst[0].payload, preparedSecond[0].payload))
	require.Equal(t, `{"a":2,"nested":{"a":false,"b":true},"z":1}`, string(preparedFirst[0].payload))
}

func TestOutboxRejectsDuplicateAggregateEventIdentityBeforeWriting(t *testing.T) {
	first := validEvent()
	second := validEvent()
	second.ID = uuid.MustParse("00000000-0000-7000-8000-000000000099")

	_, err := prepareEvents(context.Background(), []Event{first, second})
	require.ErrorIs(t, err, ErrDuplicateEvent)
}

func TestOutboxAppenderRequiresTransactionContext(t *testing.T) {
	err := NewOutbox().Append(context.Background(), validEvent())
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
}

func TestOutboxInsertRollsBackWithEnclosingMutation(t *testing.T) {
	client, mock := newOutboxMockClient(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "casbin_rules"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO "outbox_events"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(validEvent().ID.String()))
	mock.ExpectRollback()

	rollbackCause := errors.New("reject mutation")
	err := platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		txClient, resolveErr := platformtx.Client(txCtx, nil)
		require.NoError(t, resolveErr)
		_, createErr := txClient.CasbinRule.Create().SetPtype("p").SetV0("role").Save(txCtx)
		require.NoError(t, createErr)
		require.NoError(t, NewOutbox().Append(txCtx, validEvent()))
		return rollbackCause
	})
	require.ErrorIs(t, err, rollbackCause)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxSchemaHasImmutableIdentityAndRelayState(t *testing.T) {
	fields := make(map[string]bool)
	for _, outboxField := range (entschema.OutboxEvent{}).Fields() {
		descriptor := outboxField.Descriptor()
		fields[descriptor.Name] = descriptor.Immutable
	}
	for _, name := range []string{
		"event_type", "aggregate_type", "aggregate_id", "aggregate_version",
		"correlation_id", "causation_id", "payload", "occurred_at", "created_at",
	} {
		immutable, ok := fields[name]
		require.Truef(t, ok, "missing immutable event field %q", name)
		require.Truef(t, immutable, "event field %q must be immutable", name)
	}
	for _, name := range []string{
		"state", "attempt_count", "lease_owner", "lease_token", "leased_until",
		"next_attempt_at", "last_attempt_at", "published_at", "dead_at",
		"publish_reference", "last_error_code", "ambiguous",
	} {
		immutable, ok := fields[name]
		require.Truef(t, ok, "missing relay field %q", name)
		require.Falsef(t, immutable, "relay field %q must remain mutable", name)
	}

	var aggregateIdentityUnique bool
	for _, outboxIndex := range (entschema.OutboxEvent{}).Indexes() {
		descriptor := outboxIndex.Descriptor()
		if descriptor.Unique && equalStrings(descriptor.Fields, []string{
			"event_type", "aggregate_type", "aggregate_id", "aggregate_version",
		}) {
			aggregateIdentityUnique = true
		}
	}
	require.True(t, aggregateIdentityUnique, "aggregate event identity needs a database unique index")
}

func validEvent() Event {
	return Event{
		ID:               uuid.MustParse("00000000-0000-7000-8000-000000000011"),
		Type:             "identity.user.created.v1",
		AggregateType:    "user",
		AggregateID:      uuid.MustParse("00000000-0000-7000-8000-000000000021"),
		AggregateVersion: 1,
		CorrelationID:    "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
		Payload:          []byte(`{"username":"alice","revision":1}`),
		OccurredAt:       time.Date(2026, time.July, 19, 8, 0, 0, 0, time.UTC),
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func newOutboxMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}
