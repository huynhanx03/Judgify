package db

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/pkg/judging"
)

func TestPublicRuntimeCatalogReturnsOnlyActivatedBrowserProjection(t *testing.T) {
	client, mock := newPublicRuntimeCatalogMockClient(t)
	catalog, err := NewPublicRuntimeCatalog(
		client,
		"judge-release.2026.07.28.1",
	)
	if err != nil {
		t.Fatalf("NewPublicRuntimeCatalog() error = %v", err)
	}
	mock.ExpectQuery(`SELECT DISTINCT profile\.runtime_key`).
		WithArgs(
			"judge-release.2026.07.28.1",
			judging.MaximumCatalogRuntimes+1,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"runtime_key", "language", "display_name",
		}).
			AddRow("python3", "python", "Python 3").
			AddRow("cpp17", "cpp", "C++17"))

	result, err := catalog.Read(context.Background())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if result.Version < 1 || len(result.Runtimes) != 2 {
		t.Fatalf("runtime catalog = %+v", result)
	}
	if result.Runtimes[0].Key != "cpp17" ||
		result.Runtimes[0].Language != "cpp" ||
		result.Runtimes[0].DisplayName != "C++17" ||
		result.Runtimes[0].SourceFilename != "main.cpp" ||
		len(result.Runtimes[0].FileExtensions) == 0 {
		t.Fatalf("first browser runtime = %+v", result.Runtimes[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func TestPublicRuntimeCatalogFailsClosedOnUntrustedProfileLanguage(t *testing.T) {
	client, mock := newPublicRuntimeCatalogMockClient(t)
	catalog, err := NewPublicRuntimeCatalog(
		client,
		"judge-release.2026.07.28.1",
	)
	if err != nil {
		t.Fatalf("NewPublicRuntimeCatalog() error = %v", err)
	}
	mock.ExpectQuery(`SELECT DISTINCT profile\.runtime_key`).
		WithArgs(
			"judge-release.2026.07.28.1",
			judging.MaximumCatalogRuntimes+1,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"runtime_key", "language", "display_name",
		}).AddRow("ruby3", "ruby", "Ruby 3"))

	if _, err := catalog.Read(context.Background()); err == nil {
		t.Fatal("untrusted runtime language unexpectedly reached browser")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func newPublicRuntimeCatalogMockClient(
	t *testing.T,
) (*internalent.EntClient, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
	)
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	driver := entsql.OpenDB(dialect.Postgres, database)
	generated := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = generated.Close() })
	return internalent.WrapClient(generated, zap.NewNop()), mock
}
