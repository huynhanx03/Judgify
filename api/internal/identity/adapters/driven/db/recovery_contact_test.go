package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ports"
)

func TestRecoveryContactGetStateRequiresExactLiveSession(t *testing.T) {
	client, mock := newRecoveryContactMockClient(t)
	repository := NewRecoveryContactRepository(dbent.WrapClient(client, zap.NewNop()))
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	_, err := repository.GetState(context.Background(), uuid.Nil, uuid.New(), now)
	require.Error(t, err)
	_, err = repository.GetState(context.Background(), uuid.New(), uuid.Nil, now)
	require.Error(t, err)
	_, err = repository.GetState(context.Background(), uuid.New(), uuid.New(), time.Time{})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecoveryContactMutationsRequireCallerTransaction(t *testing.T) {
	client, mock := newRecoveryContactMockClient(t)
	repository := NewRecoveryContactRepository(dbent.WrapClient(client, zap.NewNop()))
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	base := ports.RecoveryContactMutationCommand{
		ReceiptID: uuid.New(), UserID: uuid.New(), SessionID: uuid.New(), IdempotencyKey: uuid.New(),
		Kind:               ports.RecoveryContactCommandCancelPending,
		RequestFingerprint: make([]byte, 32), Now: now,
		RecentAfter: now.Add(-10 * time.Minute), ReceiptExpiresAt: now.Add(24 * time.Hour),
	}

	_, err := repository.CancelPending(context.Background(), ports.RecoveryContactVersionCommand{
		Command: base, ExpectedVersion: 1,
	})
	require.Error(t, err)
	_, err = repository.RemoveCurrent(context.Background(), ports.RecoveryContactVersionCommand{
		Command: func() ports.RecoveryContactMutationCommand {
			value := base
			value.Kind = ports.RecoveryContactCommandRemove
			return value
		}(),
		ExpectedVersion: 1,
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newRecoveryContactMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	driver := entsql.OpenDB(dialect.Postgres, database)
	return generate.NewClient(generate.Driver(driver)), mock
}

var _ ports.RecoveryContactManagementRepository = (*RecoveryContactRepository)(nil)
