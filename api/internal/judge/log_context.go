package judge

import (
	"context"

	"github.com/huynhanx03/judgify/internal/platform/correlation"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
)

// leaseLogContext keeps only the durable correlation identity ambient. Job and
// attempt identifiers belong to bounded metadata on specific log events.
func leaseLogContext(
	ctx context.Context,
	lease submissionports.JobLease,
) context.Context {
	return correlation.WithContext(ctx, lease.Job.OriginCID)
}
