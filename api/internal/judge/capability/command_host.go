package capability

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// execCommandHost is deliberately package-private. It executes only the
// exact command shapes emitted by Probe; callers cannot turn it into a generic
// process runner by reusing an allowed executable with arbitrary arguments.
type commandExecutor func(
	ctx context.Context,
	executable string,
	arguments []string,
	stdout io.Writer,
	stderr io.Writer,
) error

type execCommandHost struct {
	execute    commandExecutor
	dockerHost string
}

var probeExecutables = map[ProbeCheck]string{
	ProbeDockerSecurity:    "/usr/local/bin/docker",
	ProbeCgroupMembership:  "/usr/bin/cat",
	ProbeCgroupControllers: "/usr/bin/cat",
	ProbeCgroupDelegation:  "/usr/bin/test",
	ProbeImageDigest:       "/usr/local/bin/docker",
	ProbeSandboxSemantics:  "/usr/local/bin/judgify-capability-inspector",
	ProbeRuntimeManifest:   "/usr/local/bin/judgify-capability-inspector",
	ProbeCheckerSmoke:      "/usr/local/bin/judgify-capability-inspector",
}

func (host execCommandHost) Run(ctx context.Context, command HostCommand) (HostCommandResult, error) {
	if ctx == nil || !allowedHostCommand(command) {
		return HostCommandResult{}, ErrCapabilityDenied
	}
	stdout := newBoundedBuffer(maximumProbeOutput)
	stderr := newBoundedBuffer(maximumProbeOutput)
	execute := hostCommandExecutor
	if host.execute != nil {
		execute = host.execute
	} else if host.dockerHost != "" {
		execute = dockerSocketCommandExecutor(host.dockerHost)
	}
	if err := execute(ctx, command.Executable, command.Arguments, stdout, stderr); err != nil {
		return HostCommandResult{}, fmt.Errorf("probe command %s failed: %w", command.Check, err)
	}
	return HostCommandResult{Stdout: append([]byte(nil), stdout.Bytes()...)}, nil
}

func dockerSocketCommandExecutor(dockerHost string) commandExecutor {
	return func(
		ctx context.Context,
		executable string,
		arguments []string,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		execution := exec.CommandContext(ctx, executable, arguments...)
		execution.Env = dockerSocketEnvironment(dockerHost)
		execution.Stdout = stdout
		execution.Stderr = stderr
		return execution.Run()
	}
}

// dockerSocketEnvironment removes every Docker endpoint/context override
// before pinning the only permitted Unix endpoint. The inspector inherits this
// environment, so its own fixed Docker commands cannot select another daemon.
func dockerSocketEnvironment(dockerHost string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		key, _, found := strings.Cut(value, "=")
		if !found {
			continue
		}
		switch key {
		case "DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH":
			continue
		}
		environment = append(environment, value)
	}
	return append(environment, "DOCKER_HOST="+dockerHost)
}

func hostCommandExecutor(
	ctx context.Context,
	executable string,
	arguments []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	execution := exec.CommandContext(ctx, executable, arguments...)
	execution.Stdout = stdout
	execution.Stderr = stderr
	return execution.Run()
}

func allowedHostCommand(command HostCommand) bool {
	if probeExecutables[command.Check] != command.Executable {
		return false
	}
	switch command.Check {
	case ProbeDockerSecurity:
		return slices.Equal(command.Arguments, []string{"info", "--format", "{{json .SecurityOptions}}"})
	case ProbeCgroupMembership:
		return slices.Equal(command.Arguments, []string{"/proc/self/cgroup"})
	case ProbeCgroupControllers:
		return len(command.Arguments) == 1 &&
			allowedDelegatedCgroupFile(command.Arguments[0], "cgroup.controllers")
	case ProbeCgroupDelegation:
		return len(command.Arguments) == 2 && command.Arguments[0] == "-w" &&
			allowedDelegatedCgroupFile(command.Arguments[1], "cgroup.subtree_control")
	case ProbeImageDigest:
		return len(command.Arguments) == 5 &&
			slices.Equal(command.Arguments[:4], []string{"image", "inspect", "--format", "{{json .RepoDigests}}"}) &&
			capabilityDigestPattern.MatchString(command.Arguments[4])
	case ProbeSandboxSemantics:
		return allowedSandboxArguments(command.Arguments)
	case ProbeRuntimeManifest:
		return len(command.Arguments) == 17 && command.Arguments[0] == "runtime-manifest" &&
			command.Arguments[1] == "--image" && capabilityDigestPattern.MatchString(command.Arguments[2]) &&
			command.Arguments[3] == "--runtime" && capabilityIdentifierPattern.MatchString(command.Arguments[4]) &&
			command.Arguments[5] == "--profile-version" && capabilityIdentifierPattern.MatchString(command.Arguments[6]) &&
			command.Arguments[7] == "--compiler-version" && capabilityIdentifierPattern.MatchString(command.Arguments[8]) &&
			command.Arguments[9] == "--runtime-bootstrap-version" && capabilityIdentifierPattern.MatchString(command.Arguments[10]) &&
			command.Arguments[11] == "--compile-command-id" && capabilityIdentifierPattern.MatchString(command.Arguments[12]) &&
			command.Arguments[13] == "--run-command-id" && capabilityIdentifierPattern.MatchString(command.Arguments[14]) &&
			command.Arguments[15] == "--manifest-version" && capabilityIdentifierPattern.MatchString(command.Arguments[16])
	case ProbeCheckerSmoke:
		return len(command.Arguments) == 7 && command.Arguments[0] == "checker-smoke" &&
			command.Arguments[1] == "--checker" && capabilityIdentifierPattern.MatchString(command.Arguments[2]) &&
			command.Arguments[3] == "--version" && capabilityIdentifierPattern.MatchString(command.Arguments[4]) &&
			command.Arguments[5] == "--supervisor-protocol" && capabilityIdentifierPattern.MatchString(command.Arguments[6])
	default:
		return false
	}
}

func allowedDelegatedCgroupFile(value string, name string) bool {
	if path.Clean(value) != value || path.Base(value) != name || strings.Contains(value, "\\") {
		return false
	}
	directory := path.Dir(value)
	return directory == "/sys/fs/cgroup" || strings.HasPrefix(directory, "/sys/fs/cgroup/")
}

func allowedSandboxArguments(arguments []string) bool {
	if len(arguments) != 11 || arguments[0] != "sandbox" || arguments[1] != "--image" ||
		!capabilityDigestPattern.MatchString(arguments[2]) ||
		!slices.Equal(arguments[3:11], []string{
			"--network=none",
			"--read-only",
			"--user=65532:65532",
			"--cap-drop=ALL",
			"--security-opt=no-new-privileges",
			"--security-opt=seccomp=builtin",
			"--boot-id",
			arguments[10],
		}) {
		return false
	}
	bootID, err := uuid.Parse(arguments[10])
	return err == nil && bootID != uuid.Nil
}

type boundedBuffer struct {
	limit int
	data  bytes.Buffer
}

func newBoundedBuffer(limit int) *boundedBuffer {
	return &boundedBuffer{limit: limit}
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	remaining := buffer.limit - buffer.data.Len()
	if remaining <= 0 || len(data) > remaining {
		return 0, ErrCapabilityDenied
	}
	return buffer.data.Write(data)
}

func (buffer *boundedBuffer) Bytes() []byte {
	return buffer.data.Bytes()
}
