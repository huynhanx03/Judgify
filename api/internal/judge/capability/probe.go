package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	probeCommandTimeout = 15 * time.Second
	maximumProbeOutput  = 64 << 10
)

type ProbeCheck string

const (
	ProbeDockerSecurity    ProbeCheck = "docker_security"
	ProbeCgroupMembership  ProbeCheck = "cgroup_membership"
	ProbeCgroupControllers ProbeCheck = "cgroup_controllers"
	ProbeCgroupDelegation  ProbeCheck = "cgroup_delegation"
	ProbeImageDigest       ProbeCheck = "image_digest"
	ProbeSandboxSemantics  ProbeCheck = "sandbox_semantics"
	ProbeRuntimeManifest   ProbeCheck = "runtime_manifest"
	ProbeCheckerSmoke      ProbeCheck = "checker_smoke"
)

type HostCommand struct {
	Check      ProbeCheck
	Executable string
	Arguments  []string
}

type HostCommandResult struct {
	Stdout []byte
}

// CommandHost is the only host boundary used by the probe. Production uses
// the fixed allowlisted argv executor below; unit tests inject a deterministic
// implementation and never access Docker or host cgroup state.
type CommandHost interface {
	Run(ctx context.Context, command HostCommand) (HostCommandResult, error)
}

type Probe struct {
	host  CommandHost
	clock func() time.Time
}

func NewProbe(host CommandHost, clock func() time.Time) (*Probe, error) {
	if host == nil || clock == nil {
		return nil, ErrInvalidCapability
	}
	return &Probe{host: host, clock: clock}, nil
}

// NewProductionProbe returns a fail-closed probe whose Docker CLI and nested
// inspector invocations are bound to the same Unix socket as the sandbox
// supervisor. It never inherits a Docker context or endpoint from ambient
// process configuration.
func NewProductionProbe(socketPath string) (*Probe, error) {
	if !filepath.IsAbs(socketPath) || filepath.Clean(socketPath) != socketPath ||
		strings.ContainsAny(socketPath, "\x00\r\n") {
		return nil, ErrInvalidCapability
	}
	return NewProbe(execCommandHost{dockerHost: "unix://" + socketPath}, time.Now)
}

func (probe *Probe) Probe(
	ctx context.Context,
	tuple CapabilityTuple,
	bootID uuid.UUID,
) (CapabilityEvidence, error) {
	probedAt := time.Time{}
	if probe != nil && probe.clock != nil {
		probedAt = probe.clock().UTC()
	}
	evidence := CapabilityEvidence{Tuple: tuple, BootID: bootID, ProbedAt: probedAt, Passing: false}
	if probe == nil || probe.host == nil || ctx == nil || ctx.Err() != nil ||
		probedAt.IsZero() || bootID == uuid.Nil || validateCapabilityTuple(tuple) != nil {
		return evidence, ErrInvalidCapability
	}

	dockerSecurity, err := probe.run(ctx, HostCommand{
		Check: ProbeDockerSecurity, Executable: "/usr/local/bin/docker",
		Arguments: []string{"info", "--format", "{{json .SecurityOptions}}"},
	})
	if err != nil || !validDockerSecurity(dockerSecurity.Stdout) {
		return evidence, probeFailure(ProbeDockerSecurity, err)
	}

	membership, err := probe.run(ctx, HostCommand{
		Check: ProbeCgroupMembership, Executable: "/usr/bin/cat",
		Arguments: []string{"/proc/self/cgroup"},
	})
	cgroupDirectory, validMembership := delegatedCgroupV2Directory(membership.Stdout)
	if err != nil || !validMembership {
		return evidence, probeFailure(ProbeCgroupMembership, err)
	}

	controllers, err := probe.run(ctx, HostCommand{
		Check: ProbeCgroupControllers, Executable: "/usr/bin/cat",
		Arguments: []string{path.Join(cgroupDirectory, "cgroup.controllers")},
	})
	if err != nil || !hasCgroupControllers(controllers.Stdout) {
		return evidence, probeFailure(ProbeCgroupControllers, err)
	}
	if _, err := probe.run(ctx, HostCommand{
		Check: ProbeCgroupDelegation, Executable: "/usr/bin/test",
		Arguments: []string{"-w", path.Join(cgroupDirectory, "cgroup.subtree_control")},
	}); err != nil {
		return evidence, probeFailure(ProbeCgroupDelegation, err)
	}

	image, err := probe.run(ctx, HostCommand{
		Check: ProbeImageDigest, Executable: "/usr/local/bin/docker",
		Arguments: []string{"image", "inspect", "--format", "{{json .RepoDigests}}", tuple.ImageDigest},
	})
	if err != nil || !containsExactDigest(image.Stdout, tuple.ImageDigest) {
		return evidence, probeFailure(ProbeImageDigest, err)
	}

	sandbox, err := probe.run(ctx, HostCommand{
		Check: ProbeSandboxSemantics, Executable: "/usr/local/bin/judgify-capability-inspector",
		Arguments: []string{
			"sandbox", "--image", tuple.ImageDigest,
			"--network=none",
			"--read-only",
			"--user=65532:65532",
			"--cap-drop=ALL",
			"--security-opt=no-new-privileges",
			"--security-opt=seccomp=builtin",
			"--boot-id", bootID.String(),
		},
	})
	var sandboxReport sandboxSemanticsReport
	if err != nil || decodeStrictJSON(sandbox.Stdout, &sandboxReport) != nil || !sandboxReport.complete() {
		return evidence, probeFailure(ProbeSandboxSemantics, err)
	}

	manifest, err := probe.run(ctx, HostCommand{
		Check: ProbeRuntimeManifest, Executable: "/usr/local/bin/judgify-capability-inspector",
		Arguments: []string{
			"runtime-manifest", "--image", tuple.ImageDigest,
			"--runtime", tuple.RuntimeKey,
			"--profile-version", tuple.RuntimeProfileVersion,
			"--compiler-version", tuple.CompilerVersion,
			"--runtime-bootstrap-version", tuple.RuntimeBootstrapVersion,
			"--compile-command-id", tuple.CompileCommandID,
			"--run-command-id", tuple.RunCommandID,
			"--manifest-version", tuple.ManifestVersion,
		},
	})
	if err != nil || strings.TrimSpace(string(manifest.Stdout)) != tuple.ManifestChecksum {
		return evidence, probeFailure(ProbeRuntimeManifest, err)
	}

	checker, err := probe.run(ctx, HostCommand{
		Check: ProbeCheckerSmoke, Executable: "/usr/local/bin/judgify-capability-inspector",
		Arguments: []string{
			"checker-smoke", "--checker", tuple.CheckerKey,
			"--version", tuple.CheckerVersion,
			"--supervisor-protocol", tuple.SupervisorProtocol,
		},
	})
	var checkerReport checkerSmokeReport
	if err != nil || decodeStrictJSON(checker.Stdout, &checkerReport) != nil ||
		!checkerReport.Passing ||
		checkerReport.CheckerImplementationChecksum != tuple.CheckerImplementationChecksum ||
		!capabilityDigestPattern.MatchString(checkerReport.SmokeChecksum) ||
		!capabilityDigestPattern.MatchString(checkerReport.GoldenChecksum) {
		return evidence, probeFailure(ProbeCheckerSmoke, err)
	}

	evidence.CheckerImplementationChecksum = checkerReport.CheckerImplementationChecksum
	evidence.SmokeChecksum = checkerReport.SmokeChecksum
	evidence.GoldenChecksum = checkerReport.GoldenChecksum
	evidence.Passing = true
	return evidence, nil
}

func (probe *Probe) run(ctx context.Context, command HostCommand) (HostCommandResult, error) {
	commandCtx, cancel := context.WithTimeout(ctx, probeCommandTimeout)
	defer cancel()
	result, err := probe.host.Run(commandCtx, command)
	if err != nil {
		return HostCommandResult{}, err
	}
	if len(result.Stdout) > maximumProbeOutput {
		return HostCommandResult{}, ErrCapabilityDenied
	}
	return result, nil
}

type sandboxSemanticsReport struct {
	NonRootUser         bool `json:"non_root_user"`
	UserNamespace       bool `json:"user_namespace"`
	ReadOnlyRoot        bool `json:"read_only_root"`
	NetworkDisabled     bool `json:"network_disabled"`
	NoNewPrivileges     bool `json:"no_new_privileges"`
	Seccomp             bool `json:"seccomp"`
	CapabilitiesDropped bool `json:"capabilities_dropped"`
	KillWait            bool `json:"kill_wait"`
	CPUAccounting       bool `json:"cpu_accounting"`
	MemoryAccounting    bool `json:"memory_accounting"`
	PIDEnforcement      bool `json:"pid_enforcement"`
	OOMAccounting       bool `json:"oom_accounting"`
	ArchiveCopy         bool `json:"archive_copy"`
	Removed             bool `json:"removed"`
}

func (report sandboxSemanticsReport) complete() bool {
	return report.NonRootUser && report.UserNamespace && report.ReadOnlyRoot &&
		report.NetworkDisabled && report.NoNewPrivileges && report.Seccomp &&
		report.CapabilitiesDropped && report.KillWait && report.CPUAccounting &&
		report.MemoryAccounting && report.PIDEnforcement && report.OOMAccounting &&
		report.ArchiveCopy && report.Removed
}

type checkerSmokeReport struct {
	Passing                       bool   `json:"passing"`
	CheckerImplementationChecksum string `json:"checker_implementation_checksum"`
	SmokeChecksum                 string `json:"smoke_checksum"`
	GoldenChecksum                string `json:"golden_checksum"`
}

func validDockerSecurity(data []byte) bool {
	var options []string
	if decodeStrictJSON(data, &options) != nil {
		return false
	}
	rootless := false
	seccomp := false
	for _, option := range options {
		rootless = rootless || option == "name=rootless"
		seccomp = seccomp || strings.HasPrefix(option, "name=seccomp,")
	}
	return rootless && seccomp
}

func hasCgroupControllers(data []byte) bool {
	controllers := strings.Fields(string(data))
	return slices.Contains(controllers, "cpu") && slices.Contains(controllers, "memory") &&
		slices.Contains(controllers, "pids")
}

func delegatedCgroupV2Directory(data []byte) (string, bool) {
	membership := strings.TrimSuffix(string(data), "\n")
	if membership == "" || strings.ContainsAny(membership, "\r\n\x00") {
		return "", false
	}
	parts := strings.Split(membership, ":")
	if len(parts) != 3 || parts[0] != "0" || parts[1] != "" {
		return "", false
	}
	relative := parts[2]
	if !path.IsAbs(relative) || path.Clean(relative) != relative || strings.Contains(relative, "\\") {
		return "", false
	}
	if relative == "/" {
		return "/sys/fs/cgroup", true
	}
	return path.Join("/sys/fs/cgroup", relative), true
}

func containsExactDigest(data []byte, digest string) bool {
	var digests []string
	if decodeStrictJSON(data, &digests) != nil || len(digests) == 0 ||
		!capabilityDigestPattern.MatchString(digest) {
		return false
	}
	found := false
	for _, repositoryDigest := range digests {
		repository, actualDigest, separated := strings.Cut(repositoryDigest, "@")
		if !separated || repository == "" || strings.Contains(repository, "@") ||
			strings.TrimSpace(repository) != repository ||
			!capabilityDigestPattern.MatchString(actualDigest) {
			return false
		}
		found = found || actualDigest == digest
	}
	return found
}

func decodeStrictJSON(data []byte, value any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrCapabilityDenied
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrCapabilityDenied
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrCapabilityDenied
			}
			if _, duplicate := seen[key]; duplicate {
				return ErrCapabilityDenied
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return ErrCapabilityDenied
		}
		return nil
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return ErrCapabilityDenied
		}
		return nil
	default:
		return ErrCapabilityDenied
	}
}

func probeFailure(check ProbeCheck, cause error) error {
	if cause == nil {
		cause = ErrCapabilityDenied
	}
	return fmt.Errorf("%w: %s: %v", ErrCapabilityDenied, check, cause)
}
