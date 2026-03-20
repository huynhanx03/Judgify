package settings

import "crypto/rsa"

type Config struct {
	Server        Server        `mapstructure:"server"`
	Logger        Logger        `mapstructure:"logger"`
	Database      Database      `mapstructure:"database"`
	SnowflakeNode SnowflakeNode `mapstructure:"snowflake_node"`
	JWT           JWT           `mapstructure:"jwt"`
	Google        Google        `mapstructure:"google"`
	Resend        Resend        `mapstructure:"resend"`
	FCM           FCM           `mapstructure:"fcm"`
}



type JWT struct {
	Secret         string          `mapstructure:"secret"`
	PrivateKeyPath string          `mapstructure:"private_key_path"`
	PublicKeyPath  string          `mapstructure:"public_key_path"`
	PrivateKey     *rsa.PrivateKey `mapstructure:"-"`
	PublicKey      *rsa.PublicKey  `mapstructure:"-"`
}



// Database is the configuration for the database
type Database struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// Server is the configuration for the server
type Server struct {
	Mode           string               `mapstructure:"mode"`
	Host           string               `mapstructure:"host"`
	Port           int                  `mapstructure:"port"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

type RateLimitConfig struct {
	Limit  int `mapstructure:"limit"`
	Burst  int `mapstructure:"burst"`
	Window int `mapstructure:"window"` // in seconds
}

type CircuitBreakerConfig struct {
	FailureThreshold int `mapstructure:"failure_threshold"`
	SuccessThreshold int `mapstructure:"success_threshold"`
	OpenTimeout      int `mapstructure:"open_timeout"` // in seconds
}



// Logger is the configuration for the logger
type Logger struct {
	LogLevel    string `mapstructure:"log_level"`
	FileLogName string `mapstructure:"file_log_name"`
	MaxBackups  int    `mapstructure:"max_backups"`
	MaxAge      int    `mapstructure:"max_age"`
	MaxSize     int    `mapstructure:"max_size"`
	Compress    bool   `mapstructure:"compress"`
}







type Snowflake struct {
	Epoch     int64 `mapstructure:"epoch"`
	Node      uint8 `mapstructure:"node"`
	Step      uint8 `mapstructure:"step"`
	TotalBits uint8 `mapstructure:"total_bits"`
}

type SnowflakeNode struct {
	Config   Snowflake
	WorkerID int64 `mapstructure:"worker_id"`
}

// Google is the configuration for Google OAuth
type Google struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

// Resend is the configuration for the Resend email API
type Resend struct {
	APIKey    string `mapstructure:"api_key"`
	FromEmail string `mapstructure:"from_email"`
	FromName  string `mapstructure:"from_name"`
}

// FCM is the configuration for Firebase Cloud Messaging
type FCM struct {
	ProjectID          string `mapstructure:"project_id"`
	ServiceAccountPath string `mapstructure:"service_account_path"`
}
