package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Provider struct {
	provider *sdktrace.TracerProvider
}

func New(ctx context.Context, service, version, endpoint string) (*Provider, error) {
	if ctx == nil || strings.TrimSpace(service) == "" {
		return nil, fmt.Errorf("tracing: invalid configuration")
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(service),
		semconv.ServiceVersion(version),
	))
	if err != nil {
		return nil, fmt.Errorf("tracing: resource: %w", err)
	}
	options := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if strings.TrimSpace(endpoint) != "" {
		exporter, exportErr := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
		if exportErr != nil {
			return nil, fmt.Errorf("tracing: exporter: %w", exportErr)
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return &Provider{provider: provider}, nil
}

func (provider *Provider) Name() string              { return "opentelemetry-tracer-provider" }
func (provider *Provider) Run(context.Context) error { return nil }
func (provider *Provider) Shutdown(ctx context.Context) error {
	if provider == nil || provider.provider == nil {
		return nil
	}
	return provider.provider.Shutdown(ctx)
}

var _ lifecycle.Component = (*Provider)(nil)
