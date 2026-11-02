package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestNewInstallsTracerProviderWithoutCollectorDependency(t *testing.T) {
	provider, err := New(context.Background(), "judgify-api", "release-a1b2c3d", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "request")
	if !span.SpanContext().IsValid() || ctx == nil {
		t.Fatalf("span context is invalid: %#v", span.SpanContext())
	}
	span.End()
	carrier := propagation.MapCarrier{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}
	remote := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
	remoteSpan := trace.SpanContextFromContext(remote)
	if !remoteSpan.IsValid() || !remoteSpan.IsRemote() || remoteSpan.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("W3C trace context was not extracted: %#v", remoteSpan)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
