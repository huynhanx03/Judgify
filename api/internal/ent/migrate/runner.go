// Package migration applies Judgify's immutable, checked-in Atlas migration
// chain. It is used only by the finite operator role and never by API or worker
// startup.
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	atlaspostgres "ariga.io/atlas/sql/postgres"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlclient"
	// The operator binary deliberately excludes the Ent client, which is where
	// the rest of the product registers the database/sql PostgreSQL driver.
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	commonsettings "github.com/huynhanx03/go-common/pkg/settings"

	"github.com/huynhanx03/judgify/pkg/settings"
)

const (
	defaultApplyTimeout   = 15 * time.Minute
	defaultLockTimeout    = 30 * time.Second
	defaultConnectTimeout = 10 * time.Second
	maxApplyTimeout       = 24 * time.Hour
	maxLockTimeout        = 10 * time.Minute
	maxConnectTimeout     = 2 * time.Minute
	migrationLockName     = "judgify-schema-migrations"
	migrationPoolSize     = 2 // one advisory-lock connection plus one executor connection
)

var (
	ErrInvalidConfiguration = errors.New("migration: invalid configuration")
	ErrUnsupportedDatabase  = errors.New("migration: PostgreSQL is required")
)

type Runner struct {
	logger         *zap.Logger
	applyTimeout   time.Duration
	lockTimeout    time.Duration
	connectTimeout time.Duration
	operator       string
}

func New(logger *zap.Logger, config settings.Migration) (*Runner, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	applyTimeout, err := secondsOrDefault(
		config.ApplyTimeoutSeconds,
		defaultApplyTimeout,
		maxApplyTimeout,
		"apply timeout",
	)
	if err != nil {
		return nil, err
	}
	lockTimeout, err := secondsOrDefault(
		config.LockTimeoutSeconds,
		defaultLockTimeout,
		maxLockTimeout,
		"lock timeout",
	)
	if err != nil {
		return nil, err
	}
	connectTimeout, err := secondsOrDefault(
		config.ConnectTimeoutSeconds,
		defaultConnectTimeout,
		maxConnectTimeout,
		"connect timeout",
	)
	if err != nil {
		return nil, err
	}
	if lockTimeout > applyTimeout || connectTimeout > applyTimeout {
		return nil, fmt.Errorf("%w: migration timeouts exceed apply timeout", ErrInvalidConfiguration)
	}
	return &Runner{
		logger:         logger,
		applyTimeout:   applyTimeout,
		lockTimeout:    lockTimeout,
		connectTimeout: connectTimeout,
		operator:       operatorVersion(),
	}, nil
}

func (runner *Runner) Apply(ctx context.Context, config settings.Config) (resultErr error) {
	if runner == nil || runner.logger == nil || ctx == nil {
		return ErrInvalidConfiguration
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateDatabaseConfig(config.Database); err != nil {
		return err
	}

	ctx = correlation.EnsureContext(ctx)
	ctx = commonlogger.WithContext(ctx, runner.logger)
	ctx, cancel := context.WithTimeout(ctx, runner.applyTimeout)
	defer cancel()
	startedAt := time.Now()
	log := commonlogger.FromContext(ctx).Named("migration")
	outcome := "failed"
	latestVersion := ""
	fileCount := 0
	defer func() {
		if resultErr != nil {
			if contextErr := ctx.Err(); contextErr != nil && !errors.Is(resultErr, contextErr) {
				resultErr = errors.Join(resultErr, fmt.Errorf("migration context: %w", contextErr))
			}
			resultErr = redactMigrationError(resultErr)
			log.Error("checked-in migration run failed",
				zap.String("outcome", outcome),
				zap.String("latest_version", latestVersion),
				zap.Int("file_count", fileCount),
				zap.Duration("duration", time.Since(startedAt)),
				zap.String("error_type", errorType(resultErr)),
				zap.Error(resultErr),
			)
			return
		}
		log.Info("checked-in migration run completed",
			zap.String("outcome", outcome),
			zap.String("latest_version", latestVersion),
			zap.Int("file_count", fileCount),
			zap.Duration("duration", time.Since(startedAt)),
		)
	}()
	log.Info("checked-in migration run started",
		zap.String("database_host", config.Database.Host),
		zap.Int("database_port", config.Database.Port),
		zap.String("database_name", config.Database.Database),
		zap.Duration("apply_timeout", runner.applyTimeout),
		zap.Duration("lock_timeout", runner.lockTimeout),
	)

	directory, err := loadEmbeddedDirectory(ctx)
	if err != nil {
		return fmt.Errorf("prepare checked-in migration directory: %w", err)
	}
	files, err := directory.Files()
	if err != nil {
		return fmt.Errorf("list checked-in migrations: %w", err)
	}
	fileCount = len(files)
	latestVersion = files[len(files)-1].Version()
	log.Debug("checked-in migration directory verified",
		zap.Int("file_count", fileCount),
		zap.String("latest_version", latestVersion),
	)

	client, err := runner.openClient(ctx, config.Database)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close migration database: %w", closeErr))
		}
	}()
	driverStatements, err := loadDriverStatements(ctx, directory, client.Driver)
	if err != nil {
		return err
	}
	log.Debug("PostgreSQL migration statements verified",
		zap.Int("file_count", len(driverStatements)),
	)

	locker, ok := client.Driver.(schema.Locker)
	if !ok {
		return errors.New("migration: PostgreSQL driver does not support advisory locking")
	}
	lockStartedAt := time.Now()
	log.Debug("waiting for migration advisory lock",
		zap.String("lock_name", migrationLockName),
		zap.Duration("lock_timeout", runner.lockTimeout),
	)
	unlock, err := locker.Lock(ctx, migrationLockName, runner.lockTimeout)
	if err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	log.Debug("migration advisory lock acquired",
		zap.String("lock_name", migrationLockName),
		zap.Duration("lock_wait", time.Since(lockStartedAt)),
	)
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("release migration advisory lock: %w", unlockErr))
			return
		}
		log.Debug("migration advisory lock released", zap.String("lock_name", migrationLockName))
	}()

	revisions, err := newRevisionStore(client.DB)
	if err != nil {
		return err
	}
	historyExists, err := revisions.Exists(ctx)
	if err != nil {
		return err
	}
	if !historyExists {
		// Refuse a legacy/dirty database before even creating Atlas' operational
		// table. Operators must explicitly reconcile or baseline it offline.
		if err := client.CheckClean(ctx, revisions.Ident()); err != nil {
			return fmt.Errorf("migration: refuse database without revision history: %w", err)
		}
	}
	if err := revisions.Init(ctx); err != nil {
		return err
	}
	history, err := revisions.ReadRevisions(ctx)
	if err != nil {
		return err
	}
	if err := validateRevisionHistory(
		ctx,
		directory,
		history,
		func(file atlasmigrate.File) ([]string, error) {
			statements, exists := driverStatements[file.Name()]
			if !exists {
				return nil, historyDrift("embedded migration %q has no PostgreSQL parse result", file.Name())
			}
			return statements, nil
		},
	); err != nil {
		return err
	}
	log.Debug("database revision history verified",
		zap.Int("revision_count", len(history)),
	)
	migrationLog := &atlasLogger{ctx: ctx}
	executor, err := atlasmigrate.NewExecutor(
		client.Driver,
		directory,
		revisions,
		atlasmigrate.WithExecOrder(atlasmigrate.ExecOrderLinear),
		atlasmigrate.WithLogger(migrationLog),
		atlasmigrate.WithOperatorVersion(runner.operator),
	)
	if err != nil {
		return fmt.Errorf("build Atlas migration executor: %w", err)
	}

	pending, err := executor.Pending(ctx)
	if err != nil {
		if errors.Is(err, atlasmigrate.ErrNoPendingFiles) {
			outcome = "already_current"
			return nil
		}
		return err
	}
	log.Info("pending migration files verified",
		zap.Int("pending_file_count", len(pending)),
		zap.String("from_version", historyVersion(history)),
		zap.String("to_version", pending[len(pending)-1].Version()),
	)
	if err := runner.executePendingTransactional(
		ctx,
		client.DB,
		directory,
		history,
		pending,
		migrationLog,
	); err != nil {
		return err
	}
	outcome = "applied"
	return nil
}

// executePendingTransactional applies one immutable migration file per SQL
// transaction. Product DDL and its Atlas revision row therefore commit
// atomically, eliminating the crash window between a successful statement and
// its progress write. Files that require non-transactional PostgreSQL commands
// are rejected while the embedded directory is loaded.
func (runner *Runner) executePendingTransactional(
	ctx context.Context,
	database *sql.DB,
	directory atlasmigrate.Dir,
	history []*atlasmigrate.Revision,
	pending []atlasmigrate.File,
	logger atlasmigrate.Logger,
) error {
	if runner == nil || ctx == nil || database == nil || directory == nil || logger == nil || len(pending) == 0 {
		return ErrInvalidConfiguration
	}
	atlasmigrate.LogIntro(logger, history, pending)
	for _, file := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		transaction, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %q transaction: %w", file.Version(), err)
		}
		driver, err := atlaspostgres.Open(&boundedExecQuerier{ctx: ctx, transaction: transaction})
		if err != nil {
			return errors.Join(
				fmt.Errorf("open migration %q transaction driver: %w", file.Version(), err),
				rollbackMigration(transaction, file.Version()),
			)
		}
		revisions, err := newRevisionStore(transaction)
		if err != nil {
			return errors.Join(err, rollbackMigration(transaction, file.Version()))
		}
		executor, err := atlasmigrate.NewExecutor(
			driver,
			directory,
			revisions,
			atlasmigrate.WithExecOrder(atlasmigrate.ExecOrderLinear),
			atlasmigrate.WithLogger(logger),
			atlasmigrate.WithOperatorVersion(runner.operator),
		)
		if err != nil {
			return errors.Join(
				fmt.Errorf("build migration %q transaction executor: %w", file.Version(), err),
				rollbackMigration(transaction, file.Version()),
			)
		}
		if err := executor.Execute(ctx, file); err != nil {
			return errors.Join(err, rollbackMigration(transaction, file.Version()))
		}
		if err := transaction.Commit(); err != nil {
			return errors.Join(
				fmt.Errorf("commit migration %q transaction: %w", file.Version(), err),
				rollbackMigration(transaction, file.Version()),
			)
		}
	}
	logger.Log(atlasmigrate.LogDone{})
	return nil
}

func rollbackMigration(transaction *sql.Tx, version string) error {
	if transaction == nil {
		return nil
	}
	if err := transaction.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("rollback migration %q transaction: %w", version, err)
	}
	return nil
}

func historyVersion(history []*atlasmigrate.Revision) string {
	if len(history) == 0 {
		return ""
	}
	return history[len(history)-1].Version
}

// boundedExecQuerier prevents Atlas/PostgreSQL driver initialization from
// escaping the apply deadline when it issues its metadata query with a
// background context.
type boundedExecQuerier struct {
	ctx         context.Context
	transaction *sql.Tx
}

func (querier *boundedExecQuerier) QueryContext(
	_ context.Context,
	query string,
	args ...any,
) (*sql.Rows, error) {
	if querier == nil || querier.ctx == nil || querier.transaction == nil {
		return nil, ErrInvalidConfiguration
	}
	// database/sql owns the query context until the returned Rows are closed.
	// A derived context cannot be canceled on return without racing the
	// caller's Scan. The transaction-wide apply context is already finite and
	// authoritative, so it safely covers the complete rows lifecycle.
	return querier.transaction.QueryContext(querier.ctx, query, args...)
}

func (querier *boundedExecQuerier) ExecContext(
	ctx context.Context,
	query string,
	args ...any,
) (sql.Result, error) {
	if querier == nil || querier.ctx == nil || querier.transaction == nil {
		return nil, ErrInvalidConfiguration
	}
	bounded, cancel := mergedContext(querier.ctx, ctx)
	defer cancel()
	return querier.transaction.ExecContext(bounded, query, args...)
}

func mergedContext(base, call context.Context) (context.Context, context.CancelFunc) {
	bounded, cancel := context.WithCancel(base)
	if call == nil || call == base {
		return bounded, cancel
	}
	stop := context.AfterFunc(call, cancel)
	return bounded, func() {
		stop()
		cancel()
	}
}

func (runner *Runner) openClient(
	ctx context.Context,
	config commonsettings.Database,
) (*sqlclient.Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, runner.connectTimeout)
	defer cancel()
	dsn := postgresURL(config, runner.connectTimeout, runner.applyTimeout)
	client, err := sqlclient.Open(connectCtx, dsn)
	if err != nil {
		return nil, redactedCause("open migration database: connection details redacted", err)
	}
	client.DB.SetMaxOpenConns(migrationPoolSize)
	client.DB.SetMaxIdleConns(migrationPoolSize)
	client.DB.SetConnMaxLifetime(durationOrDefault(config.ConnMaxLifetime, 30*time.Minute))
	client.DB.SetConnMaxIdleTime(durationOrDefault(config.ConnMaxIdleTime, 5*time.Minute))
	if err := client.DB.PingContext(connectCtx); err != nil {
		return nil, errors.Join(
			redactedCause("ping migration database: connection details redacted", err),
			client.Close(),
		)
	}
	return client, nil
}

func postgresURL(
	config commonsettings.Database,
	connectTimeout time.Duration,
	statementTimeout time.Duration,
) string {
	connection := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.Username, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
		Path:   "/" + config.Database,
	}
	query := connection.Query()
	query.Set("sslmode", "disable")
	query.Set("connect_timeout", strconv.Itoa(max(1, int(math.Ceil(connectTimeout.Seconds())))))
	query.Set("statement_timeout", strconv.FormatInt(max(1, statementTimeout.Milliseconds()), 10))
	query.Set("search_path", revisionTableSchema)
	query.Set("application_name", "judgify-migrate")
	connection.RawQuery = query.Encode()
	return connection.String()
}

func validateDatabaseConfig(config commonsettings.Database) error {
	driver := strings.ToLower(strings.TrimSpace(config.Driver))
	if driver != "postgres" && driver != "postgresql" {
		return fmt.Errorf("%w: %q", ErrUnsupportedDatabase, config.Driver)
	}
	if !cleanConfigValue(config.Host) || !cleanConfigValue(config.Username) ||
		!cleanConfigValue(config.Database) || config.Password == "" ||
		config.Port < 1 || config.Port > 65_535 {
		return fmt.Errorf("%w: database connection", ErrInvalidConfiguration)
	}
	return nil
}

func cleanConfigValue(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func secondsOrDefault(value int, fallback, maximum time.Duration, field string) (time.Duration, error) {
	if value == 0 {
		return fallback, nil
	}
	if value < 0 || int64(value) > int64(math.MaxInt64/time.Second) {
		return 0, fmt.Errorf("%w: %s", ErrInvalidConfiguration, field)
	}
	duration := time.Duration(value) * time.Second
	if duration > maximum {
		return 0, fmt.Errorf("%w: %s exceeds %s", ErrInvalidConfiguration, field, maximum)
	}
	return duration, nil
}

func durationOrDefault(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 || int64(seconds) > int64(math.MaxInt64/time.Second) {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func operatorVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "judgify/devel"
	}
	version := info.Main.Version
	if len(version) > 240 {
		version = version[:240]
	}
	return "judgify/" + version
}

var _ interface {
	Apply(context.Context, settings.Config) error
} = (*Runner)(nil)
