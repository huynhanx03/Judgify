package di

import (
	"context"
	"errors"
	"testing"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
)

func TestUnavailableCapabilityResolverFailsClosedAndPreservesCancellation(t *testing.T) {
	resolver := unavailableCapabilityResolver{}
	if _, err := resolver.Resolve(
		context.Background(),
		intake.CapabilityRequest{},
	); !errors.Is(err, intake.ErrCapabilityUnavailable) {
		t.Fatalf("Resolve() error = %v, want capability unavailable", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.Resolve(
		ctx,
		intake.CapabilityRequest{},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("Resolve(cancelled) error = %v, want context canceled", err)
	}
}
