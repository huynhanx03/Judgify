package operations

import (
	"time"

	"github.com/google/uuid"
)

const (
	MaximumWorkers     = 100
	MaximumActivations = 100
)

// QueueSnapshot is a bounded operational projection. It deliberately exposes
// aggregate state only; lease owners, tokens, contestant source, test assets,
// and diagnostics never cross this boundary.
type QueueSnapshot struct {
	Ready         int64      `json:"ready"`
	Leased        int64      `json:"leased"`
	RetryWait     int64      `json:"retry_wait"`
	Done          int64      `json:"done"`
	Cancelled     int64      `json:"cancelled"`
	Dead          int64      `json:"dead"`
	OldestReadyAt *time.Time `json:"oldest_ready_at,omitempty"`
}

type WorkerSnapshot struct {
	ID             uuid.UUID  `json:"id"`
	WorkerKey      string     `json:"worker_key"`
	BootID         uuid.UUID  `json:"boot_id"`
	BootStartedAt  time.Time  `json:"boot_started_at"`
	HeartbeatAt    time.Time  `json:"heartbeat_at"`
	ClaimsEnabled  bool       `json:"claims_enabled"`
	TotalCapacity  int        `json:"total_capacity"`
	FreeCapacity   int        `json:"free_capacity"`
	EvidenceCount  int        `json:"evidence_count"`
	LatestProbedAt *time.Time `json:"latest_probed_at,omitempty"`
}

type ActivationSnapshot struct {
	ID                            uuid.UUID  `json:"id"`
	ReleaseID                     string     `json:"release_id"`
	ApprovedManifestChecksum      string     `json:"approved_manifest_checksum"`
	RuntimeKey                    string     `json:"runtime_key"`
	RuntimeProfileVersion         string     `json:"runtime_profile_version"`
	CheckerKey                    string     `json:"checker_key"`
	CheckerVersion                string     `json:"checker_version"`
	CheckerImplementationChecksum string     `json:"checker_implementation_checksum"`
	CapabilityProfileHash         string     `json:"capability_profile_hash"`
	State                         string     `json:"state"`
	ActivatedAt                   time.Time  `json:"activated_at"`
	RetiredAt                     *time.Time `json:"retired_at,omitempty"`
}

type Overview struct {
	GeneratedAt                time.Time            `json:"generated_at"`
	WorkerHeartbeatFreshnessMS int64                `json:"worker_heartbeat_freshness_ms"`
	Queue                      QueueSnapshot        `json:"queue"`
	Workers                    []WorkerSnapshot     `json:"workers"`
	Activations                []ActivationSnapshot `json:"activations"`
}
