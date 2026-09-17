package infrastructure

import (
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/huynhanx03/go-common/pkg/algorithm"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"

	"github.com/huynhanx03/judgify/internal/di"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	platformdelivery "github.com/huynhanx03/judgify/internal/platform/eventdelivery"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/pkg/settings"
)

var ErrInvalidEventDelivery = errors.New("compose event delivery: invalid dependencies")

func NewEventDelivery(
	container *di.Container,
	database *internalent.EntClient,
	config settings.Outbox,
	loggerProvider *commonlogger.LoggerZap,
) (*platformdelivery.Runtime, error) {
	if container == nil || database == nil || database.Client() == nil ||
		loggerProvider == nil {
		return nil, ErrInvalidEventDelivery
	}
	config = normalizeEventDeliveryConfig(config)
	if err := validateEventDeliveryConfig(config); err != nil {
		return nil, err
	}
	routes, reconcilers, err := eventDeliveryRoutes(container, database)
	if err != nil {
		return nil, err
	}
	router, err := commonoutbox.NewRouter(routes...)
	if err != nil {
		return nil, fmt.Errorf("compose event router: %w", err)
	}
	store, err := eventing.NewStore(database.Client())
	if err != nil {
		return nil, fmt.Errorf("compose event store: %w", err)
	}
	relay, err := commonoutbox.New(
		store,
		router,
		nil,
		eventDeliveryRelayOptions(config, router.Destinations()),
	)
	if err != nil {
		return nil, fmt.Errorf("compose event relay: %w", err)
	}
	runtime, err := platformdelivery.New(
		relay,
		store,
		reconcilers,
		platformdelivery.Options{
			ReconcileInterval: config.ReconcileInterval,
			ReconcileBatch:    config.ReconcileBatch,
			MaxBatchesPerRun:  config.MaxReconcileBatches,
		},
		loggerProvider.Named("event-delivery"),
	)
	if err != nil {
		return nil, fmt.Errorf("compose event delivery runtime: %w", err)
	}
	return runtime, nil
}

func validateEventDeliveryConfig(config settings.Outbox) error {
	if config.Concurrency <= 0 || config.ClaimLimit <= 0 ||
		config.ClaimLimit > config.Concurrency || config.ClaimLimit > 100 ||
		config.MaxMessageBytes <= 0 || config.MaxAttempts == 0 ||
		config.RetryInitial <= 0 || config.RetryMaximum < config.RetryInitial ||
		config.RetryMultiplier <= 1 ||
		math.IsNaN(config.RetryMultiplier) ||
		math.IsInf(config.RetryMultiplier, 0) ||
		config.ReconcileInterval <= 0 || config.ReconcileBatch <= 0 ||
		config.ReconcileBatch > 4096 || config.MaxReconcileBatches <= 0 ||
		config.MaxReconcileBatches > 1024 {
		return ErrInvalidEventDelivery
	}
	return nil
}

func normalizeEventDeliveryConfig(config settings.Outbox) settings.Outbox {
	if config.Name == "" {
		config.Name = "integration-outbox"
	}
	if config.OwnerPrefix == "" {
		config.OwnerPrefix = "judgify"
	}
	if config.Concurrency == 0 {
		config.Concurrency = 4
	}
	if config.ClaimLimit == 0 {
		config.ClaimLimit = config.Concurrency
	}
	if config.MaxMessageBytes == 0 {
		config.MaxMessageBytes = 256 << 10
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 20
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 30 * time.Second
	}
	if config.PublishTimeout == 0 {
		config.PublishTimeout = 5 * time.Second
	}
	if config.StoreTimeout == 0 {
		config.StoreTimeout = 3 * time.Second
	}
	if config.FinalizeTimeout == 0 {
		config.FinalizeTimeout = 3 * time.Second
	}
	if config.PollInterval == 0 {
		config.PollInterval = 250 * time.Millisecond
	}
	if config.RetryInitial == 0 {
		config.RetryInitial = 250 * time.Millisecond
	}
	if config.RetryMaximum == 0 {
		config.RetryMaximum = 30 * time.Second
	}
	if config.RetryMultiplier == 0 {
		config.RetryMultiplier = 2
	}
	if config.ReconcileInterval == 0 {
		config.ReconcileInterval = 30 * time.Second
	}
	if config.ReconcileBatch == 0 {
		config.ReconcileBatch = 256
	}
	if config.MaxReconcileBatches == 0 {
		config.MaxReconcileBatches = 32
	}
	return config
}

func eventDeliveryRelayOptions(
	config settings.Outbox,
	destinations []string,
) commonoutbox.Options {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	backoff := algorithm.NewJitterBackoff(algorithm.NewExponentialBackoff(
		config.RetryInitial,
		config.RetryMaximum,
		config.RetryMultiplier,
	))
	return commonoutbox.Options{
		Name:            config.Name,
		Owner:           fmt.Sprintf("%s:%s:%d", config.OwnerPrefix, hostname, os.Getpid()),
		Concurrency:     config.Concurrency,
		ClaimLimit:      config.ClaimLimit,
		MaxMessageBytes: config.MaxMessageBytes,
		// API ownership is exactly its closed local route catalog. Worker-owned
		// destinations remain pending until the corresponding worker relay claims
		// them and can never be stolen/dead-lettered by this generic dispatcher.
		Destinations:    append([]string(nil), destinations...),
		MaxAttempts:     config.MaxAttempts,
		LeaseDuration:   config.LeaseDuration,
		PublishTimeout:  config.PublishTimeout,
		StoreTimeout:    config.StoreTimeout,
		FinalizeTimeout: config.FinalizeTimeout,
		PollInterval:    config.PollInterval,
		Backoff: commonoutbox.BackoffFunc(func(attempt uint32) time.Duration {
			return backoff.Delay(int(attempt))
		}),
	}
}
