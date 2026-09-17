package observability

import (
	"context"
	"errors"
	"sync"
	"time"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
	commonws "github.com/huynhanx03/go-common/pkg/common/websocket"

	"github.com/huynhanx03/judgify/internal/platform/eventdelivery"
)

type AuthorizationRevisionSource interface {
	DesiredRevision() int64
	AppliedRevision() int64
}

// RegisterAuthorizationCollector exports the process-local desired/applied
// fence without coupling the catalog to Casbin or a concrete runtime.
func (metrics *Metrics) RegisterAuthorizationCollector(source AuthorizationRevisionSource) error {
	if isNilInterface(source) {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("authorization-revision", CollectorFunc(func(ctx context.Context) error {
		return errors.Join(
			metrics.Set(ctx, MetricAuthorizationRevision, float64(source.DesiredRevision()), commonmetrics.Labels{"kind": "desired"}),
			metrics.Set(ctx, MetricAuthorizationRevision, float64(source.AppliedRevision()), commonmetrics.Labels{"kind": "applied"}),
		)
	}))
}

type ReplayMetricsSource interface {
	Metrics() eventdelivery.MetricsSnapshot
}

// RegisterReplayCollector reports time since the last successful repair. A
// runtime that has never reconciled accrues lag from its observation start, so
// a permanently failing startup cannot look healthy with a zero gauge.
func (metrics *Metrics) RegisterReplayCollector(source ReplayMetricsSource) error {
	if isNilInterface(source) {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("outbox-replay", CollectorFunc(func(ctx context.Context) error {
		snapshot := source.Metrics()
		reference := snapshot.LastSuccessfulReconcileAt
		if reference.IsZero() {
			reference = snapshot.ObservedSince
		}
		return metrics.Set(ctx, MetricOutboxReplayLag, ageSeconds(time.Now().UTC(), reference), nil)
	}))
}

type WebSocketMetricsSource interface {
	Metrics() commonws.MetricsSnapshot
}

type webSocketCollector struct {
	metrics  *Metrics
	source   WebSocketMetricsSource
	mu       sync.Mutex
	previous commonws.MetricsSnapshot
}

func (metrics *Metrics) RegisterWebSocketCollector(source WebSocketMetricsSource) error {
	if isNilInterface(source) {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("websocket-runtime", &webSocketCollector{
		metrics: metrics,
		source:  source,
	})
}

func (collector *webSocketCollector) Collect(ctx context.Context) error {
	if collector == nil || collector.metrics == nil || collector.source == nil || ctx == nil {
		return ErrInvalidMetrics
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	current := collector.source.Metrics()
	previous := &collector.previous
	var failures []error
	failures = appendMetricError(failures, collector.metrics.Set(
		ctx, MetricWebSocketConnections, float64(current.ActiveConnections), nil,
	))
	recordMessage := func(direction string, outcome string, delta float64, commit func()) {
		err := collector.metrics.Add(ctx, MetricWebSocketMessages, delta, commonmetrics.Labels{
			"direction": direction, "outcome": outcome,
		})
		failures = appendMetricError(failures, err)
		if err == nil && commit != nil {
			commit()
		}
	}
	recordMessage("inbound", "accepted", counterDeltaFloat(current.InboundAccepted, previous.InboundAccepted), func() {
		previous.InboundAccepted = current.InboundAccepted
	})
	// These closed catalog outcomes are intentionally materialized even though
	// the generic hub has no corresponding transition today.
	recordMessage("inbound", "delivered", 0, nil)
	recordMessage("inbound", "dropped", 0, nil)
	recordMessage("inbound", "rate_limited", counterDeltaFloat(current.InboundRateLimited, previous.InboundRateLimited), func() {
		previous.InboundRateLimited = current.InboundRateLimited
	})
	recordMessage("inbound", "invalid", counterDeltaFloat(current.InboundInvalid, previous.InboundInvalid), func() {
		previous.InboundInvalid = current.InboundInvalid
	})
	recordMessage("inbound", "error", counterDeltaFloat(current.InboundErrors, previous.InboundErrors), func() {
		previous.InboundErrors = current.InboundErrors
	})
	recordMessage(
		"outbound",
		"accepted",
		counterDeltaFloat(current.Enqueued, previous.Enqueued)+
			counterDeltaFloat(current.Coalesced, previous.Coalesced),
		func() {
			previous.Enqueued = current.Enqueued
			previous.Coalesced = current.Coalesced
		},
	)
	recordMessage("outbound", "delivered", counterDeltaFloat(current.OutboundDelivered, previous.OutboundDelivered), func() {
		previous.OutboundDelivered = current.OutboundDelivered
	})
	recordMessage(
		"outbound",
		"dropped",
		counterDeltaFloat(current.DroppedLatest, previous.DroppedLatest)+
			counterDeltaFloat(current.DroppedEphemeral, previous.DroppedEphemeral)+
			counterDeltaFloat(current.CriticalClosed, previous.CriticalClosed),
		func() {
			previous.DroppedLatest = current.DroppedLatest
			previous.DroppedEphemeral = current.DroppedEphemeral
			previous.CriticalClosed = current.CriticalClosed
		},
	)
	recordMessage("outbound", "rate_limited", 0, nil)
	recordMessage("outbound", "invalid", 0, nil)
	recordMessage("outbound", "error", counterDeltaFloat(current.OutboundErrors, previous.OutboundErrors), func() {
		previous.OutboundErrors = current.OutboundErrors
	})

	recordClose := func(class string, currentValue uint64, previousValue *uint64) {
		err := collector.metrics.Add(
			ctx,
			MetricWebSocketCloses,
			counterDeltaFloat(currentValue, *previousValue),
			commonmetrics.Labels{"class": class},
		)
		failures = appendMetricError(failures, err)
		if err == nil {
			*previousValue = currentValue
		}
	}
	recordClose("normal", current.Closes.Normal, &previous.Closes.Normal)
	recordClose("shutdown", current.Closes.Shutdown, &previous.Closes.Shutdown)
	recordClose("auth", current.Closes.Auth, &previous.Closes.Auth)
	recordClose("policy", current.Closes.Policy, &previous.Closes.Policy)
	recordClose("slow_client", current.Closes.SlowClient, &previous.Closes.SlowClient)
	recordClose("protocol", current.Closes.Protocol, &previous.Closes.Protocol)
	recordClose("internal", current.Closes.Internal, &previous.Closes.Internal)
	return errors.Join(failures...)
}

func counterDeltaFloat(current uint64, previous uint64) float64 {
	if current < previous {
		return float64(current)
	}
	return float64(current - previous)
}
