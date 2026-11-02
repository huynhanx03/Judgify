package configuration

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestAPIJudgeCatalogModeIsEnvironmentBound(t *testing.T) {
	config := validJudgeConfiguration()
	config.APIAuthoringCatalogMode = settings.JudgeAPICatalogDevelopment
	config.ReleaseManifestPath = ""
	if err := validateAPIJudge(config, false); err != nil {
		t.Fatalf("development API judge config error = %v", err)
	}
	if err := validateAPIJudge(config, true); err == nil {
		t.Fatal("production accepted the development authoring catalog")
	}

	config.APIAuthoringCatalogMode = settings.JudgeAPICatalogApprovedRelease
	if err := validateAPIJudge(config, false); err == nil {
		t.Fatal("approved-release mode accepted an empty manifest path")
	}
	config.ReleaseManifestPath = "/run/judgify/release-manifest.json"
	if err := validateAPIJudge(config, true); err != nil {
		t.Fatalf("approved production API judge config error = %v", err)
	}
}

func TestForgeConfigurationIsWorkerOnlyAndProductionPathIsAbsolute(t *testing.T) {
	config := validForgeConfiguration(t)
	if err := validateForge(config, true); err != nil {
		t.Fatalf("valid production Forge config error = %v", err)
	}

	config.DataDir = "relative/forge"
	if err := validateForge(config, true); err == nil {
		t.Fatal("production accepted a relative Forge data directory")
	}
	if err := validateForge(config, false); err != nil {
		t.Fatalf("development rejected a clean relative Forge directory: %v", err)
	}

	config = validForgeConfiguration(t)
	config.MaxStorageBytes = config.MaxSegmentBytes - 1
	if err := validateForge(config, true); err == nil {
		t.Fatal("Forge accepted storage smaller than one segment")
	}
}

func TestForgeMessageLimitIncludesOutboxEnvelopeHeadroom(t *testing.T) {
	forgeConfig := validForgeConfiguration(t)
	outbox := settings.Outbox{MaxMessageBytes: forgeConfig.MaxMessageBytes}
	if err := validateForgeOutboxBoundary(forgeConfig, outbox); err == nil {
		t.Fatal("Forge accepted no envelope headroom above outbox payload policy")
	}
	outbox.MaxMessageBytes = 256 << 10
	if err := validateForgeOutboxBoundary(forgeConfig, outbox); err != nil {
		t.Fatalf("valid Forge/outbox boundary error = %v", err)
	}
}

func TestJudgeWorkerAlwaysRequiresApprovedExecutableInputs(t *testing.T) {
	config := validJudgeConfiguration()
	config.APIAuthoringCatalogMode = settings.JudgeAPICatalogDevelopment
	if err := validateJudgeWorker(config); err != nil {
		t.Fatalf("valid worker judge config error = %v", err)
	}
	config.ReleaseManifestPath = ""
	if err := validateJudgeWorker(config); err == nil {
		t.Fatal("worker accepted an empty approved manifest path")
	}
	config = validJudgeConfiguration()
	config.DockerImage = "judgify/sandbox:development"
	if err := validateJudgeWorker(config); err == nil {
		t.Fatal("worker accepted a mutable sandbox image reference")
	}
}

func validJudgeConfiguration() settings.Judge {
	return settings.Judge{
		ReleaseID:                       "judge-release.test",
		APIAuthoringCatalogMode:         settings.JudgeAPICatalogApprovedRelease,
		ReleaseManifestPath:             "/run/judgify/release-manifest.json",
		DockerSocketPath:                "/run/docker.sock",
		StagingRoot:                     "/var/lib/judgify/staging",
		MaximumActiveSubmissionsPerUser: 8,
		MaximumReadyBacklog:             10_000,
		MaximumOldestReadyAge:           30 * time.Minute,
		MinimumFreeDiskBytes:            1,
		AdmissionDiskPath:               "/var/lib/judgify",
		WorkerHeartbeatFreshness:        30 * time.Second,
		DockerImage:                     "registry.invalid/judgify/sandbox@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PoolSize:                        3,
		WorkerCount:                     3,
		CompileTimeoutMs:                10_000,
		MaximumTestInputBytes:           1 << 20,
		MaximumOutputBytes:              1 << 20,
		MaximumDiagnosticBytes:          16 << 10,
		NetworkDisabled:                 true,
		ContainerMemoryMb:               1024,
		SandboxTmpfsSizeMb:              64,
		WorkerIdentityPrefix:            "judgify-judge",
		LeaseDuration:                   30 * time.Second,
		LeaseHeartbeat:                  5 * time.Second,
		LeaseRetryFloor:                 100 * time.Millisecond,
		MaximumAttempts:                 5,
	}
}

func validForgeConfiguration(t *testing.T) settings.Forge {
	t.Helper()
	return settings.Forge{
		DataDir:           filepath.Join(t.TempDir(), "forge"),
		Topic:             "solve-reward-transitions",
		ConsumerGroup:     "solve-reward-projector",
		MaxMessageBytes:   512 << 10,
		MaxSegmentBytes:   64 << 20,
		MaxStorageBytes:   512 << 20,
		RetentionBytes:    384 << 20,
		RetentionTime:     24 * time.Hour,
		RetentionInterval: time.Minute,
	}
}
