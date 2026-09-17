// Package testutil holds small hand-written test doubles shared across the
// service test suites.
package testutil

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/tx"
)

// TxManager is a tx.Manager for tests: it runs fn inline, without opening a
// real transaction, so the code under test executes exactly as it would inside
// DoInTx. Call-through is still verified indirectly — if a service never calls
// DoInTx, fn never runs, the repo mocks inside it go unhit, and their
// expectations fail.
type TxManager struct{}

var _ tx.Manager = TxManager{}

// DoInTx runs fn with the given context and returns its error unchanged.
func (TxManager) DoInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
