package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestRewardDeliveryRelayOwnsOnlySolveTransitionsInSourceOrder(t *testing.T) {
	config := validRewardOutboxConfig()
	options := rewardDeliveryRelayOptions(config)
	want := []string{submissioncontract.EventProblemSolveStatusChangedV1}
	if !slices.Equal(options.Destinations, want) {
		t.Fatalf("relay destinations = %v, want %v", options.Destinations, want)
	}
	if options.Concurrency != 1 || options.ClaimLimit != 1 {
		t.Fatalf(
			"reward relay ordering policy = concurrency %d, claim %d; want 1, 1",
			options.Concurrency,
			options.ClaimLimit,
		)
	}
	if options.MaxAttempts != config.MaxAttempts ||
		options.MaxMessageBytes != config.MaxMessageBytes {
		t.Fatalf("reward relay did not preserve bounded outbox policy: %+v", options)
	}
}

func TestRewardDeliveryCompositionIsInertUntilRun(t *testing.T) {
	root := t.TempDir()
	forgeConfig := settings.Forge{
		DataDir:           filepath.Join(root, "forge"),
		Topic:             "solve-reward-transitions",
		ConsumerGroup:     "solve-reward-projector",
		MaxMessageBytes:   512 << 10,
		MaxSegmentBytes:   64 << 20,
		MaxStorageBytes:   512 << 20,
		RetentionBytes:    384 << 20,
		RetentionTime:     24 * time.Hour,
		RetentionInterval: time.Minute,
	}
	database := ent.WrapClient(generate.NewClient(), zap.NewNop())
	runtime, err := newRewardDelivery(
		forgeConfig,
		validRewardOutboxConfig(),
		database,
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("newRewardDelivery() error = %v", err)
	}
	if _, err := os.Stat(forgeConfig.DataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("composition touched Forge data directory: %v", err)
	}
	if runtime.started || runtime.broker != nil || runtime.relay != nil ||
		runtime.worker != nil {
		t.Fatal("reward delivery initialized owned resources before Run")
	}
	if err := runtime.Check(context.Background()); !errors.Is(
		err,
		ErrRewardDeliveryUnavailable,
	) {
		t.Fatalf("Check() before Run = %v, want unavailable", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() before Run error = %v", err)
	}
}

func validRewardOutboxConfig() settings.Outbox {
	return settings.Outbox{
		Name:                "integration-outbox",
		OwnerPrefix:         "judgify-test",
		Concurrency:         4,
		ClaimLimit:          4,
		MaxMessageBytes:     256 << 10,
		MaxAttempts:         20,
		LeaseDuration:       30 * time.Second,
		PublishTimeout:      5 * time.Second,
		StoreTimeout:        3 * time.Second,
		FinalizeTimeout:     3 * time.Second,
		PollInterval:        250 * time.Millisecond,
		RetryInitial:        250 * time.Millisecond,
		RetryMaximum:        30 * time.Second,
		RetryMultiplier:     2,
		ReconcileInterval:   30 * time.Second,
		ReconcileBatch:      256,
		MaxReconcileBatches: 32,
	}
}
