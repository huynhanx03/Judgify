package judge

import (
	"context"
	"strconv"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
)

// leaseLogContext establishes the durable execution identity once so every
// repository, compiler, sandbox, and finalization log inherits the same
// correlation fields. Lease tokens and ownership capabilities must never be
// attached to ambient logs.
func leaseLogContext(
	ctx context.Context,
	lease submissionports.JobLease,
) context.Context {
	ctx = correlation.WithContext(ctx, lease.Job.OriginCID)
	return commonlogger.WithFields(
		ctx,
		commonlogger.String("job_id", lease.Job.ID.String()),
		commonlogger.String("submission_id", lease.Job.SubmissionID.String()),
		commonlogger.String("judge_spec_id", lease.Job.JudgeSpecID.String()),
		commonlogger.String("attempt_id", lease.Attempt.ID.String()),
		commonlogger.String("generation", strconv.FormatInt(lease.Job.Generation, 10)),
		commonlogger.String("attempt", strconv.FormatUint(uint64(lease.Attempt.AttemptNumber), 10)),
	)
}
