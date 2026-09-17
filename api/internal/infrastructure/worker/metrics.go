package worker

import (
	"context"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"

	"github.com/huynhanx03/judgify/internal/platform/observability"
	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
)

// judgeMetricsObserver is the infrastructure adapter from domain transition
// callbacks to Judgify's fixed metric catalog. It never receives identifiers.
type judgeMetricsObserver struct {
	metrics *observability.Metrics
}

func (observer judgeMetricsObserver) RecordLeaseEvent(
	ctx context.Context,
	outcome submissionservice.LeaseMetricOutcome,
	count int,
) {
	if observer.metrics == nil || count <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	} else if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	_ = observer.metrics.Add(ctx, observability.MetricJudgeLeaseEvents, float64(count), commonmetrics.Labels{
		"outcome": string(outcome),
	})
}

func (observer judgeMetricsObserver) RecordFinalization(
	ctx context.Context,
	outcome submissionservice.FinalizationMetricOutcome,
) {
	if observer.metrics == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	} else if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	_ = observer.metrics.Inc(ctx, observability.MetricJudgeFinalizations, commonmetrics.Labels{
		"outcome": string(outcome),
	})
}

var _ submissionservice.JobMetricsObserver = judgeMetricsObserver{}
var _ submissionservice.FinalizationMetricsObserver = judgeMetricsObserver{}
