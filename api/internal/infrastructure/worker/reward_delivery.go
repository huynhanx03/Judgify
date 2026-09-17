package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/algorithm"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/mq/forge"
	commonoutbox "github.com/huynhanx03/go-common/pkg/mq/outbox"
	outboxforge "github.com/huynhanx03/go-common/pkg/mq/outbox/forge"

	cultivationdb "github.com/huynhanx03/judgify/internal/cultivation/adapters/driven/db"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/judge"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	"github.com/huynhanx03/judgify/pkg/settings"
)

const (
	rewardDeliveryTopicLimit            = 2
	rewardDeliveryProducerLimit         = 2
	rewardDeliveryConsumerLimit         = 1
	rewardDeliveryConsumerGroupLimit    = 1
	rewardDeliveryEnvelopeHeadroomBytes = 32 << 10
)

var ErrRewardDeliveryUnavailable = errors.New(
	"solve reward delivery: unavailable",
)

// rewardDelivery is the worker-owned PostgreSQL outbox -> Forge ->
// Cultivation projection boundary. Composition is inert: filesystem leases,
// producers, consumers, and goroutines are opened only by Run.
type rewardDelivery struct {
	forgeConfig  settings.Forge
	outboxConfig settings.Outbox
	database     *internalent.EntClient
	logger       *zap.Logger

	mu       sync.Mutex
	started  bool
	stopped  bool
	cancel   context.CancelFunc
	done     chan struct{}
	doneOnce sync.Once

	broker    *forge.Broker
	relay     *commonoutbox.Relay
	relayLive bool
	worker    *judge.SolveRewardWorker
}

func newRewardDelivery(
	forgeConfig settings.Forge,
	outboxConfig settings.Outbox,
	database *internalent.EntClient,
	baseLogger *zap.Logger,
) (*rewardDelivery, error) {
	if !validRewardDeliveryConfig(forgeConfig, outboxConfig) ||
		database == nil || database.Client() == nil || baseLogger == nil {
		return nil, ErrInvalidWorker
	}
	return &rewardDelivery{
		forgeConfig:  forgeConfig,
		outboxConfig: outboxConfig,
		database:     database,
		logger:       baseLogger,
		done:         make(chan struct{}),
	}, nil
}

func (*rewardDelivery) Name() string { return "solve-reward-delivery" }

func (delivery *rewardDelivery) Run(ctx context.Context) error {
	if delivery == nil || ctx == nil {
		return ErrInvalidWorker
	}
	delivery.mu.Lock()
	if delivery.started || delivery.stopped {
		delivery.mu.Unlock()
		return ErrInvalidWorker
	}
	runCtx, cancel := context.WithCancel(logger.WithContext(ctx, delivery.logger))
	delivery.started, delivery.cancel = true, cancel
	delivery.mu.Unlock()
	defer delivery.finish()
	defer cancel()

	if err := delivery.initialize(runCtx); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			30*time.Second,
		)
		defer cleanupCancel()
		return errors.Join(err, delivery.shutdownOwned(cleanupCtx))
	}

	delivery.mu.Lock()
	relay := delivery.relay
	delivery.mu.Unlock()
	if relay == nil {
		return errors.Join(ErrRewardDeliveryUnavailable, delivery.shutdownOwned(context.Background()))
	}
	delivery.mu.Lock()
	delivery.relayLive = true
	delivery.mu.Unlock()
	runErr := relay.Run(runCtx)
	delivery.mu.Lock()
	delivery.relayLive = false
	delivery.mu.Unlock()
	cleanupCtx, cleanupCancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cleanupCancel()
	cleanupErr := delivery.shutdownOwned(cleanupCtx)
	if ctx.Err() != nil || runCtx.Err() != nil {
		return cleanupErr
	}
	if runErr == nil {
		runErr = errors.New("solve reward relay stopped unexpectedly")
	}
	return errors.Join(runErr, cleanupErr)
}

func (delivery *rewardDelivery) initialize(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidWorker
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	config := delivery.forgeConfig
	broker, err := forge.NewBroker(
		config.DataDir,
		forge.WithMaxTopics(rewardDeliveryTopicLimit),
		forge.WithMaxProducers(rewardDeliveryProducerLimit),
		forge.WithMaxConsumers(rewardDeliveryConsumerLimit),
		forge.WithMaxConsumerGroups(rewardDeliveryConsumerGroupLimit),
		forge.WithMaxMessageSize(config.MaxMessageBytes),
		forge.WithMaxSegmentBytes(config.MaxSegmentBytes),
		forge.WithMaxStorageBytes(config.MaxStorageBytes),
		forge.WithRetentionBytes(config.RetentionBytes),
		forge.WithRetentionTime(config.RetentionTime),
		forge.WithRetentionInterval(config.RetentionInterval),
		forge.WithOnRetentionError(func(err error) {
			if err != nil {
				delivery.logger.Warn(
					"solve reward Forge retention failed",
					zap.String("error_type", fmt.Sprintf("%T", err)),
				)
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("open solve reward Forge broker: %w", err)
	}
	delivery.mu.Lock()
	delivery.broker = broker
	delivery.mu.Unlock()

	producer, err := broker.NewProducer(config.Topic)
	if err != nil {
		return fmt.Errorf("open solve reward Forge producer: %w", err)
	}
	publisher, err := outboxforge.NewPublisher(map[string]*forge.Producer{
		submissioncontract.EventProblemSolveStatusChangedV1: producer,
	})
	if err != nil {
		return fmt.Errorf("compose solve reward Forge publisher: %w", err)
	}
	consumer, _, err := broker.NewDLQConsumer(
		config.ConsumerGroup,
		config.Topic,
	)
	if err != nil {
		return fmt.Errorf("open solve reward Forge consumer: %w", err)
	}
	projector, err := cultivationdb.NewSolveRewardProjector(delivery.database)
	if err != nil {
		return fmt.Errorf("compose solve reward projector: %w", err)
	}
	store, err := eventing.NewStore(delivery.database.Client())
	if err != nil {
		return fmt.Errorf("compose solve reward outbox store: %w", err)
	}
	relay, err := commonoutbox.New(
		store,
		publisher,
		nil,
		rewardDeliveryRelayOptions(delivery.outboxConfig),
	)
	if err != nil {
		return fmt.Errorf("compose solve reward outbox relay: %w", err)
	}
	worker, err := judge.NewSolveRewardWorker(
		consumer,
		projector,
		delivery.logger.Named("consumer"),
		judge.SolveRewardWorkerOptions{
			ReconcileInterval:   delivery.outboxConfig.ReconcileInterval,
			ReconcileBatch:      delivery.outboxConfig.ReconcileBatch,
			MaxReconcileBatches: delivery.outboxConfig.MaxReconcileBatches,
		},
	)
	if err != nil {
		return fmt.Errorf("compose solve reward worker: %w", err)
	}
	delivery.mu.Lock()
	delivery.relay, delivery.worker = relay, worker
	delivery.mu.Unlock()

	// Start is synchronous until bounded canonical PostgreSQL reconciliation
	// converges. Only then does it launch Forge consumption, and only after that
	// succeeds may this component run the PostgreSQL-to-Forge relay.
	if err := ctx.Err(); err != nil {
		return err
	}
	// The consumer has its own cancellation root so lifecycle cancellation
	// cannot stop it before relay.Shutdown has drained/finalized admissions.
	// Shutdown then cancels and drains the worker before closing the broker.
	if err := worker.Start(context.WithoutCancel(ctx)); err != nil {
		return err
	}
	return nil
}

func (delivery *rewardDelivery) Shutdown(ctx context.Context) error {
	if delivery == nil || ctx == nil {
		return ErrInvalidWorker
	}
	delivery.mu.Lock()
	started, cancel := delivery.started, delivery.cancel
	delivery.stopped = true
	delivery.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if !started {
		delivery.finish()
		return nil
	}
	shutdownErr := delivery.shutdownOwned(ctx)
	select {
	case <-delivery.done:
		return shutdownErr
	case <-ctx.Done():
		return errors.Join(shutdownErr, ctx.Err())
	}
}

func (delivery *rewardDelivery) shutdownOwned(ctx context.Context) error {
	if delivery == nil || ctx == nil {
		return ErrInvalidWorker
	}
	delivery.mu.Lock()
	relay, worker, broker := delivery.relay, delivery.worker, delivery.broker
	delivery.mu.Unlock()
	var failures []error
	// Admission stops first, so no PostgreSQL row can be finalized after its
	// producer is closed. The worker then rolls any uncommitted fetched tail
	// back before Broker.Close drains producers and releases filesystem leases.
	if relay != nil {
		failures = append(failures, relay.Shutdown(ctx))
	}
	if worker != nil {
		failures = append(failures, worker.Shutdown(ctx))
	}
	if broker != nil {
		failures = append(failures, broker.Close())
	}
	return errors.Join(failures...)
}

func (delivery *rewardDelivery) Check(ctx context.Context) error {
	if delivery == nil || ctx == nil {
		return ErrRewardDeliveryUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	delivery.mu.Lock()
	started, stopped := delivery.started, delivery.stopped
	broker, worker, relayLive :=
		delivery.broker, delivery.worker, delivery.relayLive
	delivery.mu.Unlock()
	if !started || stopped || !relayLive || broker == nil || worker == nil {
		return ErrRewardDeliveryUnavailable
	}
	if err := errors.Join(broker.Check(ctx), worker.Check(ctx)); err != nil {
		return err
	}
	snapshot, err := delivery.ForgeMetrics(ctx)
	if err != nil {
		return err
	}
	minimumHeadroom := int64(delivery.forgeConfig.MaxMessageBytes)
	if snapshot.MainStorageLimitBytes-snapshot.MainStorageBytes <
		minimumHeadroom ||
		snapshot.DLQStorageLimitBytes-snapshot.DLQStorageBytes <
			minimumHeadroom {
		return ErrRewardDeliveryUnavailable
	}
	return nil
}

func (delivery *rewardDelivery) ForgeMetrics(
	ctx context.Context,
) (observability.ForgeRuntimeSnapshot, error) {
	if delivery == nil || ctx == nil {
		return observability.ForgeRuntimeSnapshot{}, ErrRewardDeliveryUnavailable
	}
	delivery.mu.Lock()
	broker, stopped := delivery.broker, delivery.stopped
	config := delivery.forgeConfig
	delivery.mu.Unlock()
	if broker == nil || stopped {
		return observability.ForgeRuntimeSnapshot{}, ErrRewardDeliveryUnavailable
	}
	stats, err := broker.Stats(ctx)
	if err != nil {
		return observability.ForgeRuntimeSnapshot{}, err
	}
	lag, err := broker.ConsumerLag(ctx, config.ConsumerGroup, config.Topic)
	if err != nil {
		return observability.ForgeRuntimeSnapshot{}, err
	}
	snapshot := observability.ForgeRuntimeSnapshot{ConsumerLag: lag}
	dlqTopic := config.Topic + ".dlq"
	for _, topic := range stats {
		switch topic.Topic {
		case config.Topic:
			snapshot.MainStorageBytes = topic.StorageBytes
			snapshot.MainStorageLimitBytes = topic.StorageLimit
			snapshot.MainSegments = topic.Segments
		case dlqTopic:
			snapshot.DLQStorageBytes = topic.StorageBytes
			snapshot.DLQStorageLimitBytes = topic.StorageLimit
			snapshot.DLQSegments = topic.Segments
			if topic.NewestOffset < topic.OldestOffset {
				return observability.ForgeRuntimeSnapshot{}, ErrRewardDeliveryUnavailable
			}
			snapshot.DLQRecords = topic.NewestOffset - topic.OldestOffset
		}
	}
	if snapshot.MainStorageLimitBytes == 0 ||
		snapshot.DLQStorageLimitBytes == 0 {
		return observability.ForgeRuntimeSnapshot{}, ErrRewardDeliveryUnavailable
	}
	return snapshot, nil
}

func rewardDeliveryRelayOptions(config settings.Outbox) commonoutbox.Options {
	hostname, err := os.Hostname()
	if err != nil || !safeRewardOwnerToken(hostname, 63) {
		hostname = "unknown-host"
	}
	backoff := algorithm.NewJitterBackoff(algorithm.NewExponentialBackoff(
		config.RetryInitial,
		config.RetryMaximum,
		config.RetryMultiplier,
	))
	return commonoutbox.Options{
		Name: "solve-reward-outbox",
		Owner: fmt.Sprintf(
			"%s:%s:%d:solve-reward",
			config.OwnerPrefix,
			hostname,
			os.Getpid(),
		),
		Concurrency:     1,
		ClaimLimit:      1,
		MaxMessageBytes: config.MaxMessageBytes,
		Destinations: []string{
			submissioncontract.EventProblemSolveStatusChangedV1,
		},
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

func validRewardDeliveryConfig(
	forgeConfig settings.Forge,
	outbox settings.Outbox,
) bool {
	return forgeConfig.DataDir != "" &&
		filepath.Clean(forgeConfig.DataDir) == forgeConfig.DataDir &&
		forgeConfig.DataDir != "." &&
		safeRewardOwnerToken(forgeConfig.Topic, 128) &&
		safeRewardOwnerToken(forgeConfig.ConsumerGroup, 128) &&
		forgeConfig.MaxMessageBytes > 0 &&
		forgeConfig.MaxMessageBytes <= 4<<20 &&
		forgeConfig.MaxSegmentBytes >= int64(forgeConfig.MaxMessageBytes) &&
		forgeConfig.MaxSegmentBytes <= 1<<30 &&
		forgeConfig.MaxStorageBytes >= forgeConfig.MaxSegmentBytes &&
		forgeConfig.MaxStorageBytes <= 16<<30 &&
		forgeConfig.RetentionBytes >= forgeConfig.MaxSegmentBytes &&
		forgeConfig.RetentionBytes < forgeConfig.MaxStorageBytes &&
		forgeConfig.RetentionTime >= time.Minute &&
		forgeConfig.RetentionTime <= 30*24*time.Hour &&
		forgeConfig.RetentionInterval >= time.Second &&
		forgeConfig.RetentionInterval <= time.Hour &&
		safeRewardOwnerPrefix(outbox.OwnerPrefix, 128) &&
		outbox.MaxMessageBytes > 0 &&
		outbox.MaxMessageBytes <= 4<<20 &&
		forgeConfig.MaxMessageBytes >=
			outbox.MaxMessageBytes+rewardDeliveryEnvelopeHeadroomBytes &&
		outbox.MaxAttempts > 0 && outbox.MaxAttempts <= 100 &&
		outbox.LeaseDuration > 0 &&
		outbox.PublishTimeout > 0 &&
		outbox.StoreTimeout > 0 &&
		outbox.FinalizeTimeout > 0 &&
		outbox.PublishTimeout < outbox.LeaseDuration &&
		outbox.StoreTimeout < outbox.LeaseDuration &&
		outbox.FinalizeTimeout < outbox.LeaseDuration &&
		outbox.StoreTimeout+outbox.PublishTimeout+
			outbox.FinalizeTimeout < outbox.LeaseDuration &&
		outbox.PollInterval > 0 &&
		outbox.RetryInitial > 0 &&
		outbox.RetryMaximum >= outbox.RetryInitial &&
		outbox.RetryMultiplier > 1 &&
		!math.IsNaN(outbox.RetryMultiplier) &&
		!math.IsInf(outbox.RetryMultiplier, 0) &&
		outbox.ReconcileInterval > 0 &&
		outbox.ReconcileBatch > 0 && outbox.ReconcileBatch <= 4096 &&
		outbox.MaxReconcileBatches > 0 &&
		outbox.MaxReconcileBatches <= 1024
}

func safeRewardOwnerPrefix(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' ||
			character == '-' || character == ':' {
			continue
		}
		return false
	}
	return value != "." && value != ".."
}

func safeRewardOwnerToken(value string, maximum int) bool {
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

func (delivery *rewardDelivery) finish() {
	if delivery == nil {
		return
	}
	delivery.doneOnce.Do(func() { close(delivery.done) })
}
