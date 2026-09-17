package infrastructure

import (
	"errors"
	"slices"
	"testing"
	"time"

	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestNormalizeEventDeliveryConfigProducesBoundedProductionPolicy(t *testing.T) {
	t.Parallel()

	config := normalizeEventDeliveryConfig(settings.Outbox{})
	if err := validateEventDeliveryConfig(config); err != nil {
		t.Fatalf("validate defaults: %v", err)
	}
	destinations := []string{"contest.completed.v1", "material.view-recorded.v1"}
	options := eventDeliveryRelayOptions(config, destinations)
	if !slices.Equal(options.Destinations, destinations) {
		t.Fatalf("relay destination allowlist = %v, want %v", options.Destinations, destinations)
	}
	if slices.Contains(
		options.Destinations,
		submissioncontract.EventProblemSolveStatusChangedV1,
	) {
		t.Fatalf("API relay claims worker-owned solve destination: %v", options.Destinations)
	}
	if options.Concurrency != config.Concurrency ||
		options.ClaimLimit != config.ClaimLimit ||
		options.MaxAttempts != config.MaxAttempts {
		t.Fatalf("relay options do not preserve config: %+v", options)
	}
}

func TestValidateEventDeliveryConfigRejectsUnsafeRetryAndClaimPolicy(t *testing.T) {
	t.Parallel()

	valid := normalizeEventDeliveryConfig(settings.Outbox{})
	cases := []settings.Outbox{
		func() settings.Outbox {
			value := valid
			value.ClaimLimit = 101
			return value
		}(),
		func() settings.Outbox {
			value := valid
			value.RetryMaximum = value.RetryInitial - time.Nanosecond
			return value
		}(),
		func() settings.Outbox {
			value := valid
			value.RetryMultiplier = 1
			return value
		}(),
		func() settings.Outbox {
			value := valid
			value.ReconcileBatch = 4097
			return value
		}(),
	}
	for _, config := range cases {
		if !errors.Is(validateEventDeliveryConfig(config), ErrInvalidEventDelivery) {
			t.Fatalf("unsafe config was accepted: %+v", config)
		}
	}
}
