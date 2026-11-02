package migration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
)

func TestRevisionHistoryVerifiesCompletedFileChecksums(t *testing.T) {
	t.Parallel()

	directory := checkedDirectory(t, map[string]string{
		"20260719000000_initial.sql": "CREATE TABLE example (id bigint PRIMARY KEY);",
	})
	revision := completedRevision(t, directory, "20260719000000")
	if err := validateRevisionHistory(context.Background(), directory, []*atlasmigrate.Revision{revision}, testStatements); err != nil {
		t.Fatalf("validate completed revision: %v", err)
	}

	revision.Hash = "different"
	err := validateRevisionHistory(context.Background(), directory, []*atlasmigrate.Revision{revision}, testStatements)
	if !errors.Is(err, ErrHistoryDrift) {
		t.Fatalf("changed completed revision hash error = %v, want ErrHistoryDrift", err)
	}
}

func TestRevisionHistoryRejectsGapsAndUnknownRevisions(t *testing.T) {
	t.Parallel()

	directory := checkedDirectory(t, map[string]string{
		"20260719000000_first.sql":  "CREATE TABLE first_table (id bigint PRIMARY KEY);",
		"20260719000001_second.sql": "CREATE TABLE second_table (id bigint PRIMARY KEY);",
		"20260719000002_third.sql":  "CREATE TABLE third_table (id bigint PRIMARY KEY);",
	})
	first := completedRevision(t, directory, "20260719000000")
	third := completedRevision(t, directory, "20260719000002")
	err := validateRevisionHistory(context.Background(), directory, []*atlasmigrate.Revision{first, third}, testStatements)
	if !errors.Is(err, ErrHistoryDrift) {
		t.Fatalf("history gap error = %v, want ErrHistoryDrift", err)
	}

	unknown := *first
	unknown.Version = "20260719000009"
	err = validateRevisionHistory(context.Background(), directory, []*atlasmigrate.Revision{&unknown}, testStatements)
	if !errors.Is(err, ErrHistoryDrift) {
		t.Fatalf("unknown revision error = %v, want ErrHistoryDrift", err)
	}
}

func TestRevisionHistoryRejectsMalformedPartialLedgerWithoutPanicking(t *testing.T) {
	t.Parallel()

	directory := checkedDirectory(t, map[string]string{
		"20260719000000_initial.sql": "CREATE TABLE example (id bigint PRIMARY KEY); ALTER TABLE example ADD COLUMN name text;",
	})
	revision := completedRevision(t, directory, "20260719000000")
	file := migrationFile(t, directory, revision.Version)
	statements, err := file.Stmts()
	if err != nil {
		t.Fatalf("parse migration fixture: %v", err)
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(statements[0]))
	revision.Applied = 1
	revision.PartialHashes = []string{"h1:" + base64.StdEncoding.EncodeToString(hasher.Sum(nil))}
	if err := validateRevisionHistory(context.Background(), directory, []*atlasmigrate.Revision{revision}, testStatements); err != nil {
		t.Fatalf("validate legitimate partial revision: %v", err)
	}

	revision.PartialHashes = nil
	err = validateRevisionHistory(context.Background(), directory, []*atlasmigrate.Revision{revision}, testStatements)
	if !errors.Is(err, ErrHistoryDrift) {
		t.Fatalf("missing partial hashes error = %v, want ErrHistoryDrift", err)
	}
}

func checkedDirectory(t *testing.T, statements map[string]string) *atlasmigrate.MemDir {
	t.Helper()
	directory, err := loadDirectory(context.Background(), checkedMigrationFS(t, statements))
	if err != nil {
		t.Fatalf("load checked migration fixture: %v", err)
	}
	return directory
}

func completedRevision(
	t *testing.T,
	directory *atlasmigrate.MemDir,
	version string,
) *atlasmigrate.Revision {
	t.Helper()
	file := migrationFile(t, directory, version)
	statements, err := file.Stmts()
	if err != nil {
		t.Fatalf("parse %s: %v", file.Name(), err)
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum directory: %v", err)
	}
	hash, err := checksum.SumByName(file.Name())
	if err != nil {
		t.Fatalf("checksum %s: %v", file.Name(), err)
	}
	return &atlasmigrate.Revision{
		Version:         file.Version(),
		Description:     file.Desc(),
		Type:            atlasmigrate.RevisionTypeExecute,
		Applied:         len(statements),
		Total:           len(statements),
		ExecutedAt:      time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC),
		ExecutionTime:   time.Second,
		Hash:            hash,
		OperatorVersion: "judgify/test",
	}
}

func migrationFile(t *testing.T, directory *atlasmigrate.MemDir, version string) atlasmigrate.File {
	t.Helper()
	files, err := directory.Files()
	if err != nil {
		t.Fatalf("list migration fixtures: %v", err)
	}
	for _, file := range files {
		if file.Version() == version {
			return file
		}
	}
	t.Fatalf("migration fixture version %s not found", version)
	return nil
}

func testStatements(file atlasmigrate.File) ([]string, error) {
	return file.Stmts()
}
