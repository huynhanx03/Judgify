// Package dbtest contains test-only database harness configuration.
package dbtest

import (
	"os"
	"testing"
)

// DatabaseURLEnvironment is the DSN injected by the integration runner.
const DatabaseURLEnvironment = "JUDGIFY_TEST_DATABASE_URL"

// DatabaseURL returns the runner-provided database DSN.
func DatabaseURL() string {
	return os.Getenv(DatabaseURLEnvironment)
}

// RequireDatabaseURL returns the integration-runner DSN or skips the test.
// It keeps local unit runs Docker-free while making every integration test use
// the same explicit opt-in environment contract.
func RequireDatabaseURL(t testing.TB) string {
	t.Helper()
	dsn := DatabaseURL()
	if dsn == "" {
		t.Skipf("%s is not configured", DatabaseURLEnvironment)
	}
	return dsn
}
