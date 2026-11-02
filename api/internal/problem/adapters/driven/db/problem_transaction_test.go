package db

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var errProblemTransactionWrite = errors.New("transaction write probe")

func TestProblemCreateUsesTransactionScopedClient(t *testing.T) {
	repository, driver := newTransactionProbeProblemRepository(t)

	err := dbent.NewTxManager(repository.client).DoInTx(
		context.Background(),
		func(txCtx context.Context) error {
			return repository.Create(txCtx, validTransactionProbeProblem())
		},
	)

	require.Error(t, err)
	require.Zero(t, driver.rootWriteCount())
	require.Equal(t, 1, driver.tx.writeCount())
	require.Zero(t, driver.tx.commitCount())
	require.Equal(t, 1, driver.tx.rollbackCount())
}

func validTransactionProbeProblem() *entity.Problem {
	return &entity.Problem{
		ID:            uuid.New(),
		Slug:          "transaction-probe",
		Title:         "Transaction probe",
		Description:   "Verifies that writes remain inside their transaction.",
		DifficultyID:  uuid.New(),
		TimeLimitMs:   1000,
		MemoryLimitKb: 262144,
		AuthorID:      uuid.New(),
	}
}

func newTransactionProbeProblemRepository(
	t *testing.T,
) (*ProblemRepository, *transactionProbeDriver) {
	t.Helper()

	driver := &transactionProbeDriver{tx: &transactionProbeTx{}}
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	repository, ok := NewProblemRepository(dbent.WrapClient(client, nil)).(*ProblemRepository)
	require.True(t, ok)
	return repository, driver
}

type transactionProbeDriver struct {
	mu         sync.Mutex
	rootWrites int
	tx         *transactionProbeTx
}

func (d *transactionProbeDriver) Exec(context.Context, string, any, any) error {
	d.mu.Lock()
	d.rootWrites++
	d.mu.Unlock()
	return errors.New("root write probe")
}

func (d *transactionProbeDriver) Query(context.Context, string, any, any) error {
	d.mu.Lock()
	d.rootWrites++
	d.mu.Unlock()
	return errors.New("root write probe")
}

func (d *transactionProbeDriver) Tx(context.Context) (dialect.Tx, error) {
	return d.tx, nil
}

func (d *transactionProbeDriver) Close() error {
	return nil
}

func (d *transactionProbeDriver) Dialect() string {
	return dialect.Postgres
}

func (d *transactionProbeDriver) rootWriteCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rootWrites
}

type transactionProbeTx struct {
	mu        sync.Mutex
	writes    int
	commits   int
	rollbacks int
}

func (t *transactionProbeTx) Exec(context.Context, string, any, any) error {
	t.mu.Lock()
	t.writes++
	t.mu.Unlock()
	return errProblemTransactionWrite
}

func (t *transactionProbeTx) Query(context.Context, string, any, any) error {
	t.mu.Lock()
	t.writes++
	t.mu.Unlock()
	return errProblemTransactionWrite
}

func (t *transactionProbeTx) Commit() error {
	t.mu.Lock()
	t.commits++
	t.mu.Unlock()
	return nil
}

func (t *transactionProbeTx) Rollback() error {
	t.mu.Lock()
	t.rollbacks++
	t.mu.Unlock()
	return nil
}

func (t *transactionProbeTx) writeCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writes
}

func (t *transactionProbeTx) commitCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.commits
}

func (t *transactionProbeTx) rollbackCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rollbacks
}
