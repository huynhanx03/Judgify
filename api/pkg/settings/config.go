package settings

import (
	"time"

	gcsettings "github.com/huynhanx03/go-common/pkg/settings"
)

// Generic config blocks are shared via go-common; aliases keep existing
// references (settings.Server, settings.JWT, ...) working unchanged.
type (
	Server               = gcsettings.Server
	RateLimitConfig      = gcsettings.RateLimitConfig
	CircuitBreakerConfig = gcsettings.CircuitBreakerConfig
	Logger               = gcsettings.Logger
	Database             = gcsettings.Database
	JWT                  = gcsettings.JWT
	Snowflake            = gcsettings.Snowflake
	SnowflakeNode        = gcsettings.SnowflakeNode
)

// Config is the application configuration for the Judgify API,
// composing shared blocks from go-common with Judgify-specific sections.
type Config struct {
	Server               Server               `mapstructure:"server"`
	Logger               Logger               `mapstructure:"logger"`
	Database             Database             `mapstructure:"database"`
	Migration            Migration            `mapstructure:"migration"`
	Observability        Observability        `mapstructure:"observability"`
	SnowflakeNode        SnowflakeNode        `mapstructure:"snowflake_node"`
	JWT                  JWT                  `mapstructure:"jwt"`
	Authentication       Authentication       `mapstructure:"authentication"`
	Google               Google               `mapstructure:"google"`
	GitHub               GitHub               `mapstructure:"github"`
	Resend               Resend               `mapstructure:"resend"`
	SecurityEmail        SecurityEmail        `mapstructure:"security_email"`
	FCM                  FCM                  `mapstructure:"fcm"`
	Judge                Judge                `mapstructure:"judge"`
	Forge                Forge                `mapstructure:"forge"`
	Outbox               Outbox               `mapstructure:"outbox"`
	Operation            Operation            `mapstructure:"operation"`
	AdminCommand         AdminCommand         `mapstructure:"admin_command"`
	NotificationCampaign NotificationCampaign `mapstructure:"notification_campaign"`
}

// Migration bounds the finite operator-only schema migration command. Zero
// values use production-safe defaults in the migration runner.
type Migration struct {
	ApplyTimeoutSeconds   int `mapstructure:"apply_timeout_seconds"`
	LockTimeoutSeconds    int `mapstructure:"lock_timeout_seconds"`
	ConnectTimeoutSeconds int `mapstructure:"connect_timeout_seconds"`
}

// Observability defines immutable build identity plus one private probe
// listener per long-running role. These listeners are for the container
// network and host operator only; the public reverse proxy must never route
// their /metrics endpoint.
type Observability struct {
	Version                      string        `mapstructure:"version"`
	APIProbeAddress              string        `mapstructure:"api_probe_address"`
	WorkerProbeAddress           string        `mapstructure:"worker_probe_address"`
	EnvoyAuthzAddress            string        `mapstructure:"envoy_authz_address"`
	EnvoyAuthzCertificatePath    string        `mapstructure:"envoy_authz_certificate_path"`
	EnvoyAuthzPrivateKeyPath     string        `mapstructure:"envoy_authz_private_key_path"`
	EnvoyAuthzClientCAPath       string        `mapstructure:"envoy_authz_client_ca_path"`
	DebugMaximumDuration         time.Duration `mapstructure:"debug_maximum_duration"`
	DebugRecentAuthenticationTTL time.Duration `mapstructure:"debug_recent_authentication_ttl"`
}

// Authentication contains Judgify's application-level session and token
// policy. Cryptographic material stays in JWT.Secret so deployments have one
// secret source; distinct HMAC and AEAD keys are derived with domain-separated
// labels by the identity package.
type Authentication struct {
	Issuer                          string        `mapstructure:"issuer"`
	Audience                        string        `mapstructure:"audience"`
	SigningKeyID                    string        `mapstructure:"signing_key_id"`
	SecretKeyID                     string        `mapstructure:"secret_key_id"`
	AccessTTL                       time.Duration `mapstructure:"access_ttl"`
	RefreshTTL                      time.Duration `mapstructure:"refresh_ttl"`
	SessionIdleTTL                  time.Duration `mapstructure:"session_idle_ttl"`
	SessionAbsoluteTTL              time.Duration `mapstructure:"session_absolute_ttl"`
	RecoveryReceiptTTL              time.Duration `mapstructure:"recovery_receipt_ttl"`
	PasswordResetTTL                time.Duration `mapstructure:"password_reset_ttl"`
	PasswordResetShortWindow        time.Duration `mapstructure:"password_reset_short_window"`
	PasswordResetShortLimit         int           `mapstructure:"password_reset_short_limit"`
	PasswordResetDailyLimit         int           `mapstructure:"password_reset_daily_limit"`
	RecoveryVerificationTTL         time.Duration `mapstructure:"recovery_verification_ttl"`
	RecoveryReauthenticationTTL     time.Duration `mapstructure:"recovery_reauthentication_ttl"`
	RecoveryResendCooldown          time.Duration `mapstructure:"recovery_resend_cooldown"`
	RecoveryCommandReceiptTTL       time.Duration `mapstructure:"recovery_command_receipt_ttl"`
	RecoveryReplacementDailyLimit   int           `mapstructure:"recovery_replacement_daily_limit"`
	RecoveryVerificationHourlyLimit int           `mapstructure:"recovery_verification_hourly_limit"`
	RecoveryVerificationDailyLimit  int           `mapstructure:"recovery_verification_daily_limit"`
	RecoveryTerminalRetention       time.Duration `mapstructure:"recovery_terminal_retention"`
	ReceiptCleanupInterval          time.Duration `mapstructure:"receipt_cleanup_interval"`
	ReceiptCleanupTimeout           time.Duration `mapstructure:"receipt_cleanup_timeout"`
	ReceiptCleanupBatch             int           `mapstructure:"receipt_cleanup_batch"`
	ReceiptCleanupBatches           int           `mapstructure:"receipt_cleanup_batches"`
	RefreshCookieName               string        `mapstructure:"refresh_cookie_name"`
	RefreshCookiePath               string        `mapstructure:"refresh_cookie_path"`
	RefreshCookieSecure             bool          `mapstructure:"refresh_cookie_secure"`
	RefreshCookieSameSite           string        `mapstructure:"refresh_cookie_same_site"`
	AccessCookieName                string        `mapstructure:"access_cookie_name"`
	CSRFCookieName                  string        `mapstructure:"csrf_cookie_name"`
	PreSessionCookieName            string        `mapstructure:"pre_session_cookie_name"`
	PreSessionTTL                   time.Duration `mapstructure:"pre_session_ttl"`
	OAuthLoginTicketCookieName      string        `mapstructure:"oauth_login_ticket_cookie_name"`
	OAuthLoginTicketCookiePath      string        `mapstructure:"oauth_login_ticket_cookie_path"`
	OAuthLinkTicketCookieName       string        `mapstructure:"oauth_link_ticket_cookie_name"`
	OAuthLinkTicketCookiePath       string        `mapstructure:"oauth_link_ticket_cookie_path"`
	OAuthCompletionURL              string        `mapstructure:"oauth_completion_url"`
	BrowserOrigins                  []string      `mapstructure:"browser_origins"`
	MaximumCredentialBytes          int           `mapstructure:"maximum_credential_bytes"`
	MaximumUserAgentBytes           int           `mapstructure:"maximum_user_agent_bytes"`
	TraitOfferTTL                   time.Duration `mapstructure:"trait_offer_ttl"`
}

type JudgeAPIAuthoringCatalogMode string

const (
	// JudgeAPICatalogApprovedRelease makes the API project and validate problem
	// authoring choices from the exact approved release artifact shared with
	// workers. Production API processes must always use this mode.
	JudgeAPICatalogApprovedRelease JudgeAPIAuthoringCatalogMode = "approved_release"
	// JudgeAPICatalogDevelopment exposes only code-owned authoring metadata to a
	// non-production API. It cannot supply worker capabilities or executable
	// image identities and is therefore safe for lightweight local development.
	JudgeAPICatalogDevelopment JudgeAPIAuthoringCatalogMode = "code_owned_development"
)

// Judge is the configuration for the code judge system.
type Judge struct {
	ReleaseID                       string                       `mapstructure:"release_id"`
	APIAuthoringCatalogMode         JudgeAPIAuthoringCatalogMode `mapstructure:"api_authoring_catalog_mode"`
	ReleaseManifestPath             string                       `mapstructure:"release_manifest_path"`
	DockerSocketPath                string                       `mapstructure:"docker_socket_path"`
	StagingRoot                     string                       `mapstructure:"staging_root"`
	MaximumActiveSubmissionsPerUser int                          `mapstructure:"maximum_active_submissions_per_user"`
	MaximumReadyBacklog             int                          `mapstructure:"maximum_ready_backlog"`
	MaximumOldestReadyAge           time.Duration                `mapstructure:"maximum_oldest_ready_age"`
	MinimumFreeDiskBytes            uint64                       `mapstructure:"minimum_free_disk_bytes"`
	AdmissionDiskPath               string                       `mapstructure:"admission_disk_path"`
	WorkerHeartbeatFreshness        time.Duration                `mapstructure:"worker_heartbeat_freshness"`
	DockerImage                     string                       `mapstructure:"docker_image"`
	PoolSize                        int                          `mapstructure:"pool_size"`
	WorkerCount                     int                          `mapstructure:"worker_count"`
	CompileTimeoutMs                int                          `mapstructure:"compile_timeout_ms"`
	// These two values are deployment assertions and must match pkg/judging;
	// the API, database, immutable bundles, and workers share one byte policy.
	MaximumTestInputBytes  int           `mapstructure:"maximum_test_input_bytes"`
	MaximumOutputBytes     int           `mapstructure:"maximum_output_bytes"`
	MaximumDiagnosticBytes int           `mapstructure:"maximum_diagnostic_bytes"`
	NetworkDisabled        bool          `mapstructure:"network_disabled"`
	ContainerMemoryMb      int           `mapstructure:"container_memory_mb"`   // hard per-container memory ceiling (default: 1024)
	SandboxTmpfsSizeMb     int           `mapstructure:"sandbox_tmpfs_size_mb"` // /sandbox tmpfs size (default: 64)
	WorkerIdentityPrefix   string        `mapstructure:"worker_identity_prefix"`
	LeaseDuration          time.Duration `mapstructure:"lease_duration"`
	LeaseHeartbeat         time.Duration `mapstructure:"lease_heartbeat"`
	LeaseRetryFloor        time.Duration `mapstructure:"lease_retry_floor"`
	MaximumAttempts        uint32        `mapstructure:"maximum_attempts"`
}

// Forge configures the worker-owned local solve-reward delivery buffer.
// PostgreSQL remains canonical; topic and group names are Judgify-owned
// deployment identities rather than shared-library policy.
type Forge struct {
	DataDir           string        `mapstructure:"data_dir"`
	Topic             string        `mapstructure:"topic"`
	ConsumerGroup     string        `mapstructure:"consumer_group"`
	MaxMessageBytes   int           `mapstructure:"max_message_bytes"`
	MaxSegmentBytes   int64         `mapstructure:"max_segment_bytes"`
	MaxStorageBytes   int64         `mapstructure:"max_storage_bytes"`
	RetentionBytes    int64         `mapstructure:"retention_bytes"`
	RetentionTime     time.Duration `mapstructure:"retention_time"`
	RetentionInterval time.Duration `mapstructure:"retention_interval"`
}

// Outbox configures the role-owned durable PostgreSQL event dispatchers and
// bounded projection-repair loops. Durations are decoded from human-readable
// config values.
type Outbox struct {
	Name                string        `mapstructure:"name"`
	OwnerPrefix         string        `mapstructure:"owner_prefix"`
	Concurrency         int           `mapstructure:"concurrency"`
	ClaimLimit          int           `mapstructure:"claim_limit"`
	MaxMessageBytes     int           `mapstructure:"max_message_bytes"`
	MaxAttempts         uint32        `mapstructure:"max_attempts"`
	LeaseDuration       time.Duration `mapstructure:"lease_duration"`
	PublishTimeout      time.Duration `mapstructure:"publish_timeout"`
	StoreTimeout        time.Duration `mapstructure:"store_timeout"`
	FinalizeTimeout     time.Duration `mapstructure:"finalize_timeout"`
	PollInterval        time.Duration `mapstructure:"poll_interval"`
	RetryInitial        time.Duration `mapstructure:"retry_initial"`
	RetryMaximum        time.Duration `mapstructure:"retry_maximum"`
	RetryMultiplier     float64       `mapstructure:"retry_multiplier"`
	ReconcileInterval   time.Duration `mapstructure:"reconcile_interval"`
	ReconcileBatch      int           `mapstructure:"reconcile_batch"`
	MaxReconcileBatches int           `mapstructure:"max_reconcile_batches"`
}

// NotificationCampaign bounds the single-machine operation worker and the
// reviewed audience confirmation window. Confirmation keys are derived from
// JWT.Secret and therefore are not duplicated in configuration.
type NotificationCampaign struct {
	MaximumRecipients       int           `mapstructure:"maximum_recipients"`
	BatchSize               int           `mapstructure:"batch_size"`
	ConfirmationTTL         time.Duration `mapstructure:"confirmation_ttl"`
	RecentAuthenticationTTL time.Duration `mapstructure:"recent_authentication_ttl"`
}

// AdminCommand configures the common reviewed-command security window. The
// HMAC key is domain-separated from JWT.Secret by admincommand.
type AdminCommand struct {
	ConfirmationTTL         time.Duration `mapstructure:"confirmation_ttl"`
	RecentAuthenticationTTL time.Duration `mapstructure:"recent_authentication_ttl"`
}

// Operation configures the domain-neutral, single-host durable work runner.
// Domain batch semantics remain registered in code and never enter config.
type Operation struct {
	OwnerPrefix     string        `mapstructure:"owner_prefix"`
	LeaseDuration   time.Duration `mapstructure:"lease_duration"`
	BatchTimeout    time.Duration `mapstructure:"batch_timeout"`
	IdleWait        time.Duration `mapstructure:"idle_wait"`
	RetryInitial    time.Duration `mapstructure:"retry_initial"`
	RetryMaximum    time.Duration `mapstructure:"retry_maximum"`
	MaximumAttempts uint32        `mapstructure:"maximum_attempts"`
}

// Google is the configuration for Google OAuth
type Google struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

// GitHub is the configuration for GitHub OAuth
type GitHub struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

// Resend is the configuration for the Resend email API
type Resend struct {
	APIKey          string `mapstructure:"api_key"`
	FromEmail       string `mapstructure:"from_email"`
	FromName        string `mapstructure:"from_name"`
	ResetURL        string `mapstructure:"reset_url"`
	VerificationURL string `mapstructure:"verification_url"`
	InvitationURL   string `mapstructure:"invitation_url"`
}

// SecurityEmail configures the durable, API-owned security-email dispatcher.
// Provider secrets remain in Resend; this section contains only bounded
// operational policy and encrypted-payload limits.
type SecurityEmail struct {
	OwnerPrefix            string        `mapstructure:"owner_prefix"`
	PollInterval           time.Duration `mapstructure:"poll_interval"`
	ClaimTimeout           time.Duration `mapstructure:"claim_timeout"`
	DeliveryTimeout        time.Duration `mapstructure:"delivery_timeout"`
	FinalizeTimeout        time.Duration `mapstructure:"finalize_timeout"`
	LeaseDuration          time.Duration `mapstructure:"lease_duration"`
	BatchSize              int           `mapstructure:"batch_size"`
	Concurrency            int           `mapstructure:"concurrency"`
	MaximumAttempts        uint32        `mapstructure:"maximum_attempts"`
	RetryInitial           time.Duration `mapstructure:"retry_initial"`
	RetryMaximum           time.Duration `mapstructure:"retry_maximum"`
	RetryMultiplier        float64       `mapstructure:"retry_multiplier"`
	MaximumCiphertextBytes int           `mapstructure:"maximum_ciphertext_bytes"`
}

// FCM is the configuration for Firebase Cloud Messaging
type FCM struct {
	ProjectID          string `mapstructure:"project_id"`
	ServiceAccountPath string `mapstructure:"service_account_path"`
}
