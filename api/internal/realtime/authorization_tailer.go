package realtime

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultPollInterval   = 2 * time.Second
	defaultReadinessStale = 30 * time.Second
	minimumReadinessPolls = 3
	maximumReadinessStale = 5 * time.Minute
	maximumFailureBackoff = 30 * time.Second
)

type AuthorizationTailerConfig struct {
	PollInterval        time.Duration
	ReadinessStaleAfter time.Duration
}

func DefaultAuthorizationTailerConfig() AuthorizationTailerConfig {
	return AuthorizationTailerConfig{
		PollInterval:        defaultPollInterval,
		ReadinessStaleAfter: defaultReadinessStale,
	}
}

func normalizeAuthorizationTailerConfig(
	config AuthorizationTailerConfig,
) (AuthorizationTailerConfig, error) {
	if config.PollInterval == 0 {
		config.PollInterval = defaultPollInterval
	}
	minimumStaleness := minimumReadinessPolls * config.PollInterval
	if config.ReadinessStaleAfter == 0 {
		config.ReadinessStaleAfter = max(defaultReadinessStale, minimumStaleness)
	}
	if config.PollInterval <= 0 || config.PollInterval > time.Minute ||
		config.ReadinessStaleAfter < minimumStaleness ||
		config.ReadinessStaleAfter > maximumReadinessStale {
		return AuthorizationTailerConfig{}, ErrInvalidConfig
	}
	return config, nil
}

// AuthorizationTailer is a fail-safe socket fence for the committed singleton
// authorization revision. The durable identity outbox route owns browser
// invalidations; this poller only terminates stale authenticated connections
// when that route is delayed or unavailable. Every reconnect obtains the
// authoritative REST projection.
type AuthorizationTailer struct {
	config       AuthorizationTailerConfig
	source       AuthorizationRevisionSource
	revoker      ConnectionRevoker
	logger       *zap.Logger
	life         componentLifecycle
	bootstrapped atomic.Bool
	revision     atomic.Int64
	health       *activityHealth
}

func NewAuthorizationTailer(
	config AuthorizationTailerConfig,
	source AuthorizationRevisionSource,
	publisher AuthorizationChangePublisher,
	revoker ConnectionRevoker,
	logger *zap.Logger,
) (*AuthorizationTailer, error) {
	return newAuthorizationTailer(config, source, publisher, revoker, logger, time.Now)
}

func newAuthorizationTailer(
	config AuthorizationTailerConfig,
	source AuthorizationRevisionSource,
	publisher AuthorizationChangePublisher,
	revoker ConnectionRevoker,
	logger *zap.Logger,
	clock func() time.Time,
) (*AuthorizationTailer, error) {
	normalized, err := normalizeAuthorizationTailerConfig(config)
	// The durable identity outbox route is the only producer of browser
	// invalidations. Retain the publisher argument for source compatibility
	// with existing construction sites, but do not use state polling to create
	// another realtime event stream.
	_ = publisher
	if err != nil || source == nil || revoker == nil || logger == nil {
		return nil, ErrInvalidConfig
	}
	health, err := newActivityHealth(clock)
	if err != nil {
		return nil, err
	}
	return &AuthorizationTailer{
		config:  normalized,
		source:  source,
		revoker: revoker,
		logger:  logger.Named("authorization-tailer"),
		life:    newComponentLifecycle(),
		health:  health,
	}, nil
}

func (*AuthorizationTailer) Name() string { return "realtime-authorization-tailer" }

func (tailer *AuthorizationTailer) Run(ctx context.Context) error {
	if tailer == nil {
		return ErrInvalidLifecycle
	}
	runContext, err := tailer.life.start(ctx)
	if err != nil {
		if ctx != nil && ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer tailer.life.finish()
	runContext = commonlogger.WithContext(runContext, tailer.logger)
	log := commonlogger.FromContext(runContext)
	log.Info(
		"authorization realtime tailer started",
		zap.Duration("poll_interval", tailer.config.PollInterval),
		zap.Duration("readiness_stale_after", tailer.config.ReadinessStaleAfter),
	)
	if !tailer.bootstrap(runContext) {
		log.Info("authorization realtime tailer stopped")
		return nil
	}
	timer := time.NewTimer(tailer.config.PollInterval)
	defer timer.Stop()
	failureDelay := tailer.config.PollInterval
	failureCeiling := max(maximumFailureBackoff, tailer.config.PollInterval)
	for {
		select {
		case <-runContext.Done():
			log.Info("authorization realtime tailer stopped")
			return nil
		case <-timer.C:
			if tailer.runPoll(runContext) {
				failureDelay = tailer.config.PollInterval
			} else {
				failureDelay = min(failureCeiling, failureDelay*2)
			}
			timer.Reset(failureDelay)
		}
	}
}

func (tailer *AuthorizationTailer) Shutdown(ctx context.Context) error {
	if tailer == nil {
		return ErrInvalidLifecycle
	}
	return tailer.life.shutdown(ctx)
}

func (tailer *AuthorizationTailer) Check(ctx context.Context) error {
	if tailer == nil || ctx == nil {
		return ErrHealthUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !tailer.life.running() || !tailer.bootstrapped.Load() ||
		!tailer.health.available(tailer.config.ReadinessStaleAfter) {
		return ErrHealthUnavailable
	}
	return nil
}

func (tailer *AuthorizationTailer) CurrentRevision() int64 {
	if tailer == nil {
		return 0
	}
	return tailer.revision.Load()
}

func (tailer *AuthorizationTailer) bootstrap(ctx context.Context) bool {
	timer := time.NewTimer(0)
	defer timer.Stop()
	retryDelay := tailer.config.PollInterval
	retryCeiling := max(maximumFailureBackoff, tailer.config.PollInterval)
	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			operationContext := tailer.operationContext(ctx)
			change, err := tailer.source.Current(operationContext)
			if err == nil {
				err = change.Validate()
			}
			if err == nil {
				tailer.revision.Store(change.Event.Revision)
				tailer.health.succeeded()
				tailer.bootstrapped.Store(true)
				commonlogger.FromContext(operationContext).Info(
					"authorization realtime revision initialized",
					zap.Int64("authorization_revision", change.Event.Revision),
				)
				return true
			}
			tailer.health.failed()
			commonlogger.FromContext(operationContext).Error(
				"authorization realtime revision initialization failed",
				zap.Error(err),
				zap.Duration("retry_in", retryDelay),
			)
			timer.Reset(retryDelay)
			retryDelay = min(retryCeiling, retryDelay*2)
		}
	}
}

func (tailer *AuthorizationTailer) runPoll(ctx context.Context) bool {
	operationContext := tailer.operationContext(ctx)
	fenced, err := tailer.pollOnce(operationContext)
	if err != nil {
		if ctx.Err() == nil {
			tailer.health.failed()
			commonlogger.FromContext(operationContext).Warn(
				"authorization realtime poll failed",
				zap.Error(err),
			)
		}
		return false
	}
	tailer.health.succeeded()
	if fenced {
		commonlogger.FromContext(operationContext).Debug(
			"authorization realtime revision fenced",
			zap.Int64("authorization_revision", tailer.CurrentRevision()),
		)
	}
	return true
}

func (tailer *AuthorizationTailer) pollOnce(ctx context.Context) (bool, error) {
	if tailer == nil || ctx == nil {
		return false, ErrInvalidConfig
	}
	change, err := tailer.source.Current(ctx)
	if err != nil {
		return false, fmt.Errorf("fetch authorization realtime revision: %w", err)
	}
	if err := change.Validate(); err != nil {
		return false, err
	}
	current := tailer.revision.Load()
	if change.Event.Revision < current {
		return false, ErrInvalidChange
	}
	if change.Event.Revision == current {
		return false, nil
	}
	revoked, err := tailer.revoker.RevokeAllAuthenticated(ctx)
	if err != nil {
		return false, fmt.Errorf("revoke stale authorization connections: %w", err)
	}
	tailer.revision.Store(change.Event.Revision)
	commonlogger.FromContext(ctx).Info(
		"stale authorization realtime connections fenced",
		zap.Int64("authorization_revision", change.Event.Revision),
		zap.Int("connection_count", revoked),
	)
	return true, nil
}

func (tailer *AuthorizationTailer) operationContext(parent context.Context) context.Context {
	ctx := correlation.WithContext(parent, correlation.New())
	return commonlogger.WithContext(ctx, tailer.logger)
}
