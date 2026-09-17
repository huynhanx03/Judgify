package ent

import (
	"context"
	"database/sql"
	"errors"

	"entgo.io/ent/dialect"
	"github.com/huynhanx03/go-common/pkg/common/tx"
	"github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/pkg/settings"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
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
		driver = ent.WrapLogging(driver, 0)
	}
	client := generate.NewClient(generate.Driver(driver))

	return &EntClient{client: client, driver: driver, log: log}, nil
}

// WrapClient wraps an existing generated client.
func WrapClient(client *generate.Client, log *zap.Logger) *EntClient {
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
	return client
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
