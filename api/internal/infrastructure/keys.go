package infrastructure

import (
	"log"
	"os"

	"github.com/huynhanx03/judgify/pkg/utils"

	"github.com/huynhanx03/judgify/global"
)

func SetupKeys() {
	// Load Private Key
	privBytes, err := os.ReadFile(global.Config.JWT.PrivateKeyPath)
	if err != nil {
		log.Fatalf("failed to read private key: %v", err)
	}

	global.Config.JWT.PrivateKey, err = utils.ParseRSAPrivateKey(privBytes)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	// Load Public Key
	pubBytes, err := os.ReadFile(global.Config.JWT.PublicKeyPath)
	if err != nil {
		log.Fatalf("failed to read public key: %v", err)
	}

	global.Config.JWT.PublicKey, err = utils.ParseRSAPublicKey(pubBytes)
	if err != nil {
		log.Fatalf("failed to parse public key: %v", err)
	}
}
