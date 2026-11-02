package db

import (
	"context"
	"database/sql"
)

// sqlQueryer is implemented by both database/sql transactions and Ent's
// transaction-scoped generated clients. Ent deliberately exposes QueryContext
// rather than QueryRowContext, so this adapter keeps raw SQL reads on the same
// transaction without falling back to the process database handle.
type sqlQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type singleRow struct {
	rows *sql.Rows
	err  error
}

func querySingleRow(
	ctx context.Context,
	queryer sqlQueryer,
	statement string,
	arguments ...any,
) *singleRow {
	if ctx == nil || queryer == nil {
		return &singleRow{err: sql.ErrConnDone}
	}
	rows, err := queryer.QueryContext(ctx, statement, arguments...)
	return &singleRow{rows: rows, err: err}
}

func (row *singleRow) Scan(destinations ...any) error {
	if row == nil {
		return sql.ErrConnDone
	}
	if row.err != nil {
		return row.err
	}
	if row.rows == nil {
		return sql.ErrConnDone
	}
	defer func() { _ = row.rows.Close() }()
	if !row.rows.Next() {
		if err := row.rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := row.rows.Scan(destinations...); err != nil {
		return err
	}
	return row.rows.Err()
}
