package infrastructure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type namedAPIComponent string

func (component namedAPIComponent) Name() string         { return string(component) }
func (namedAPIComponent) Run(context.Context) error      { return nil }
func (namedAPIComponent) Shutdown(context.Context) error { return nil }

func TestAPIComponentOrderStopsHTTPBeforeSessionWriter(t *testing.T) {
	components := orderedAPIComponents(
		namedAPIComponent("contest"),
		namedAPIComponent("scheduler"),
		namedAPIComponent("process-liveness"),
		namedAPIComponent("realtime-tickets"),
		namedAPIComponent("realtime-websocket-hub"),
		namedAPIComponent("realtime-authorization-tailer"),
		namedAPIComponent("event-delivery"),
		namedAPIComponent("operation-runner"),
		namedAPIComponent("security-email-dispatcher"),
		namedAPIComponent("receipt-janitor"),
		namedAPIComponent("authorization"),
		namedAPIComponent("session-authenticator"),
		namedAPIComponent("debug-window"),
		namedAPIComponent("private-probe-server"),
		namedAPIComponent("opentelemetry-tracer-provider"),
		namedAPIComponent("envoy-ext-authz-grpc-server"),
		namedAPIComponent("http-server"),
	)
	require.Len(t, components, 17)
	require.Equal(t, "process-liveness", components[2].Name())
	require.Equal(t, "realtime-tickets", components[3].Name())
	require.Equal(t, "realtime-websocket-hub", components[4].Name())
	require.Equal(t, "realtime-authorization-tailer", components[5].Name())
	require.Equal(t, "event-delivery", components[6].Name())
	require.Equal(t, "operation-runner", components[7].Name())
	require.Equal(t, "security-email-dispatcher", components[8].Name())
	require.Equal(t, "receipt-janitor", components[9].Name())
	require.Equal(t, "authorization", components[10].Name())
	require.Equal(t, "session-authenticator", components[11].Name())
	require.Equal(t, "debug-window", components[12].Name())
	require.Equal(t, "private-probe-server", components[13].Name())
	require.Equal(t, "opentelemetry-tracer-provider", components[14].Name())
	require.Equal(t, "envoy-ext-authz-grpc-server", components[15].Name())
	require.Equal(t, "http-server", components[16].Name())

	shutdownOrder := make([]string, 0, len(components))
	for index := len(components) - 1; index >= 0; index-- {
		shutdownOrder = append(shutdownOrder, components[index].Name())
	}
	require.Equal(t, []string{
		"http-server",
		"envoy-ext-authz-grpc-server",
		"opentelemetry-tracer-provider",
		"private-probe-server",
		"debug-window",
		"session-authenticator",
		"authorization",
		"receipt-janitor",
		"security-email-dispatcher",
		"operation-runner",
		"event-delivery",
		"realtime-authorization-tailer",
		"realtime-websocket-hub",
		"realtime-tickets",
		"process-liveness",
		"scheduler",
		"contest",
	}, shutdownOrder)
}
