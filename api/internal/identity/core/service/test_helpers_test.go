package service

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// setupGlobalConfig initializes global.Config with test RSA keys.
// Must be called once before any test that generates JWT tokens.
func setupGlobalConfig(t *testing.T) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}

	global.Config = settings.Config{
		JWT: settings.JWT{
			PrivateKey: privateKey,
			PublicKey:  &privateKey.PublicKey,
		},
	}
}
