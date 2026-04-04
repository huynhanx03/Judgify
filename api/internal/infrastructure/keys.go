package infrastructure

import (
	"os"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/pkg/utils"

	"github.com/huynhanx03/judgify/global"
)

func SetupKeys() {
	// Load Private Key
	privBytes, err := os.ReadFile(global.Config.JWT.PrivateKeyPath)
	if err != nil {
		global.LoggerZap.Fatal("failed to read private key", zap.Error(err))
	}

	global.Config.JWT.PrivateKey, err = utils.ParseRSAPrivateKey(privBytes)
	if err != nil {
		global.LoggerZap.Fatal("failed to parse private key", zap.Error(err))
	}

	// Load Public Key
	pubBytes, err := os.ReadFile(global.Config.JWT.PublicKeyPath)
	if err != nil {
		global.LoggerZap.Fatal("failed to read public key", zap.Error(err))
	}

	global.Config.JWT.PublicKey, err = utils.ParseRSAPublicKey(pubBytes)
	if err != nil {
		global.LoggerZap.Fatal("failed to parse public key", zap.Error(err))
	}
	global.LoggerZap.Named("security").Info("RSA keys loaded successfully")
}
