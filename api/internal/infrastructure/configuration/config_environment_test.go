package configuration

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestBindJWTSecretUsesExplicitEnvironmentWithoutExposingIt(t *testing.T) {
	secret := strings.Repeat("s", 32)
	t.Setenv("JWT_SECRET", secret)
	loader := viper.New()
	loader.SetDefault("jwt.secret", "${JWT_SECRET}")
	requireNoError(t, bindJWTSecret(loader))
	if got := loader.GetString("jwt.secret"); got != secret {
		t.Fatalf("jwt.secret was not resolved from the explicit environment")
	}
}

func TestBindJWTSecretLeavesMissingPlaceholderForFailClosedValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	loader := viper.New()
	loader.SetDefault("jwt.secret", "${JWT_SECRET}")
	requireNoError(t, bindJWTSecret(loader))
	if got := loader.GetString("jwt.secret"); got != "${JWT_SECRET}" {
		t.Fatalf("missing secret unexpectedly resolved")
	}
}

func TestBindPasswordResetDeliveryUsesExplicitEnvironment(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test_value")
	t.Setenv("RESEND_FROM_EMAIL", "security@example.com")
	t.Setenv("RESEND_FROM_NAME", "Security")
	t.Setenv("RESEND_RESET_URL", "https://app.example.com/reset-password")
	t.Setenv("RESEND_VERIFICATION_URL", "https://app.example.com/verify-recovery-contact")
	loader := viper.New()
	loader.SetDefault("resend.api_key", "${RESEND_API_KEY}")
	loader.SetDefault("resend.from_email", "${RESEND_FROM_EMAIL}")
	requireNoError(t, bindPasswordResetDelivery(loader))
	if loader.GetString("resend.api_key") != "re_test_value" ||
		loader.GetString("resend.from_email") != "security@example.com" ||
		loader.GetString("resend.from_name") != "Security" ||
		loader.GetString("resend.reset_url") != "https://app.example.com/reset-password" ||
		loader.GetString("resend.verification_url") != "https://app.example.com/verify-recovery-contact" {
		t.Fatal("password reset delivery environment was not bound")
	}
}

func TestBindPasswordResetDeliveryLeavesMissingSecretsFailClosed(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("RESEND_FROM_EMAIL", "")
	loader := viper.New()
	loader.SetDefault("resend.api_key", "${RESEND_API_KEY}")
	loader.SetDefault("resend.from_email", "${RESEND_FROM_EMAIL}")
	requireNoError(t, bindPasswordResetDelivery(loader))
	if loader.GetString("resend.api_key") != "${RESEND_API_KEY}" ||
		loader.GetString("resend.from_email") != "${RESEND_FROM_EMAIL}" {
		t.Fatal("missing Resend secret unexpectedly resolved")
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
