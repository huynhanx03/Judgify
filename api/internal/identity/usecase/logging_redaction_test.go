package usecase

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthenticationLogsDoNotAttachRawIdentityOrRecoveryFields(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(source), "authentication.go"))
	require.NoError(t, err)

	for _, forbidden := range []string{
		`zap.String("username"`,
		`zap.String("email"`,
		`zap.String("recovery_email"`,
		`zap.String("token"`,
		`zap.String("password"`,
		`zap.ByteString("ciphertext"`,
	} {
		require.NotContains(t, string(contents), forbidden)
	}
}
