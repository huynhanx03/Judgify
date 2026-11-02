package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const traceparentLength = 55

// CaptureTraceparent retains only the sampled W3C span identity needed to
// relate a later durable attempt to the work that created it.
func CaptureTraceparent(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() || !span.IsSampled() {
		return ""
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	value := carrier.Get("traceparent")
	if len(value) != traceparentLength {
		return ""
	}
	return value
}

// OriginLink validates an optional persisted traceparent before using it as
// a link. A delayed job is a new trace, never a child of the old HTTP span.
func OriginLink(traceparent string) (trace.Link, bool) {
	if len(traceparent) != traceparentLength {
		return trace.Link{}, false
	}
	carrier := propagation.MapCarrier{"traceparent": traceparent}
	ctx := propagation.TraceContext{}.Extract(context.Background(), carrier)
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() || !span.IsSampled() {
		return trace.Link{}, false
	}
	return trace.Link{SpanContext: span}, true
}

// StartDurableAttempt starts a fresh worker trace and links the HTTP origin
// when it was sampled. A queued attempt does not inherit the request lifetime.
func StartDurableWork(ctx context.Context, name, originTraceparent string) (context.Context, trace.Span) {
	options := []trace.SpanStartOption{
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	if link, valid := OriginLink(originTraceparent); valid {
		options = append(options, trace.WithLinks(link))
	}
	return otel.Tracer("judgify/durable").Start(ctx, name, options...)
}
