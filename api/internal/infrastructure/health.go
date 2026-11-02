package infrastructure

import (
	"errors"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

	"github.com/huynhanx03/judgify/internal/platform/observability"
	"github.com/huynhanx03/judgify/internal/platform/processhealth"
)

var ErrInvalidAPIHealth = errors.New("api health: invalid dependencies")

const healthCheckTimeout = time.Second

// APIHealthDependencies are the bounded, secret-free readiness dependencies
// needed before an API instance can receive new requests.
type APIHealthDependencies struct {
	Database                    commonhealth.Checker
	EventDelivery               commonhealth.Checker
	Authorization               commonhealth.Checker
	RealtimeHub                 commonhealth.Checker
	RealtimeAuthorizationTailer commonhealth.Checker
	SecurityEmail               commonhealth.Checker
	ReceiptJanitor              commonhealth.Checker
	OperationRunner             commonhealth.Checker
	DebugWindow                 commonhealth.Checker
}

// NewAPIHealthRegistry retains the unobserved construction contract used by
// focused package tests. Production composition uses the observed variant.
func NewAPIHealthRegistry(
	dependencies APIHealthDependencies,
) (*commonhealth.Registry, *processhealth.Control, error) {
	return NewObservedAPIHealthRegistry(dependencies, nil)
}

// NewObservedAPIHealthRegistry builds the fixed API health contract and
// records only bounded checker names and outcomes.
func NewObservedAPIHealthRegistry(
	dependencies APIHealthDependencies,
	metrics *observability.Metrics,
) (*commonhealth.Registry, *processhealth.Control, error) {
	if dependencies.Database == nil || dependencies.EventDelivery == nil ||
		dependencies.Authorization == nil ||
		dependencies.RealtimeHub == nil ||
		dependencies.RealtimeAuthorizationTailer == nil ||
		dependencies.SecurityEmail == nil ||
		dependencies.ReceiptJanitor == nil ||
		dependencies.OperationRunner == nil ||
		dependencies.DebugWindow == nil {
		return nil, nil, ErrInvalidAPIHealth
	}
	critical := func(
		name string,
		checker commonhealth.Checker,
		errorCode string,
	) processhealth.Dependency {
		return processhealth.Dependency{
			Name: name, Checker: checker,
			Probes:   commonhealth.ProbeReadiness,
			Critical: true, Timeout: healthCheckTimeout,
			ErrorCode: errorCode,
		}
	}
	degraded := func(
		name string,
		checker commonhealth.Checker,
		errorCode string,
	) processhealth.Dependency {
		dependency := critical(name, checker, errorCode)
		dependency.Critical = false
		return dependency
	}
	registry, control, err := processhealth.New(
		processhealth.RoleAPI,
		[]processhealth.Dependency{
			critical("database", dependencies.Database, "database_unavailable"),
			critical("event-delivery", dependencies.EventDelivery, "event_delivery_unavailable"),
			critical("authorization", dependencies.Authorization, "authorization_revision_unhealthy"),
			critical("realtime-hub", dependencies.RealtimeHub, "realtime_hub_unavailable"),
			critical(
				"realtime-authorization-tailer",
				dependencies.RealtimeAuthorizationTailer,
				"realtime_authorization_tailer_unavailable",
			),
			degraded(
				"receipt-janitor",
				dependencies.ReceiptJanitor,
				"receipt_janitor_degraded",
			),
			degraded(
				"security-email-dispatcher",
				dependencies.SecurityEmail,
				"security_email_dispatcher_degraded",
			),
			degraded(
				"operation-runner",
				dependencies.OperationRunner,
				"operation_runner_degraded",
			),
			critical(
				"debug-window",
				dependencies.DebugWindow,
				"debug_window_revert_unhealthy",
			),
		},
		metrics,
	)
	if err != nil {
		return nil, nil, errors.Join(ErrInvalidAPIHealth, err)
	}
	return registry, control, nil
}
