package configuration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const localEnvironmentFixture = `DATABASE_HOST=127.0.0.1
DATABASE_PORT=55199
DATABASE_USERNAME=audit_user
DATABASE_DATABASE=audit_database
DATABASE_PASSWORD=local-diagnostic-password-123456789
JWT_SECRET=local-diagnostic-jwt-secret-123456789
GOOGLE_CLIENT_ID=audit-google-client-id
GOOGLE_CLIENT_SECRET=audit-google-client-secret
GITHUB_CLIENT_ID=audit-github-client-id
GITHUB_CLIENT_SECRET=audit-github-client-secret
RESEND_API_KEY=local-diagnostic-resend-key
RESEND_FROM_EMAIL=security@localhost.invalid
RESEND_FROM_NAME=Judgify
RESEND_RESET_URL=http://localhost:3000/reset-password
RESEND_VERIFICATION_URL=http://localhost:3000/verify-recovery-contact
RESEND_INVITATION_URL=http://localhost:3000/accept-invitation
`

func prepareLocalConfigFiles(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile("../../../config/local.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "config/local.yaml"), raw, 0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, ".env"), []byte(localEnvironmentFixture), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_USERNAME", "DATABASE_DATABASE",
		"DATABASE_PASSWORD", "DATABASE_PASSWORD_FILE", "JWT_SECRET", "JWT_SECRET_FILE",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_CLIENT_SECRET_FILE",
		"GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_CLIENT_SECRET_FILE",
		"RESEND_API_KEY", "RESEND_API_KEY_FILE", "RESEND_FROM_EMAIL", "RESEND_FROM_NAME",
		"RESEND_RESET_URL", "RESEND_VERIFICATION_URL", "RESEND_INVITATION_URL",
		"JUDGIFY_VERSION",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("GO_ENV", "local")
	t.Chdir(root)
}

func TestLoadLocalDatabaseOverridesFromDotEnv(t *testing.T) {
	prepareLocalConfigFiles(t)
	got, err := Load(context.Background(), ProfileMigrate, OSFileSecretSource{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Database.Host != "127.0.0.1" || got.Database.Port != 55199 ||
		got.Database.Username != "audit_user" || got.Database.Database != "audit_database" {
		t.Fatal("database aliases from dotenv were not applied")
	}
}

func TestLoadLocalAPIWithDotEnvOAuthIDs(t *testing.T) {
	prepareLocalConfigFiles(t)
	got, err := Load(context.Background(), ProfileAPI, OSFileSecretSource{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Google.ClientID != "audit-google-client-id" ||
		got.GitHub.ClientID != "audit-github-client-id" {
		t.Fatal("OAuth client IDs from dotenv were not applied")
	}
}

func TestLoadProcessEnvironmentOverridesDotEnv(t *testing.T) {
	prepareLocalConfigFiles(t)
	t.Setenv("DATABASE_PORT", "55200")
	t.Setenv("GOOGLE_CLIENT_ID", "process-google-client-id")
	got, err := Load(context.Background(), ProfileAPI, OSFileSecretSource{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Database.Port != 55200 || got.Google.ClientID != "process-google-client-id" {
		t.Fatal("process environment must take precedence over dotenv")
	}
}
