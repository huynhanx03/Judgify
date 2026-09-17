package configuration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

	"github.com/huynhanx03/judgify/pkg/settings"
)

// Profile selects only the configuration and secret surface required by one
// process. Operator commands are finite DML-only tools and deliberately do not
// require browser authentication or email-provider credentials.
type Profile string

const (
	ProfileAPI         Profile = "api"
	ProfileJudgeWorker Profile = "judge-worker"
	ProfileMigrate     Profile = "migrate"
	ProfileOperator    Profile = "operator"
	ProfileAll         Profile = "all"
)

const (
	maximumSecretBytes                = 64 << 10
	maximumConfigBytes                = 1 << 20
	minimumForgeEnvelopeHeadroomBytes = 32 << 10
)

var (
	ErrInvalidConfiguration = errors.New("bootstrap: invalid configuration")
	ErrInvalidSecretSource  = errors.New("bootstrap: invalid secret source")
)

// ConfigError identifies only a code-owned field. It never renders the field
// value because startup configuration commonly contains credentials.
type ConfigError struct {
	Field string
}

func (failure *ConfigError) Error() string {
	if failure == nil || failure.Field == "" {
		return ErrInvalidConfiguration.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidConfiguration, failure.Field)
}

func (failure *ConfigError) Unwrap() error {
	return ErrInvalidConfiguration
}

type secretBinding struct {
	field     string
	directEnv string
	fileEnv   string
	profiles  []Profile
}

var secretBindings = []secretBinding{
	{
		field: "database.password", directEnv: "DATABASE_PASSWORD",
		fileEnv: "DATABASE_PASSWORD_FILE",
		profiles: []Profile{
			ProfileAPI, ProfileJudgeWorker, ProfileMigrate,
			ProfileOperator, ProfileAll,
		},
	},
	{
		field: "jwt.secret", directEnv: "JWT_SECRET",
		fileEnv:  "JWT_SECRET_FILE",
		profiles: []Profile{ProfileAPI, ProfileAll},
	},
	{
		field: "google.client_secret", directEnv: "GOOGLE_CLIENT_SECRET",
		fileEnv:  "GOOGLE_CLIENT_SECRET_FILE",
		profiles: []Profile{ProfileAPI, ProfileAll},
	},
	{
		field: "github.client_secret", directEnv: "GITHUB_CLIENT_SECRET",
		fileEnv:  "GITHUB_CLIENT_SECRET_FILE",
		profiles: []Profile{ProfileAPI, ProfileAll},
	},
	{
		field: "resend.api_key", directEnv: "RESEND_API_KEY",
		fileEnv:  "RESEND_API_KEY_FILE",
		profiles: []Profile{ProfileAPI, ProfileAll},
	},
}

// Load validates and resolves exactly the configuration surface required by
// profile. It performs no resource acquisition and is safe for finite
// operator-only commands to use without importing the application bootstrap
// graph.
func Load(
	ctx context.Context,
	profile Profile,
	secrets SecretSource,
) (settings.Config, error) {
	if ctx == nil || !profile.valid() || secrets == nil {
		return settings.Config{}, ErrInvalidConfiguration
	}
	if err := ctx.Err(); err != nil {
		return settings.Config{}, err
	}
	environment := strings.TrimSpace(os.Getenv("GO_ENV"))
	if environment == "" {
		environment = "local"
	}
	if !safeEnvironmentName(environment) {
		return settings.Config{}, configInvalid("GO_ENV")
	}
	configFile := filepath.Join("config", environment+".yaml")
	raw, err := readBoundedConfig(configFile)
	if err != nil {
		return settings.Config{}, err
	}
	if err := validateConfigurationKeys(raw, reflect.TypeOf(settings.Config{})); err != nil {
		return settings.Config{}, err
	}

	loader := viper.New()
	loader.SetConfigFile(configFile)
	loader.SetConfigType("yaml")
	if err := loader.ReadInConfig(); err != nil {
		return settings.Config{}, fmt.Errorf(
			"%w: configuration file",
			ErrInvalidConfiguration,
		)
	}
	loader.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	loader.AutomaticEnv()
	production := environment == "production"
	if !production {
		loader.SetConfigFile(".env")
		loader.SetConfigType("env")
		if err := loader.MergeInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) && !os.IsNotExist(err) {
				return settings.Config{}, fmt.Errorf(
					"%w: environment file",
					ErrInvalidConfiguration,
				)
			}
		}
	}
	for _, binding := range secretBindings {
		if !binding.requiredBy(profile) {
			continue
		}
		if err := bindSecret(
			ctx,
			loader,
			binding,
			secrets,
			production,
		); err != nil {
			return settings.Config{}, err
		}
	}
	for _, binding := range [][2]string{
		{"database.host", "DATABASE_HOST"},
		{"database.port", "DATABASE_PORT"},
		{"database.username", "DATABASE_USERNAME"},
		{"database.database", "DATABASE_DATABASE"},
		{"google.client_id", "GOOGLE_CLIENT_ID"},
		{"github.client_id", "GITHUB_CLIENT_ID"},
		{"observability.version", "JUDGIFY_VERSION"},
		{"resend.from_email", "RESEND_FROM_EMAIL"},
		{"resend.from_name", "RESEND_FROM_NAME"},
		{"resend.reset_url", "RESEND_RESET_URL"},
		{"resend.verification_url", "RESEND_VERIFICATION_URL"},
		{"resend.invitation_url", "RESEND_INVITATION_URL"},
	} {
		if err := bindEnvironmentValue(
			loader,
			binding[0],
			binding[1],
		); err != nil {
			return settings.Config{}, err
		}
	}
	if profile == ProfileJudgeWorker || profile == ProfileAll {
		if err := bindEnvironmentValue(
			loader,
			"forge.data_dir",
			"FORGE_DATA_DIR",
		); err != nil {
			return settings.Config{}, err
		}
	}

	var config settings.Config
	if err := loader.Unmarshal(&config); err != nil {
		return settings.Config{}, fmt.Errorf(
			"%w: decode",
			ErrInvalidConfiguration,
		)
	}
	if config.Server.Mode.IsProd() != production {
		return settings.Config{}, configInvalid("GO_ENV")
	}
	if err := validateConfig(config, profile); err != nil {
		return settings.Config{}, err
	}
	return config, nil
}

func bindSecret(
	ctx context.Context,
	loader *viper.Viper,
	binding secretBinding,
	source SecretSource,
	fileOnly bool,
) error {
	if loader == nil || source == nil ||
		binding.field == "" || binding.directEnv == "" ||
		binding.fileEnv == "" {
		return ErrInvalidConfiguration
	}
	if err := loader.BindEnv(binding.field, binding.directEnv); err != nil {
		return configInvalid(binding.field)
	}
	direct, hasDirect := externalValue(loader, binding.directEnv)
	fileName, hasFile := externalValue(loader, binding.fileEnv)
	if hasDirect && hasFile {
		return configInvalid(binding.field + ".source")
	}
	if fileOnly && hasDirect {
		return configInvalid(binding.field + ".source")
	}
	if hasFile {
		if fileOnly &&
			(!filepath.IsAbs(fileName) || filepath.Clean(fileName) != fileName) {
			return configInvalid(binding.field + ".source")
		}
		value, err := source.Read(ctx, fileName)
		if err != nil {
			return errors.Join(
				ErrInvalidSecretSource,
				configInvalid(binding.field),
			)
		}
		loader.Set(binding.field, string(value))
		return nil
	}
	if hasDirect {
		loader.Set(binding.field, direct)
	}
	return nil
}

func bindEnvironmentValue(
	loader *viper.Viper,
	nestedKey string,
	environmentKey string,
) error {
	if loader == nil || nestedKey == "" || environmentKey == "" {
		return ErrInvalidConfiguration
	}
	if err := loader.BindEnv(nestedKey, environmentKey); err != nil {
		return configInvalid(nestedKey)
	}
	if value, exists := externalValue(loader, environmentKey); exists {
		loader.Set(nestedKey, value)
	}
	return nil
}

func externalValue(loader *viper.Viper, key string) (string, bool) {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value, true
	}
	value := loader.GetString(key)
	return value, value != ""
}

func validateConfig(config settings.Config, profile Profile) error {
	if !profile.valid() {
		return configInvalid("profile")
	}
	if err := config.Server.Validate(); err != nil {
		return configInvalid("server")
	}
	if strings.ToLower(strings.TrimSpace(config.Database.Driver)) != "postgres" {
		return configInvalid("database.driver")
	}
	if err := config.Database.Validate(); err != nil {
		return configInvalid("database")
	}
	if err := validateLogger(config); err != nil {
		return err
	}
	if err := validateObservability(
		config.Observability,
		profile,
		config.Server.Mode.IsProd(),
	); err != nil {
		return err
	}
	if profile == ProfileMigrate {
		return validateMigration(config.Migration)
	}
	if profile == ProfileJudgeWorker {
		if err := validateJudgeWorker(config.Judge); err != nil {
			return err
		}
		if err := validateOutbox(config.Outbox); err != nil {
			return err
		}
		if err := validateForge(config.Forge, config.Server.Mode.IsProd()); err != nil {
			return err
		}
		return validateForgeOutboxBoundary(config.Forge, config.Outbox)
	}
	if profile == ProfileOperator {
		if config.Judge.WorkerHeartbeatFreshness <= 0 {
			return configInvalid("judge.worker_heartbeat_freshness")
		}
		return nil
	}
	if profile == ProfileAPI || profile == ProfileAll {
		if err := validateTrustedProxies(config); err != nil {
			return err
		}
		if err := validateAPIConfig(config); err != nil {
			return err
		}
	}
	if profile == ProfileAll {
		if err := validateJudgeWorker(config.Judge); err != nil {
			return err
		}
		if err := validateForge(config.Forge, config.Server.Mode.IsProd()); err != nil {
			return err
		}
		return validateForgeOutboxBoundary(config.Forge, config.Outbox)
	}
	return nil
}

func validateObservability(
	config settings.Observability,
	profile Profile,
	production bool,
) error {
	if !safeBuildLabel(config.Version) {
		return configInvalid("observability.version")
	}
	if (profile == ProfileAPI || profile == ProfileAll) &&
		(config.DebugMaximumDuration < time.Second ||
			config.DebugMaximumDuration > time.Hour ||
			config.DebugRecentAuthenticationTTL < time.Minute ||
			config.DebugRecentAuthenticationTTL > time.Hour) {
		return configInvalid("observability.debug_window")
	}
	switch profile {
	case ProfileAPI:
		if !validProbeAddress(config.APIProbeAddress, production) {
			return configInvalid("observability.api_probe_address")
		}
	case ProfileJudgeWorker:
		if !validProbeAddress(config.WorkerProbeAddress, production) {
			return configInvalid("observability.worker_probe_address")
		}
	case ProfileAll:
		if !validProbeAddress(config.APIProbeAddress, production) ||
			!validProbeAddress(config.WorkerProbeAddress, production) ||
			config.APIProbeAddress == config.WorkerProbeAddress {
			return configInvalid("observability.probe_addresses")
		}
	}
	return nil
}

func validProbeAddress(value string, loopbackOnly bool) bool {
	if !validListenAddress(value) {
		return false
	}
	if !loopbackOnly {
		return true
	}
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validListenAddress(value string) bool {
	if !safeText(value, 256) {
		return false
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		return false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return host == "localhost"
}

func validateAPIConfig(config settings.Config) error {
	if err := config.JWT.Validate(); err != nil ||
		!validSecret(config.JWT.Secret, 32) ||
		strings.TrimSpace(config.JWT.PrivateKeyPath) == "" ||
		strings.TrimSpace(config.JWT.PublicKeyPath) == "" {
		return configInvalid("jwt")
	}
	authentication := config.Authentication
	if !safeToken(authentication.Issuer, 128) ||
		!safeToken(authentication.Audience, 128) ||
		!safeToken(authentication.SigningKeyID, 128) ||
		!safeToken(authentication.SecretKeyID, 128) ||
		authentication.AccessTTL <= 0 ||
		authentication.RefreshTTL <= authentication.AccessTTL ||
		authentication.SessionIdleTTL <= 0 ||
		authentication.SessionIdleTTL >
			authentication.SessionAbsoluteTTL ||
		authentication.SessionAbsoluteTTL >
			authentication.RefreshTTL ||
		authentication.RecoveryReceiptTTL <= 0 ||
		authentication.PasswordResetTTL <= 0 ||
		authentication.MaximumCredentialBytes < 256 ||
		authentication.MaximumCredentialBytes > 64<<10 ||
		authentication.MaximumUserAgentBytes < 64 ||
		authentication.MaximumUserAgentBytes > 4<<10 ||
		authentication.TraitOfferTTL <= 0 {
		return configInvalid("authentication")
	}
	if err := validateBrowserSecurity(config); err != nil {
		return err
	}
	if err := validateOAuthProvider(
		"google",
		config.Google.ClientID,
		config.Google.ClientSecret,
		config.Google.RedirectURL,
		config.Server.Mode.IsProd(),
	); err != nil {
		return err
	}
	if err := validateOAuthProvider(
		"github",
		config.GitHub.ClientID,
		config.GitHub.ClientSecret,
		config.GitHub.RedirectURL,
		config.Server.Mode.IsProd(),
	); err != nil {
		return err
	}
	if !validSecret(config.Resend.APIKey, 8) ||
		strings.TrimSpace(config.Resend.FromEmail) == "" ||
		strings.TrimSpace(config.Resend.FromName) == "" ||
		validateAbsoluteHTTPURL(
			config.Resend.ResetURL,
			config.Server.Mode.IsProd(),
		) != nil ||
		validateAbsoluteHTTPURL(
			config.Resend.VerificationURL,
			config.Server.Mode.IsProd(),
		) != nil ||
		validateAbsoluteHTTPURL(
			config.Resend.InvitationURL,
			config.Server.Mode.IsProd(),
		) != nil {
		return configInvalid("resend")
	}
	if err := validateOutbox(config.Outbox); err != nil {
		return err
	}
	if err := validateOperation(config.Operation); err != nil {
		return err
	}
	if err := validateAdminCommand(config.AdminCommand); err != nil {
		return err
	}
	if err := validateCampaign(config.NotificationCampaign); err != nil {
		return err
	}
	if err := validateSecurityEmail(config.SecurityEmail); err != nil {
		return err
	}
	return validateAPIJudge(config.Judge, config.Server.Mode.IsProd())
}

func validateLogger(config settings.Config) error {
	level := strings.ToLower(strings.TrimSpace(config.Logger.LogLevel))
	switch level {
	case "debug", "info", "warn", "error":
	default:
		return configInvalid("logger.log_level")
	}
	if config.Server.Mode.IsProd() && level == "debug" {
		return configInvalid("logger.log_level")
	}
	if config.Logger.FileLogName != "" &&
		(config.Logger.MaxBackups <= 0 ||
			config.Logger.MaxAge <= 0 ||
			config.Logger.MaxSize <= 0) {
		return configInvalid("logger.rotation")
	}
	return nil
}

func validateTrustedProxies(config settings.Config) error {
	if config.Server.Mode.IsProd() &&
		len(config.Server.TrustedProxies) == 0 {
		return configInvalid("server.trusted_proxies")
	}
	for _, value := range config.Server.TrustedProxies {
		value = strings.TrimSpace(value)
		if value == "0.0.0.0/0" || value == "::/0" {
			return configInvalid("server.trusted_proxies")
		}
		if net.ParseIP(value) == nil {
			if _, _, err := net.ParseCIDR(value); err != nil {
				return configInvalid("server.trusted_proxies")
			}
		}
	}
	return nil
}

func validateMigration(config settings.Migration) error {
	if config.ApplyTimeoutSeconds <= 0 ||
		config.LockTimeoutSeconds <= 0 ||
		config.ConnectTimeoutSeconds <= 0 ||
		config.LockTimeoutSeconds >= config.ApplyTimeoutSeconds ||
		config.ConnectTimeoutSeconds > config.LockTimeoutSeconds {
		return configInvalid("migration")
	}
	return nil
}

func validateBrowserSecurity(config settings.Config) error {
	authentication := config.Authentication
	production := config.Server.Mode.IsProd()
	if len(authentication.BrowserOrigins) == 0 ||
		strings.TrimSpace(authentication.RefreshCookieName) == "" ||
		strings.TrimSpace(authentication.RefreshCookiePath) == "" ||
		strings.TrimSpace(authentication.AccessCookieName) == "" ||
		strings.TrimSpace(authentication.CSRFCookieName) == "" ||
		strings.TrimSpace(authentication.PreSessionCookieName) == "" {
		return configInvalid("authentication.browser")
	}
	if production &&
		(!authentication.RefreshCookieSecure ||
			!strings.HasPrefix(
				authentication.AccessCookieName,
				"__Host-",
			) ||
			!strings.HasPrefix(
				authentication.CSRFCookieName,
				"__Host-",
			)) {
		return configInvalid("authentication.cookies")
	}
	switch strings.ToLower(
		strings.TrimSpace(authentication.RefreshCookieSameSite),
	) {
	case "strict", "lax":
	default:
		return configInvalid("authentication.refresh_cookie_same_site")
	}
	for _, origin := range authentication.BrowserOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" ||
			parsed.User != nil || parsed.RawQuery != "" ||
			parsed.Fragment != "" ||
			(parsed.Path != "" && parsed.Path != "/") ||
			(production && parsed.Scheme != "https") ||
			(!production &&
				parsed.Scheme != "http" &&
				parsed.Scheme != "https") {
			return configInvalid("authentication.browser_origins")
		}
	}
	if validateAbsoluteHTTPURL(
		authentication.OAuthCompletionURL,
		production,
	) != nil {
		return configInvalid("authentication.oauth_completion_url")
	}
	return nil
}

func validateOAuthProvider(
	field string,
	clientID string,
	clientSecret string,
	redirectURL string,
	production bool,
) error {
	if !safeText(clientID, 512) ||
		!validSecret(clientSecret, 8) ||
		validateAbsoluteHTTPURL(redirectURL, production) != nil {
		return configInvalid(field)
	}
	return nil
}

func validateAbsoluteHTTPURL(value string, production bool) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		parsed.Fragment != "" {
		return ErrInvalidConfiguration
	}
	if production && parsed.Scheme != "https" {
		return ErrInvalidConfiguration
	}
	if !production &&
		parsed.Scheme != "http" &&
		parsed.Scheme != "https" {
		return ErrInvalidConfiguration
	}
	return nil
}

func validateOutbox(config settings.Outbox) error {
	if !safeToken(config.Name, 128) ||
		!safeToken(config.OwnerPrefix, 128) ||
		config.Concurrency < 1 || config.Concurrency > 64 ||
		config.ClaimLimit < 1 || config.ClaimLimit > config.Concurrency ||
		config.ClaimLimit > 100 ||
		config.MaxMessageBytes < 1 ||
		config.MaxMessageBytes > 4<<20 ||
		config.MaxAttempts < 1 || config.MaxAttempts > 100 ||
		config.LeaseDuration <= 0 ||
		config.PublishTimeout <= 0 ||
		config.StoreTimeout <= 0 ||
		config.FinalizeTimeout <= 0 ||
		config.PublishTimeout >= config.LeaseDuration ||
		config.StoreTimeout >= config.LeaseDuration ||
		config.FinalizeTimeout >= config.LeaseDuration ||
		config.StoreTimeout+config.PublishTimeout+
			config.FinalizeTimeout >= config.LeaseDuration ||
		config.PollInterval <= 0 ||
		config.RetryInitial <= 0 ||
		config.RetryMaximum < config.RetryInitial ||
		config.RetryMultiplier <= 1 ||
		math.IsNaN(config.RetryMultiplier) ||
		math.IsInf(config.RetryMultiplier, 0) ||
		config.ReconcileInterval <= 0 ||
		config.ReconcileBatch < 1 || config.ReconcileBatch > 4096 ||
		config.MaxReconcileBatches < 1 ||
		config.MaxReconcileBatches > 1024 {
		return configInvalid("outbox")
	}
	return nil
}

func validateForge(config settings.Forge, production bool) error {
	cleanDirectory := filepath.Clean(config.DataDir)
	if !safeText(config.DataDir, 4096) ||
		cleanDirectory != config.DataDir || cleanDirectory == "." ||
		(production && !filepath.IsAbs(config.DataDir)) ||
		!safeForgeResourceName(config.Topic, 128) ||
		!safeForgeResourceName(config.ConsumerGroup, 128) ||
		config.MaxMessageBytes < 1 ||
		config.MaxMessageBytes > 4<<20 ||
		config.MaxSegmentBytes < int64(config.MaxMessageBytes) ||
		config.MaxSegmentBytes > 1<<30 ||
		config.MaxStorageBytes < config.MaxSegmentBytes ||
		config.MaxStorageBytes > 16<<30 ||
		config.RetentionBytes < config.MaxSegmentBytes ||
		config.RetentionBytes >= config.MaxStorageBytes ||
		config.RetentionTime < time.Minute ||
		config.RetentionTime > 30*24*time.Hour ||
		config.RetentionInterval < time.Second ||
		config.RetentionInterval > time.Hour {
		return configInvalid("forge")
	}
	return nil
}

func validateForgeOutboxBoundary(
	forgeConfig settings.Forge,
	outbox settings.Outbox,
) error {
	if forgeConfig.MaxMessageBytes <
		outbox.MaxMessageBytes+minimumForgeEnvelopeHeadroomBytes {
		return configInvalid("forge.max_message_bytes")
	}
	return nil
}

func validateOperation(config settings.Operation) error {
	if !safeToken(config.OwnerPrefix, 128) ||
		config.LeaseDuration < 5*time.Second ||
		config.BatchTimeout <= 0 ||
		config.BatchTimeout >= config.LeaseDuration ||
		config.IdleWait <= 0 ||
		config.RetryInitial <= 0 ||
		config.RetryMaximum < config.RetryInitial ||
		config.MaximumAttempts < 1 ||
		config.MaximumAttempts > 100 {
		return configInvalid("operation")
	}
	return nil
}

func validateCampaign(config settings.NotificationCampaign) error {
	if config.MaximumRecipients < 1 ||
		config.MaximumRecipients > 1_000_000 ||
		config.BatchSize < 1 ||
		config.BatchSize > config.MaximumRecipients ||
		config.ConfirmationTTL <= 0 ||
		config.RecentAuthenticationTTL <= 0 {
		return configInvalid("notification_campaign")
	}
	return nil
}

func validateAdminCommand(config settings.AdminCommand) error {
	if config.ConfirmationTTL < 30*time.Second ||
		config.ConfirmationTTL > 15*time.Minute ||
		config.RecentAuthenticationTTL < time.Minute ||
		config.RecentAuthenticationTTL > time.Hour {
		return configInvalid("admin_command")
	}
	return nil
}

func validateSecurityEmail(config settings.SecurityEmail) error {
	if !safeToken(config.OwnerPrefix, 128) ||
		config.PollInterval <= 0 ||
		config.ClaimTimeout <= 0 ||
		config.DeliveryTimeout <= 0 ||
		config.FinalizeTimeout <= 0 ||
		config.LeaseDuration <= 0 ||
		config.ClaimTimeout >= config.LeaseDuration ||
		config.DeliveryTimeout >= config.LeaseDuration ||
		config.FinalizeTimeout >= config.LeaseDuration ||
		config.BatchSize < 1 || config.BatchSize > 100 ||
		config.Concurrency < 1 || config.Concurrency > 64 ||
		config.MaximumAttempts < 1 ||
		config.RetryInitial <= 0 ||
		config.RetryMaximum < config.RetryInitial ||
		config.RetryMultiplier < 1 ||
		config.MaximumCiphertextBytes < 256 ||
		config.MaximumCiphertextBytes > 1<<20 {
		return configInvalid("security_email")
	}
	return nil
}

func validateJudgeCommon(config settings.Judge) error {
	if !safeToken(config.ReleaseID, 128) ||
		config.MaximumActiveSubmissionsPerUser < 1 ||
		config.MaximumReadyBacklog < 1 ||
		config.MaximumOldestReadyAge <= 0 ||
		config.MinimumFreeDiskBytes == 0 ||
		strings.TrimSpace(config.AdmissionDiskPath) == "" ||
		config.WorkerHeartbeatFreshness <= 0 ||
		config.MaximumTestInputBytes <= 0 ||
		config.MaximumOutputBytes <= 0 ||
		config.MaximumDiagnosticBytes <= 0 {
		return configInvalid("judge")
	}
	return nil
}

func validateAPIJudge(config settings.Judge, production bool) error {
	if err := validateJudgeCommon(config); err != nil {
		return err
	}
	switch config.APIAuthoringCatalogMode {
	case settings.JudgeAPICatalogApprovedRelease:
		if strings.TrimSpace(config.ReleaseManifestPath) == "" {
			return configInvalid("judge.release_manifest_path")
		}
	case settings.JudgeAPICatalogDevelopment:
		if production {
			return configInvalid("judge.api_authoring_catalog_mode")
		}
	default:
		return configInvalid("judge.api_authoring_catalog_mode")
	}
	return nil
}

func validateJudgeWorker(config settings.Judge) error {
	if err := validateJudgeCommon(config); err != nil {
		return err
	}
	if strings.TrimSpace(config.ReleaseManifestPath) == "" {
		return configInvalid("judge.release_manifest_path")
	}
	if !validDigestReference(config.DockerImage) ||
		!filepath.IsAbs(config.DockerSocketPath) ||
		!filepath.IsAbs(config.StagingRoot) ||
		config.DockerSocketPath == "/var/run/docker.sock" ||
		!config.NetworkDisabled ||
		config.PoolSize < 1 || config.WorkerCount < 1 ||
		config.CompileTimeoutMs < 1 ||
		config.ContainerMemoryMb < 64 ||
		config.SandboxTmpfsSizeMb < 8 ||
		!safeToken(config.WorkerIdentityPrefix, 128) ||
		config.LeaseDuration <= 0 ||
		config.LeaseHeartbeat <= 0 ||
		config.LeaseHeartbeat > config.LeaseDuration/4 ||
		config.LeaseRetryFloor <= 0 ||
		config.MaximumAttempts < 1 {
		return configInvalid("judge.worker")
	}
	return nil
}

func readBoundedConfig(name string) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil || info.Mode()&os.ModeSymlink != 0 ||
		!info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 ||
		info.Size() < 1 || info.Size() > maximumConfigBytes {
		return nil, configInvalid("configuration_file")
	}
	raw, err := os.ReadFile(name)
	if err != nil || len(raw) == 0 ||
		len(raw) > maximumConfigBytes ||
		!utf8.Valid(raw) {
		return nil, configInvalid("configuration_file")
	}
	return raw, nil
}

func validateConfigurationKeys(
	raw []byte,
	configType reflect.Type,
) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil ||
		len(document.Content) != 1 {
		return configInvalid("configuration_file")
	}
	if err := validateMappingNode(
		document.Content[0],
		configType,
		"",
	); err != nil {
		return err
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return configInvalid("configuration_file")
	}
	return nil
}

func validateMappingNode(
	node *yaml.Node,
	valueType reflect.Type,
	path string,
) error {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType.Kind() != reflect.Struct ||
		node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	fields := make(map[string]reflect.Type, valueType.NumField())
	for index := 0; index < valueType.NumField(); index++ {
		field := valueType.Field(index)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fields[name] = field.Type
	}
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index]
		value := node.Content[index+1]
		fieldType, exists := fields[key.Value]
		fieldPath := key.Value
		if path != "" {
			fieldPath = path + "." + key.Value
		}
		if !exists {
			return configInvalid(fieldPath)
		}
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct &&
			value.Kind == yaml.MappingNode {
			if err := validateMappingNode(
				value,
				fieldType,
				fieldPath,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func safeEnvironmentName(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func safeToken(value string, maximum int) bool {
	if !safeText(value, maximum) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' ||
			character == '-' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func safeForgeResourceName(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return value != "." && value != ".."
}

func safeText(value string, maximum int) bool {
	if value == "" ||
		len(value) > maximum ||
		strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) ||
		strings.Contains(value, "${") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validSecret(value string, minimum int) bool {
	if len(value) < minimum ||
		len(value) > maximumSecretBytes ||
		!utf8.ValidString(value) ||
		strings.Contains(value, "${") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func safeBuildLabel(value string) bool {
	if !safeText(value, 128) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' ||
			character == '-' || character == '+' {
			continue
		}
		return false
	}
	return true
}

func validDigestReference(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 ||
		strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	digest := value
	if marker := strings.LastIndex(value, "@sha256:"); marker >= 0 {
		if marker == 0 || strings.Contains(value[:marker], "@") {
			return false
		}
		digest = value[marker+1:]
	} else if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	body := strings.TrimPrefix(digest, "sha256:")
	if len(body) != 64 {
		return false
	}
	distinct := false
	for index := range body {
		character := body[index]
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return false
		}
		if index > 0 && character != body[0] {
			distinct = true
		}
	}
	return distinct
}

func configInvalid(field string) error {
	return &ConfigError{Field: field}
}

func (profile Profile) valid() bool {
	switch profile {
	case ProfileAPI, ProfileJudgeWorker, ProfileMigrate,
		ProfileOperator, ProfileAll:
		return true
	default:
		return false
	}
}

func (binding secretBinding) requiredBy(profile Profile) bool {
	for _, candidate := range binding.profiles {
		if candidate == profile {
			return true
		}
	}
	return false
}

// These wrappers retain the focused loader contracts used by existing tests
// and local tooling while production startup uses the file-aware binding path.
func bindJWTSecret(loader *viper.Viper) error {
	return bindEnvironmentValue(loader, "jwt.secret", "JWT_SECRET")
}

func bindPasswordResetDelivery(loader *viper.Viper) error {
	for _, binding := range [][2]string{
		{"resend.api_key", "RESEND_API_KEY"},
		{"resend.from_email", "RESEND_FROM_EMAIL"},
		{"resend.from_name", "RESEND_FROM_NAME"},
		{"resend.reset_url", "RESEND_RESET_URL"},
		{"resend.verification_url", "RESEND_VERIFICATION_URL"},
		{"resend.invitation_url", "RESEND_INVITATION_URL"},
	} {
		if err := bindEnvironmentValue(
			loader,
			binding[0],
			binding[1],
		); err != nil {
			return err
		}
	}
	return nil
}
