package eventing

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	generatedoutbox "github.com/huynhanx03/judgify/internal/ent/generate/outboxevent"
)

func TestStoreClaimUsesAtomicSkipLockedLease(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 19, 5, 0, 0, 0, time.UTC)
	store.clock = func() time.Time { return now }
	store.token = func() string { return "00000000-0000-7000-8000-000000000099" }

	eventID := uuid.MustParse("00000000-0000-7000-8000-000000000011")
	aggregateID := uuid.MustParse("00000000-0000-7000-8000-000000000021")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "outbox_events".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows(generatedoutbox.Columns).AddRow(
			eventID.String(),
			"submission.judge.requested.v1",
			"submission",
			aggregateID.String(),
			int64(1),
			"018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
			nil,
			[]byte(`{"submission_id":"00000000-0000-7000-8000-000000000021"}`),
			now.Add(-time.Second),
			now.Add(-time.Second),
			"pending",
			uint32(0),
			nil,
			nil,
			nil,
			now.Add(-time.Second),
			nil,
			nil,
			nil,
			nil,
			nil,
			false,
		))
	mock.ExpectExec(`UPDATE "outbox_events" SET .* WHERE .*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	claimed, err := store.Claim(context.Background(), commonoutbox.ClaimRequest{
		Owner:         "worker-1",
		Limit:         1,
		LeaseDuration: 30 * time.Second,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, eventID.String(), claimed[0].Lease.MessageID)
	require.Equal(t, uint32(1), claimed[0].Lease.Attempt)
	require.Equal(t, "submission.judge.requested.v1", claimed[0].Message.Destination)
	require.Equal(t, aggregateID.String(), string(claimed[0].Message.Key))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreClaimReturnsNoMessagesWhenCommitFails(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 19, 5, 0, 0, 0, time.UTC)
	store.clock = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "outbox_events".*FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows(generatedoutbox.Columns))
	commitErr := errors.New("commit outcome unknown")
	mock.ExpectCommit().WillReturnError(commitErr)

	claimed, err := store.Claim(context.Background(), commonoutbox.ClaimRequest{
		Owner:         "worker-1",
		Limit:         1,
		LeaseDuration: time.Second,
	})
	require.Nil(t, claimed)
	require.ErrorIs(t, err, commitErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreClaimRestrictsRowsToRequestedDestinations(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	store.clock = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "outbox_events".*"event_type" IN \(\$[0-9]+\).*FOR UPDATE SKIP LOCKED`).
		WithArgs(
			generatedoutbox.StatePending,
			now,
			generatedoutbox.StateLeased,
			now,
			"problem.solve_status_changed.v1",
		).
		WillReturnRows(sqlmock.NewRows(generatedoutbox.Columns))
	mock.ExpectCommit()

	claimed, err := store.Claim(context.Background(), commonoutbox.ClaimRequest{
		Owner:         "worker-1",
		Limit:         1,
		LeaseDuration: 30 * time.Second,
		Destinations:  []string{"problem.solve_status_changed.v1"},
	})
	require.NoError(t, err)
	require.Empty(t, claimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreFinalizationIsFencedByMessageTokenAndAttempt(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)
	store.clock = func() time.Time {
		return time.Date(2026, time.July, 19, 5, 0, 0, 0, time.UTC)
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "outbox_events"`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err = store.MarkPublished(context.Background(), commonoutbox.Lease{
		MessageID: "00000000-0000-7000-8000-000000000011",
		Token:     "00000000-0000-7000-8000-000000000099",
		Attempt:   2,
	}, commonoutbox.PublishAck{})
	require.ErrorIs(t, err, commonoutbox.ErrLeaseLost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreReplayDeadUsesObservedAttemptFenceAndFreshBudget(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	store.clock = func() time.Time { return now }
	eventID := uuid.MustParse("00000000-0000-7000-8000-000000000011")

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "outbox_events"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	replayed, err := store.ReplayDead(context.Background(), ReplayDeadRequest{
		EventID: eventID, ExpectedAttempts: 20,
	})
	require.NoError(t, err)
	require.True(t, replayed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreReplayDeadReturnsFalseWhenFenceChanged(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)
	store.clock = func() time.Time {
		return time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	}
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "outbox_events"`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	replayed, err := store.ReplayDead(context.Background(), ReplayDeadRequest{
		EventID:          uuid.MustParse("00000000-0000-7000-8000-000000000011"),
		ExpectedAttempts: 20,
	})
	require.NoError(t, err)
	require.False(t, replayed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreRejectsUnboundedOperationsBeforeDatabaseAccess(t *testing.T) {
	client, mock := newStoreMockClient(t)
	store, err := NewStore(client)
	require.NoError(t, err)

	_, err = store.Claim(context.Background(), commonoutbox.ClaimRequest{
		Owner:         "worker",
		Limit:         maximumClaimLimit + 1,
		LeaseDuration: time.Second,
	})
	require.ErrorIs(t, err, ErrInvalidStoreOperation)
	_, err = store.Claim(context.Background(), commonoutbox.ClaimRequest{
		Owner:         "worker",
		Limit:         1,
		LeaseDuration: time.Second,
		Destinations:  []string{"problem.solve_status_changed.v1", "problem.solve_status_changed.v1"},
	})
	require.ErrorIs(t, err, ErrInvalidStoreOperation)
	err = store.MarkRetry(context.Background(), commonoutbox.Lease{
		MessageID: uuid.NewString(), Token: uuid.NewString(), Attempt: 1,
	}, commonoutbox.RetryDisposition{Delay: maximumRetryDelay + time.Second, Code: "publish"})
	require.ErrorIs(t, err, ErrInvalidStoreOperation)
	_, err = store.ReplayDead(context.Background(), ReplayDeadRequest{})
	require.ErrorIs(t, err, ErrInvalidStoreOperation)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newStoreMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}
