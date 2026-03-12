package ent

import (
	"context"
	"fmt"

	"github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/pkg/settings"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/adapters/driven/db/ent/generate/runtime"
)

type (
	// EntClient wraps the generated ent.Client with transaction support.
	EntClient struct {
		client *generate.Client
		log    *zap.Logger
	}

	// Tx interface for transaction operations.
	Tx interface {
		Client() *generate.Client
		OnRollback(f generate.RollbackHook)
		OnCommit(f generate.CommitHook)
	}

	TxKeyType string
)

const TXKey TxKeyType = "txKey"

// NewEntClient creates a new EntClient.
func NewEntClient(dbSettings settings.Database, debug bool, log *zap.Logger) (*EntClient, error) {
	driver, err := ent.NewDriver(dbSettings)
	if err != nil {
		return nil, err
	}

	client := generate.NewClient(generate.Driver(driver))

	if debug {
		client = client.Debug()
	}

	return &EntClient{client: client, log: log}, nil
}

// WrapClient wraps an existing generated client.
func WrapClient(client *generate.Client, log *zap.Logger) *EntClient {
	return &EntClient{client: client, log: log}
}

func (c *EntClient) Client() *generate.Client {
	return c.client
}

func (c *EntClient) Close() error {
	return c.client.Close()
}

// GetTx retrieves the transaction from context.
func GetTx(ctx context.Context) *generate.Tx {
	if tx, ok := ctx.Value(TXKey).(*generate.Tx); ok {
		return tx
	}
	return nil
}

// DB returns the transactional client if present, otherwise the standard client.
func (c *EntClient) DB(ctx context.Context) *generate.Client {
	if tx := GetTx(ctx); tx != nil {
		return tx.Client()
	}
	return c.client
}

// ClientTx aliases DB for backward compatibility.
func (c *EntClient) ClientTx(ctx context.Context) *generate.Client {
	return c.DB(ctx)
}

// DoInTx executes fn within a transaction.
func (c *EntClient) DoInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx := GetTx(ctx); tx != nil {
		return fn(ctx)
	}

	tx, err := c.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	// Inject transaction into context
	ctx = context.WithValue(ctx, TXKey, tx)

	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	if err := fn(ctx); err != nil {
		// Log rollback error if necessary
		if rerr := tx.Rollback(); rerr != nil {
			if c.log != nil {
				c.log.Error("failed to rollback transaction", zap.Error(rerr))
			}
			return fmt.Errorf("rolling back transaction: %v (original error: %w)", rerr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
