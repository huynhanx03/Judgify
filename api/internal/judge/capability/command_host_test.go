package capability

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestAllowedHostCommandRejectsExecutableArgumentAndCgroupPathConfusion(t *testing.T) {
	digest := "sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac"
	bootID := "99999999-9999-4999-8999-999999999999"
	valid := []HostCommand{
		{Check: ProbeDockerSecurity, Executable: "/usr/local/bin/docker", Arguments: []string{"info", "--format", "{{json .SecurityOptions}}"}},
		{Check: ProbeCgroupMembership, Executable: "/usr/bin/cat", Arguments: []string{"/proc/self/cgroup"}},
		{Check: ProbeCgroupControllers, Executable: "/usr/bin/cat", Arguments: []string{"/sys/fs/cgroup/system.slice/judgify-worker.service/cgroup.controllers"}},
		{Check: ProbeCgroupDelegation, Executable: "/usr/bin/test", Arguments: []string{"-w", "/sys/fs/cgroup/system.slice/judgify-worker.service/cgroup.subtree_control"}},
		{Check: ProbeImageDigest, Executable: "/usr/local/bin/docker", Arguments: []string{"image", "inspect", "--format", "{{json .RepoDigests}}", digest}},
		{Check: ProbeSandboxSemantics, Executable: "/usr/local/bin/judgify-capability-inspector", Arguments: []string{"sandbox", "--image", digest, "--network=none", "--read-only", "--user=65532:65532", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--security-opt=seccomp=builtin", "--boot-id", bootID}},
		{Check: ProbeRuntimeManifest, Executable: "/usr/local/bin/judgify-capability-inspector", Arguments: []string{
			"runtime-manifest", "--image", digest, "--runtime", "cpp17",
			"--profile-version", "2026.07.1", "--compiler-version", "gcc-15.2.0",
			"--runtime-bootstrap-version", "alpine3.23-musl",
			"--compile-command-id", "cpp17.compile.v1", "--run-command-id", "cpp17.run.v1",
			"--manifest-version", "cpp17-artifact.v1",
		}},
		{Check: ProbeCheckerSmoke, Executable: "/usr/local/bin/judgify-capability-inspector", Arguments: []string{"checker-smoke", "--checker", "exact", "--version", "v1", "--supervisor-protocol", "sandbox-supervisor.v1"}},
	}
	for _, command := range valid {
		if !allowedHostCommand(command) {
			t.Fatalf("valid command rejected: %+v", command)
		}
	}

	invalid := []HostCommand{
		{Check: ProbeDockerSecurity, Executable: "/usr/bin/cat", Arguments: valid[0].Arguments},
		{Check: ProbeDockerSecurity, Executable: "/usr/local/bin/docker", Arguments: []string{"info", "--format", "{{json .SecurityOptions}}", "extra"}},
		{Check: ProbeCgroupMembership, Executable: "/usr/bin/cat", Arguments: []string{"/proc/1/cgroup"}},
		{Check: ProbeCgroupControllers, Executable: "/usr/bin/cat", Arguments: []string{"/sys/fs/cgroup/worker/../cgroup.controllers"}},
		{Check: ProbeCgroupControllers, Executable: "/usr/bin/cat", Arguments: []string{"/sys/fs/cgroup-other/cgroup.controllers"}},
		{Check: ProbeCgroupDelegation, Executable: "/usr/bin/test", Arguments: []string{"-w", "/sys/fs/cgroup/worker/cgroup.controllers"}},
		{Check: ProbeImageDigest, Executable: "/usr/local/bin/docker", Arguments: []string{"image", "inspect", "--format", "{{json .RepoDigests}}", "judgify/runtime:latest"}},
		{Check: ProbeSandboxSemantics, Executable: valid[5].Executable, Arguments: append(append([]string(nil), valid[5].Arguments...), "extra")},
		{Check: ProbeSandboxSemantics, Executable: valid[5].Executable, Arguments: []string{"--image", digest, "--network=host", "--read-only", "--user=65532:65532", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--security-opt=seccomp=builtin", "--boot-id", bootID}},
		{Check: ProbeRuntimeManifest, Executable: valid[6].Executable, Arguments: []string{
			"runtime-manifest", "--image", digest, "--runtime", "cpp17;id",
			"--profile-version", "2026.07.1", "--compiler-version", "gcc-15.2.0",
			"--runtime-bootstrap-version", "alpine3.23-musl",
			"--compile-command-id", "cpp17.compile.v1", "--run-command-id", "cpp17.run.v1",
			"--manifest-version", "cpp17-artifact.v1",
		}},
		{Check: ProbeCheckerSmoke, Executable: valid[7].Executable, Arguments: []string{"checker-smoke", "--checker", "exact", "--version", "v1", "--supervisor-protocol", "sandbox-supervisor.v1", "extra"}},
	}
	for _, command := range invalid {
		if allowedHostCommand(command) {
			t.Fatalf("invalid command accepted: %+v", command)
		}
	}
}

func TestExecCommandHostPropagatesTimeoutAndBoundsBothOutputStreams(t *testing.T) {
	command := HostCommand{
		Check: ProbeCgroupMembership, Executable: "/usr/bin/cat",
		Arguments: []string{"/proc/self/cgroup"},
	}

	t.Run("timeout", func(t *testing.T) {
		host := execCommandHost{execute: func(
			ctx context.Context,
			_ string,
			_ []string,
			_ io.Writer,
			_ io.Writer,
		) error {
			<-ctx.Done()
			return ctx.Err()
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		if _, err := host.Run(ctx, command); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run() error = %v, want deadline exceeded", err)
		}
	})

	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream+" bound", func(t *testing.T) {
			host := execCommandHost{execute: func(
				_ context.Context,
				_ string,
				_ []string,
				stdout io.Writer,
				stderr io.Writer,
			) error {
				writer := stdout
				if stream == "stderr" {
					writer = stderr
				}
				_, err := io.WriteString(writer, strings.Repeat("x", maximumProbeOutput+1))
				return err
			}}
			if _, err := host.Run(context.Background(), command); !errors.Is(err, ErrCapabilityDenied) {
				t.Fatalf("Run() error = %v, want output bound denial", err)
			}
		})
	}
}
