package infrastructure

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/huynhanx03/judgify/global"
)

// LoadConfig loads configuration from file
func LoadConfig() {
	viper := viper.New()

	// Get environment
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "local"
	}

	// Set config file
	configFile := fmt.Sprintf("config/%s.yaml", env)
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("Failed to read config file: %v", err))
	}

	// Enable reading from environment variables
	viper.AutomaticEnv()
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	_ = viper.MergeInConfig()

	// Unmarshal config
	if err := viper.Unmarshal(&global.Config); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal config: %v", err))
	}

	fmt.Printf("Loaded configuration from: %s\n", configFile)
}
