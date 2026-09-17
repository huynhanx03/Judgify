package migration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/DATA-DOG/go-sqlmock"
)

const postgresParametersQuery = `SELECT current_setting('server_version_num'), current_setting('default_table_access_method', true), current_setting('crdb_version', true)`

func TestPendingFilesCommitDDLAndRevisionLedgerAtomically(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	directory, file, statements, hash := transactionalFixture(t)
	partialHash := statementHash(statements[0])
	runner := &Runner{operator: "judgify/test"}

	mock.ExpectBegin()
	mock.ExpectQuery(postgresParametersQuery).WillReturnRows(
		sqlmock.NewRows([]string{"server_version_num", "default_table_access_method", "crdb_version"}).
			AddRow("160000", "heap", nil),
	)
	expectMissingRevision(mock, file.Version())
	expectRevisionWrite(mock, file, hash, 0, len(statements), []byte("[]"), "", "")
	mock.ExpectExec(statements[0]).WillReturnResult(sqlmock.NewResult(0, 0))
	expectRevisionWrite(
		mock,
		file,
		hash,
		1,
		len(statements),
		[]byte(`["`+partialHash+`"]`),
		"",
		"",
	)
	expectRevisionWrite(mock, file, hash, len(statements), len(statements), []byte("[]"), "", "")
	mock.ExpectCommit()

	if err := runner.executePendingTransactional(
		context.Background(),
		database,
		directory,
		nil,
		[]atlasmigrate.File{file},
		atlasmigrate.NopLogger{},
	); err != nil {
		t.Fatalf("execute transactional migration: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("transactional migration SQL: %v", err)
	}
}

func TestBoundedExecQuerierKeepsQueryContextAliveThroughRowsScan(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	mock.ExpectBegin()
	transaction, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() { _ = transaction.Rollback() })

	const query = "SELECT current_setting('server_version_num')"
	mock.ExpectQuery(query).WillReturnRows(
		sqlmock.NewRows([]string{"server_version_num"}).AddRow("160000"),
	)
	querier := &boundedExecQuerier{
		ctx:         context.Background(),
		transaction: transaction,
	}
	rows, err := querier.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatalf("query metadata: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("metadata row unavailable after QueryContext returned: %v", rows.Err())
	}
	var version string
	if err := rows.Scan(&version); err != nil {
		t.Fatalf("scan metadata after QueryContext returned: %v", err)
	}
	if version != "160000" {
		t.Fatalf("server version = %q", version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("metadata query SQL: %v", err)
	}
}

func TestFailedFileRollsBackDDLAndRevisionLedgerTogether(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	directory, file, statements, hash := transactionalFixture(t)
	runner := &Runner{operator: "judgify/test"}
	driverError := errors.New("database rejected DDL")

	mock.ExpectBegin()
	mock.ExpectQuery(postgresParametersQuery).WillReturnRows(
		sqlmock.NewRows([]string{"server_version_num", "default_table_access_method", "crdb_version"}).
			AddRow("160000", "heap", nil),
	)
	expectMissingRevision(mock, file.Version())
	expectRevisionWrite(mock, file, hash, 0, len(statements), []byte("[]"), "", "")
	mock.ExpectExec(statements[0]).WillReturnError(driverError)
	expectRevisionWrite(
		mock,
		file,
		hash,
		0,
		len(statements),
		[]byte("[]"),
		"migration execution failed",
		"statement 1 redacted",
	)
	mock.ExpectRollback()

	err = runner.executePendingTransactional(
		context.Background(),
		database,
		directory,
		nil,
		[]atlasmigrate.File{file},
		atlasmigrate.NopLogger{},
	)
	if !errors.Is(err, driverError) {
		t.Fatalf("failed transactional migration error = %v, want driver error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("rolled-back migration SQL: %v", err)
	}
}

func transactionalFixture(
	t *testing.T,
) (*atlasmigrate.MemDir, atlasmigrate.File, []string, string) {
	t.Helper()
	directory := checkedDirectory(t, map[string]string{
		"20260719000000_initial.sql": "CREATE TABLE example (id bigint PRIMARY KEY);",
	})
	file := migrationFile(t, directory, "20260719000000")
	statements, err := file.Stmts()
	if err != nil {
		t.Fatalf("parse transactional fixture: %v", err)
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum transactional fixture: %v", err)
	}
	hash, err := checksum.SumByName(file.Name())
	if err != nil {
		t.Fatalf("lookup transactional fixture checksum: %v", err)
	}
	return directory, file, statements, hash
}

func expectMissingRevision(mock sqlmock.Sqlmock, version string) {
	query := `SELECT ` + selectRevisionColumns + `
FROM "public"."atlas_schema_revisions"
WHERE "version" = $1`
	mock.ExpectQuery(query).
		WithArgs(version).
		WillReturnRows(sqlmock.NewRows(revisionColumnNames()))
}

func expectRevisionWrite(
	mock sqlmock.Sqlmock,
	file atlasmigrate.File,
	hash string,
	applied int,
	total int,
	partialHashes []byte,
	failure string,
	failureStatement string,
) {
	mock.ExpectExec(upsertRevisionSQL).
		WithArgs(
			file.Version(),
			file.Desc(),
			int64(atlasmigrate.RevisionTypeExecute),
			applied,
			total,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			failure,
			failureStatement,
			hash,
			partialHashes,
			"judgify/test",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func statementHash(statement string) string {
	digest := sha256.Sum256([]byte(statement))
	return "h1:" + base64.StdEncoding.EncodeToString(digest[:])
}
