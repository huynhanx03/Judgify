package ent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"entgo.io/ent/dialect"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/database/ent"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/pkg/settings"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/tenantctx"
)

// EntClient wraps the generated client and resolves the single platform
// transaction scope used by identity, audit, outbox, judge, and submissions.
type EntClient struct {
	client *generate.Client
	driver dialect.Driver
	log    *zap.Logger
}

// NewEntClient creates a new EntClient.
func NewEntClient(dbSettings settings.Database, debug bool, log *zap.Logger) (*EntClient, error) {
	baseDriver, err := ent.NewDriver(dbSettings)
	if err != nil {
		return nil, err
	}
	var driver dialect.Driver = baseDriver
	if debug {
		// Ent's generated Debug logger includes raw query text and arguments.
		// The shared driver wrapper retains contextual timing while exposing
		// only a statement hash and operation name.
		driver = ent.WrapLogging(commonlogger.Wrap(log), driver, 0)
	}
	client := generate.NewClient(generate.Driver(driver))
	installTenantIsolation(client)

	return &EntClient{client: client, driver: driver, log: log}, nil
}

// WrapClient wraps an existing generated client.
func WrapClient(client *generate.Client, log *zap.Logger) *EntClient {
	installTenantIsolation(client)
	return &EntClient{client: client, log: log}
}

// WrapClientWithDriver records the already-owned Ent driver for read-only
// adapters that must share the process database pool. The EntClient remains
// the sole driver lifecycle owner.
func WrapClientWithDriver(
	client *generate.Client,
	driver dialect.Driver,
	log *zap.Logger,
) *EntClient {
	installTenantIsolation(client)
	return &EntClient{client: client, driver: driver, log: log}
}

// DialectDriver exposes the concrete process-owned driver to adapters which
// cannot operate through an Ent entity client. The logging/observation wrapper
// is removed so callers can use vendor APIs such as *entsql.Driver.DB.
// Consequently, statements executed through the returned driver or its
// *sql.DB do not pass through the generic Ent statement observer. Callers must
// never close it or use it for runtime schema changes.
func (c *EntClient) DialectDriver() (dialect.Driver, bool) {
	if c == nil || c.driver == nil {
		return nil, false
	}
	return ent.UnwrapLogging(c.driver), true
}

func (c *EntClient) Client() *generate.Client {
	return c.client
}

func (c *EntClient) Close() error {
	return c.client.Close()
}

// DB returns the transactional client if present, otherwise the standard client.
func (c *EntClient) DB(ctx context.Context) *generate.Client {
	if c == nil || c.client == nil {
		panic(platformtx.ErrInvalidUnitOfWork)
	}
	client, err := platformtx.Client(ctx, c.client)
	if err != nil {
		// A closed transaction context is a programmer error. Panicking is
		// intentionally fail-closed: falling back to the root client here would
		// turn a post-rollback write into an independent committed mutation.
		panic(err)
	}
	if scope, ok := tenantctx.FromContext(ctx); ok && !scope.SQLBound() {
		if _, transactionErr := platformtx.Client(ctx, nil); transactionErr == nil {
			arrayValue, arrayErr := scope.PostgreSQLUUIDArray()
			if arrayErr != nil {
				panic(arrayErr)
			}
			if _, bindErr := client.ExecContext(ctx,
				"SELECT pg_catalog.set_config('app.tenant_ids', $1, true)", arrayValue); bindErr != nil {
				panic(fmt.Errorf("install tenant transaction scope: %w", bindErr))
			}
			scope.MarkSQLBound()
		}
	}
	return client
}

// GetOutboxEvent reads an idempotency receipt through a tenant-bound short
// transaction when the caller is outside a UnitOfWork. Outbox RLS uses the
// transaction-local tenant setting, so a tenant predicate alone is not enough.
func (c *EntClient) GetOutboxEvent(ctx context.Context, id uuid.UUID) (*generate.OutboxEvent, error) {
	if c == nil || c.client == nil || ctx == nil || id == uuid.Nil {
		return nil, platformtx.ErrInvalidUnitOfWork
	}
	if _, err := platformtx.Client(ctx, nil); err == nil {
		return c.DB(ctx).OutboxEvent.Get(ctx, id)
	} else if !errors.Is(err, platformtx.ErrTransactionRequired) {
		return nil, err
	}
	var record *generate.OutboxEvent
	err := ent.WithTx(ctx, c.client.Tx, func(transaction *generate.Tx) error {
		if _, scoped := tenantctx.FromContext(ctx); scoped {
			if err := tenantctx.BindSQLScope(ctx, transaction); err != nil {
				return err
			}
		}
		var err error
		record, err = transaction.Client().OutboxEvent.Get(ctx, id)
		return err
	})
	return record, err
}

// WithPrincipalRead runs an account-scoped control-plane read with the
// authenticated principal bound to transaction-local RLS settings.
func (c *EntClient) WithPrincipalRead(ctx context.Context, userID uuid.UUID, fn func(*generate.Client) error) error {
	if c == nil || c.client == nil || ctx == nil || userID == uuid.Nil || fn == nil {
		return platformtx.ErrInvalidUnitOfWork
	}
	if _, err := platformtx.Client(ctx, nil); err == nil {
		client := c.DB(ctx)
		if err := tenantctx.BindPrincipalSQL(ctx, userID, client); err != nil {
			return err
		}
		return fn(client)
	} else if !errors.Is(err, platformtx.ErrTransactionRequired) {
		return err
	}
	return ent.WithTx(ctx, c.client.Tx, func(transaction *generate.Tx) error {
		if _, scoped := tenantctx.FromContext(ctx); scoped {
			if err := tenantctx.BindSQLScope(ctx, transaction); err != nil {
				return err
			}
		}
		if err := tenantctx.BindPrincipalSQL(ctx, userID, transaction); err != nil {
			return err
		}
		return fn(transaction.Client())
	})
}

// DoInTx executes fn within a transaction.
func (c *EntClient) DoInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return (&entTxManager{client: c}).DoInTx(ctx, fn)
}

// entTxManager adapts EntClient to the tx.Manager interface.
type entTxManager struct {
	client  *EntClient
	options *sql.TxOptions
}

// NewTxManager creates a tx.Manager backed by the given EntClient.
func NewTxManager(client *EntClient) tx.Manager {
	return &entTxManager{client: client}
}

// NewReadSnapshotTxManager provides a stable snapshot for multi-query read
// models. PostgreSQL repeatable-read guarantees every statement observes the
// same committed snapshot; read-only prevents accidental mutations.
func NewReadSnapshotTxManager(client *EntClient) tx.Manager {
	return &entTxManager{
		client: client,
		options: &sql.TxOptions{
			Isolation: sql.LevelRepeatableRead,
			ReadOnly:  true,
		},
	}
}

func (m *entTxManager) DoInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m == nil || m.client == nil || m.client.client == nil || ctx == nil || fn == nil {
		return platformtx.ErrInvalidUnitOfWork
	}
	if _, err := platformtx.Client(ctx, nil); err == nil {
		// Nested service units join the existing physical transaction. The
		// direct platform UnitOfWork API remains strict and rejects nesting.
		return fn(ctx)
	} else if !errors.Is(err, platformtx.ErrTransactionRequired) {
		return err
	}
	if m.options != nil {
		return platformtx.NewEntUnitOfWorkWithOptions(m.client.client, *m.options).Within(ctx, fn)
	}
	return platformtx.NewEntUnitOfWork(m.client.client).Within(ctx, fn)
}

// RegisterAfterCommit is an optional capability discovered by services that
// need to invalidate process-local state only after the outermost commit.
func (m *entTxManager) RegisterAfterCommit(
	ctx context.Context,
	callback func(context.Context) error,
) error {
	return platformtx.AfterCommit(ctx, platformtx.AfterCommitFunc(callback))
}

// RegisterAfterCompletion supports commit/rollback publication fences without
// exposing Ent or the platform transaction package to application services.
func (m *entTxManager) RegisterAfterCompletion(
	ctx context.Context,
	callback func(context.Context, bool) error,
) error {
	return platformtx.RegisterAfterCompletion(ctx, platformtx.AfterCompletionFunc(callback))
}
