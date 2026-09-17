package migration

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	commonsettings "github.com/huynhanx03/go-common/pkg/settings"

	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestNewRunnerUsesBoundedDefaultsAndRejectsInvalidBudgets(t *testing.T) {
	t.Parallel()

	runner, err := New(zap.NewNop(), settings.Migration{})
	if err != nil {
		t.Fatalf("new default runner: %v", err)
	}
	if runner.applyTimeout != defaultApplyTimeout ||
		runner.lockTimeout != defaultLockTimeout ||
		runner.connectTimeout != defaultConnectTimeout {
		t.Fatalf("runner timeouts = %s/%s/%s", runner.applyTimeout, runner.lockTimeout, runner.connectTimeout)
	}

	_, err = New(zap.NewNop(), settings.Migration{
		ApplyTimeoutSeconds: 1,
		LockTimeoutSeconds:  2,
	})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("invalid runner timeout error = %v", err)
	}
	_, err = New(zap.NewNop(), settings.Migration{ConnectTimeoutSeconds: -1})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("negative connect timeout error = %v", err)
	}
	_, err = New(zap.NewNop(), settings.Migration{ApplyTimeoutSeconds: int(maxApplyTimeout/time.Second) + 1})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("unbounded apply timeout error = %v", err)
	}
}

func TestPostgresURLUsesEscapingAndFiniteConnectTimeout(t *testing.T) {
	t.Parallel()

	dsn := postgresURL(commonsettings.Database{
		Host:     "2001:db8::1",
		Port:     5432,
		Username: "operator@example.com",
		Password: "p@ss:/?#word",
		Database: "judgify",
	}, 1500*time.Millisecond, 2*time.Minute)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse generated PostgreSQL URL: %v", err)
	}
	password, ok := parsed.User.Password()
	if !ok || password != "p@ss:/?#word" || parsed.User.Username() != "operator@example.com" {
		t.Fatalf("generated URL credentials were not escaped round-trip")
	}
	if parsed.Query().Get("connect_timeout") != "2" ||
		parsed.Query().Get("sslmode") != "disable" ||
		parsed.Query().Get("search_path") != revisionTableSchema ||
		parsed.Query().Get("application_name") != "judgify-migrate" ||
		parsed.Query().Get("statement_timeout") != "120000" {
		t.Fatalf("generated URL query = %q", parsed.RawQuery)
	}
	if strings.Contains(dsn, "p@ss:/?#word") {
		t.Fatal("generated URL contains unescaped password")
	}
}

// The operator binary excludes the Ent client, so this package is the only
// thing that registers the driver Atlas resolves for the "postgres" scheme.
func TestPostgresDatabaseDriverIsRegisteredForTheOperatorBinary(t *testing.T) {
	t.Parallel()

	for _, driver := range sql.Drivers() {
		if driver == "postgres" {
			return
		}
	}
	t.Fatalf("database/sql driver %q is not registered; drivers = %v", "postgres", sql.Drivers())
}

func TestRunnerFailsBeforeOpeningDatabaseForCanceledContext(t *testing.T) {
	t.Parallel()

	runner, err := New(zap.NewNop(), settings.Migration{})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = runner.Apply(ctx, settings.Config{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled apply error = %v, want context canceled", err)
	}
}

func TestMigrationLoggingCarriesCorrelationIDWithoutRawSQL(t *testing.T) {
	t.Parallel()

	core, observed := observer.New(zap.DebugLevel)
	ctx := correlation.WithContext(context.Background(), "019f650d-992f-7a57-a996-aa9e4588ea97")
	ctx = commonlogger.WithContext(ctx, zap.New(core))
	log := &atlasLogger{ctx: ctx}
	file := atlasmigrate.NewLocalFile(
		"20260719040000_sensitive.sql",
		[]byte("INSERT INTO credentials(secret) VALUES ('do-not-log-me');"),
	)
	log.Log(atlasmigrate.LogFile{File: file})
	log.Log(atlasmigrate.LogStmt{
		SQL: "INSERT INTO credentials(secret) VALUES ('do-not-log-me');",
		Stmt: &atlasmigrate.Stmt{
			Pos:  42,
			Text: "INSERT INTO credentials(secret) VALUES ('do-not-log-me');",
		},
	})

	entries := observed.All()
	if len(entries) != 2 {
		t.Fatalf("migration log entries = %d, want 2", len(entries))
	}
	for _, entry := range entries {
		encoded := entry.ContextMap()
		if encoded["correlation_id"] != "019f650d-992f-7a57-a996-aa9e4588ea97" {
			t.Fatalf("structured migration log correlation_id = %v", encoded["correlation_id"])
		}
		if strings.Contains(entry.Message+fieldsString(encoded), "do-not-log-me") {
			t.Fatal("raw migration SQL leaked into structured logs")
		}
	}
}

func TestStatementErrorsAreRedactedButRemainInspectable(t *testing.T) {
	t.Parallel()

	driverError := errors.New("database rejected statement")
	original := &atlasmigrate.StmtExecError{
		Version: "20260719040000",
		Stmt: &atlasmigrate.Stmt{
			Pos:  99,
			Text: "INSERT INTO credentials(secret) VALUES ('do-not-log-me');",
		},
		Err: driverError,
	}
	redacted := redactMigrationError(original)
	if strings.Contains(redacted.Error(), "do-not-log-me") {
		t.Fatal("redacted migration error contains raw SQL")
	}
	if !errors.Is(redacted, driverError) {
		t.Fatal("redacted migration error no longer unwraps to driver error")
	}
	var inspected *atlasmigrate.StmtExecError
	if !errors.As(redacted, &inspected) || inspected.Version != original.Version {
		t.Fatal("redacted migration error no longer supports errors.As")
	}
}

func TestRedactedCausesDoNotExposeConnectionOrSQLSecrets(t *testing.T) {
	t.Parallel()

	cause := errors.New("postgres://operator:do-not-log@database/judgify: SELECT secret")
	redacted := redactedCause("open migration database: connection details redacted", cause)
	if strings.Contains(redacted.Error(), "do-not-log") || strings.Contains(redacted.Error(), "SELECT secret") {
		t.Fatalf("redacted cause leaked sensitive details: %v", redacted)
	}
	if !errors.Is(redacted, cause) {
		t.Fatal("redacted cause no longer supports errors.Is")
	}
}

func fieldsString(fields map[string]any) string {
	var builder strings.Builder
	for key, value := range fields {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(strings.TrimSpace(toString(value)))
	}
	return builder.String()
}

func toString(value any) string {
	if stringValue, ok := value.(string); ok {
		return stringValue
	}
	return ""
}
