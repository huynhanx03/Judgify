package audit

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

func TestParseSearchRequestRejectsMalformedAndUnboundedInputs(t *testing.T) {
	t.Parallel()

	validCursor, err := encodeCursor(Cursor{
		OccurredAt: time.Date(2026, time.July, 22, 8, 0, 0, 123, time.UTC),
		ID:         uuid.MustParse("0198330b-3400-7000-8000-000000000001"),
	})
	require.NoError(t, err)

	tests := map[string]SearchRequest{
		"invalid actor":       {ActorUserID: "not-a-uuid"},
		"invalid action":      {Action: "role read"},
		"invalid resource":    {Resource: " role"},
		"invalid resource id": {ResourceID: "target\nvalue"},
		"invalid cid":         {CorrelationID: "cid with spaces"},
		"invalid from":        {From: "yesterday"},
		"invalid to":          {To: "tomorrow"},
		"reversed window": {
			From: "2026-07-23T00:00:00Z",
			To:   "2026-07-22T00:00:00Z",
		},
		"zero limit":        {Limit: "0"},
		"oversize limit":    {Limit: "101"},
		"non numeric limit": {Limit: "many"},
		"invalid cursor":    {Cursor: "not-a-cursor"},
		"cursor before window": {
			From:   "2026-07-23T00:00:00Z",
			Cursor: validCursor,
		},
	}
	for name, request := range tests {
		request := request
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, parseErr := ParseSearchRequest(request)
			require.ErrorIs(t, parseErr, ErrInvalidSearch)
		})
	}
}

func TestParseSearchRequestDefaultsAndCursorRoundTrip(t *testing.T) {
	t.Parallel()

	cursor := Cursor{
		OccurredAt: time.Date(2026, time.July, 22, 8, 0, 0, 987654321, time.FixedZone("ICT", 7*60*60)),
		ID:         uuid.MustParse("0198330b-3400-7000-8000-000000000002"),
	}
	encoded, err := encodeCursor(cursor)
	require.NoError(t, err)

	filter, err := ParseSearchRequest(SearchRequest{
		ActorUserID:   "0198330b-3400-7000-8000-000000000003",
		Action:        "identity.role.replace",
		Resource:      "authorization_policy",
		ResourceID:    "role:moderator",
		CorrelationID: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
		From:          "2026-07-21T00:00:00+07:00",
		To:            "2026-07-24T00:00:00+07:00",
		Cursor:        encoded,
	})
	require.NoError(t, err)
	require.Equal(t, DefaultSearchLimit, filter.Limit)
	require.Equal(t, cursor.ID, filter.Cursor.ID)
	require.True(t, cursor.OccurredAt.UTC().Equal(filter.Cursor.OccurredAt))
	require.Equal(t, time.Date(2026, 7, 20, 17, 0, 0, 0, time.UTC), *filter.From)
	require.Equal(t, time.Date(2026, 7, 23, 17, 0, 0, 0, time.UTC), *filter.To)
}

func TestCursorRejectsTimestampsOutsidePostgresRange(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("0198330b-3400-7000-8000-000000000004")
	tests := map[string]time.Time{
		"before finite range": time.Unix(-210866803200, 0).UTC().Add(-time.Nanosecond),
		"at exclusive end":    time.Unix(9224318016000, 0).UTC(),
	}
	for name, occurredAt := range tests {
		occurredAt := occurredAt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			forged := uncheckedAuditCursor(occurredAt, id)
			_, err := decodeCursor(forged)
			require.ErrorIs(t, err, ErrInvalidSearch)

			_, err = ParseSearchRequest(SearchRequest{Cursor: forged})
			require.ErrorIs(t, err, ErrInvalidSearch)

			_, err = encodeCursor(Cursor{OccurredAt: occurredAt, ID: id})
			require.ErrorIs(t, err, ErrInvalidSearch)

			err = (Filter{
				Cursor: Cursor{OccurredAt: occurredAt, ID: id},
				Limit:  DefaultSearchLimit,
			}).Validate()
			require.ErrorIs(t, err, ErrInvalidSearch)
		})
	}
}

func TestCursorAcceptsPostgresFiniteBoundaryTimestamps(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("0198330b-3400-7000-8000-000000000005")
	for _, occurredAt := range []time.Time{
		time.Unix(-210866803200, 0).UTC(),
		time.Unix(9224318016000, 0).UTC().Add(-time.Nanosecond),
	} {
		encoded, err := encodeCursor(Cursor{OccurredAt: occurredAt, ID: id})
		require.NoError(t, err)
		decoded, err := decodeCursor(encoded)
		require.NoError(t, err)
		require.Equal(t, id, decoded.ID)
		require.True(t, occurredAt.Equal(decoded.OccurredAt))
	}
}

func TestQueryUsesDeterministicKeysetAndDefensivelyRedactsStoredDocuments(t *testing.T) {
	t.Parallel()

	client, mock := newAuditReadMockClient(t)
	occurredAt := time.Date(2026, time.July, 22, 8, 0, 0, 0, time.UTC)
	firstID := uuid.MustParse("0198330b-3400-7000-8000-000000000010")
	secondID := uuid.MustParse("0198330b-3400-7000-8000-00000000000f")
	actorID := uuid.MustParse("0198330b-3400-7000-8000-000000000020")

	rows := sqlmock.NewRows([]string{
		"id", "actor_user_id", "action", "resource", "resource_id", "reason",
		"before_data", "after_data", "correlation_id", "occurred_at",
	}).
		AddRow(
			firstID, actorID, "identity.role.replace", "authorization_policy", "role:moderator", "approved",
			json.RawMessage(`{"username":"alice","password":"must-never-leak","actor_session_id":"session-before"}`),
			json.RawMessage(`{"nested":{"refresh_token":"also-secret","actorSessionId":"session-after","session_id":"generic-session-after","target_session_id":"target-session-remains-auditable"}}`),
			"018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17", occurredAt,
		).
		AddRow(
			secondID, nil, "identity.role.replace", "authorization_policy", "role:moderator", "approved",
			json.RawMessage(`{}`), json.RawMessage(`{}`),
			"018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17", occurredAt,
		)
	mock.ExpectQuery(`SELECT .* FROM "audit_logs" WHERE .*"actor_user_id".*"action".*"resource".*"resource_id".*"correlation_id".*"occurred_at".*"id" <.* ORDER BY .*"occurred_at" DESC.*"id" DESC LIMIT 2`).
		WillReturnRows(rows)

	reader, err := NewQuery(client)
	require.NoError(t, err)
	page, err := reader.Search(context.Background(), Filter{
		ActorUserID:   &actorID,
		Action:        "identity.role.replace",
		Resource:      "authorization_policy",
		ResourceID:    "role:moderator",
		CorrelationID: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
		From:          timePointer(occurredAt.Add(-time.Hour)),
		To:            timePointer(occurredAt.Add(time.Hour)),
		Cursor: Cursor{
			OccurredAt: occurredAt.Add(time.Minute),
			ID:         uuid.MustParse("0198330b-3400-7000-8000-000000000099"),
		},
		Limit: 1,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.True(t, page.HasMore)
	require.NotEmpty(t, page.NextCursor)
	require.Equal(t, "[REDACTED]", page.Items[0].Before["password"])
	require.Equal(t, "[REDACTED]", page.Items[0].Before["actor_session_id"])
	require.Equal(t, "alice", page.Items[0].Before["username"])
	nested, ok := page.Items[0].After["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", nested["refresh_token"])
	require.Equal(t, "[REDACTED]", nested["actorSessionId"])
	require.Equal(t, "[REDACTED]", nested["session_id"])
	require.Equal(t, "target-session-remains-auditable", nested["target_session_id"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryFailsClosedForUnsafeStoredAuditDocument(t *testing.T) {
	t.Parallel()

	client, mock := newAuditReadMockClient(t)
	rows := sqlmock.NewRows([]string{
		"id", "actor_user_id", "action", "resource", "resource_id", "reason",
		"before_data", "after_data", "correlation_id", "occurred_at",
	}).AddRow(
		uuid.MustParse("0198330b-3400-7000-8000-000000000010"), nil,
		"identity.role.replace", "authorization_policy", "role:moderator", "approved",
		json.RawMessage(`{"duplicate":1,"duplicate":2}`), json.RawMessage(`{}`),
		"018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17", time.Now().UTC(),
	)
	mock.ExpectQuery(`SELECT .* FROM "audit_logs"`).WillReturnRows(rows)

	reader, err := NewQuery(client)
	require.NoError(t, err)
	_, err = reader.Search(context.Background(), Filter{Limit: 1})
	require.ErrorIs(t, err, ErrUnsafeStoredRecord)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryLogsOnlyCorrelationIDCountsAndFilterPresence(t *testing.T) {
	t.Parallel()

	client, mock := newAuditReadMockClient(t)
	mock.ExpectQuery(`SELECT .* FROM "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "actor_user_id", "action", "resource", "resource_id", "reason",
		"before_data", "after_data", "correlation_id", "occurred_at",
	}))
	reader, err := NewQuery(client)
	require.NoError(t, err)

	core, observed := observer.New(zap.DebugLevel)
	ctx := logger.WithContext(context.Background(), zap.New(core))
	ctx = correlation.WithContext(ctx, "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17")
	page, err := reader.Search(ctx, Filter{
		Action: "identity.role.replace",
		Limit:  1,
	})
	require.NoError(t, err)
	require.Empty(t, page.Items)
	require.Equal(t, 1, observed.Len())
	fields := observed.All()[0].ContextMap()
	require.Equal(t, "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17", fields["correlation_id"])
	require.Equal(t, true, fields["filter_action"])
	require.NotContains(t, fields, "action")
	for _, value := range fields {
		require.NotEqual(t, "identity.role.replace", value)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func timePointer(value time.Time) *time.Time { return &value }

func uncheckedAuditCursor(occurredAt time.Time, id uuid.UUID) string {
	payload := make([]byte, cursorBinaryBytes)
	payload[0] = cursorVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(occurredAt.Unix()))
	binary.BigEndian.PutUint32(payload[9:13], uint32(occurredAt.Nanosecond()))
	copy(payload[13:], id[:])
	return base64.RawURLEncoding.EncodeToString(payload)
}

func newAuditReadMockClient(t *testing.T) (*internalent.EntClient, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	generated := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = generated.Close() })
	return internalent.WrapClient(generated, nil), mock
}
