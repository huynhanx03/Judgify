package inspector

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

func TestInspectorReadsRuntimeManifestFromPinnedImageLabels(t *testing.T) {
	checksum := "sha256:f3c7dd28b1ecf7e40c4c695129fd1b67cc65a4149421133446f062886c24dd9d"
	runner := &fakeRunner{responses: [][]byte{[]byte(`{
		"io.judgify.runtime.cpp17.profile-version":"2026.07.1",
		"io.judgify.runtime.cpp17.compiler-version":"gcc-15.2.0",
		"io.judgify.runtime.cpp17.runtime-bootstrap-version":"alpine3.23-musl",
		"io.judgify.runtime.cpp17.compile-command-id":"cpp17.compile.v1",
		"io.judgify.runtime.cpp17.run-command-id":"cpp17.run.v1",
		"io.judgify.runtime.cpp17.manifest-version":"cpp17-artifact.v1",
		"io.judgify.runtime.cpp17.manifest-checksum":"` + checksum + `"
	}`)}}
	value, err := New(runner).RuntimeManifest(context.Background(), RuntimeManifestRequest{
		ImageDigest: "sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac",
		RuntimeKey:  "cpp17", ProfileVersion: "2026.07.1",
		CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
		CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
		ManifestVersion: "cpp17-artifact.v1",
	})
	if err != nil {
		t.Fatalf("RuntimeManifest() error = %v", err)
	}
	if value != checksum {
		t.Fatalf("RuntimeManifest() = %q, want %q", value, checksum)
	}
	want := Command{Executable: "/usr/local/bin/docker", Arguments: []string{
		"image", "inspect", "--format", "{{json .Config.Labels}}",
		"sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac",
	}}
	if len(runner.commands) != 1 || runner.commands[0].Executable != want.Executable ||
		!slices.Equal(runner.commands[0].Arguments, want.Arguments) {
		t.Fatalf("commands = %+v, want %+v", runner.commands, want)
	}
}

func TestInspectorRejectsRuntimeManifestWithIncompleteExecutableIdentity(t *testing.T) {
	checksum := "sha256:f3c7dd28b1ecf7e40c4c695129fd1b67cc65a4149421133446f062886c24dd9d"
	labels := []byte(`{
		"io.judgify.runtime.cpp17.profile-version":"2026.07.1",
		"io.judgify.runtime.cpp17.compiler-version":"gcc-15.2.0",
		"io.judgify.runtime.cpp17.runtime-bootstrap-version":"alpine3.23-musl",
		"io.judgify.runtime.cpp17.compile-command-id":"cpp17.compile.v1",
		"io.judgify.runtime.cpp17.manifest-version":"cpp17-artifact.v1",
		"io.judgify.runtime.cpp17.manifest-checksum":"` + checksum + `"
	}`)
	_, err := New(&fakeRunner{responses: [][]byte{labels}}).RuntimeManifest(
		context.Background(),
		RuntimeManifestRequest{
			ImageDigest: "sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac",
			RuntimeKey:  "cpp17", ProfileVersion: "2026.07.1",
			CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
			CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
			ManifestVersion: "cpp17-artifact.v1",
		},
	)
	if err == nil {
		t.Fatal("runtime manifest without run command identity passed")
	}
}

func TestInspectorExecutesBuiltInCheckerGoldenCorpusAndBindsChecksums(t *testing.T) {
	inspector := New(&fakeRunner{})
	for _, checker := range []string{"exact", "token", "float"} {
		report, err := inspector.CheckerSmoke(context.Background(), CheckerSmokeRequest{
			CheckerKey: checker, Version: "v1", SupervisorProtocol: "sandbox-supervisor.v1",
		})
		if err != nil {
			t.Fatalf("CheckerSmoke(%q) error = %v", checker, err)
		}
		if !report.Passing || !digestPattern.MatchString(report.CheckerImplementationChecksum) ||
			!digestPattern.MatchString(report.SmokeChecksum) ||
			!digestPattern.MatchString(report.GoldenChecksum) ||
			report.CheckerImplementationChecksum == report.GoldenChecksum ||
			report.SmokeChecksum == report.GoldenChecksum {
			t.Fatalf("CheckerSmoke(%q) report = %+v", checker, report)
		}
	}
	if _, err := inspector.CheckerSmoke(context.Background(), CheckerSmokeRequest{
		CheckerKey: "custom", Version: "v1", SupervisorProtocol: "sandbox-supervisor.v1",
	}); err == nil {
		t.Fatal("unknown checker golden corpus passed")
	}
}

func TestInspectorRunsSandboxSelfTestWithFixedHardeningAndLifecycle(t *testing.T) {
	selfReport, err := json.Marshal(SandboxReport{
		NonRootUser: true, UserNamespace: true, ReadOnlyRoot: true,
		NetworkDisabled: true, NoNewPrivileges: true, Seccomp: true,
		CapabilitiesDropped: true, CPUAccounting: true, MemoryAccounting: true,
		PIDEnforcement: true, OOMAccounting: true,
	})
	if err != nil {
		t.Fatalf("marshal self report: %v", err)
	}
	runner := &fakeRunner{responses: [][]byte{
		[]byte("judge-capability-99999999-9999-4999-8999-999999999999\n"),
		selfReport,
		[]byte("0\n"),
		[]byte("archive-proof"),
		{},
	}}
	report, err := New(runner).Sandbox(context.Background(), SandboxRequest{
		ImageDigest: "sha256:90caa209195cca93d8c1a9dae708e2a4ecf6b402349a4b3ff899c2d7297157ac",
		BootID:      "99999999-9999-4999-8999-999999999999",
	})
	if err != nil {
		t.Fatalf("Sandbox() error = %v", err)
	}
	if !report.Complete() {
		t.Fatalf("Sandbox() report = %+v", report)
	}
	for _, command := range runner.commands {
		if command.Executable != "/usr/local/bin/docker" || slices.Contains(command.Arguments, "sh") ||
			slices.Contains(command.Arguments, "-c") {
			t.Fatalf("unsafe sandbox command: %+v", command)
		}
	}
	create := runner.commands[0].Arguments
	for _, fixed := range []string{"--network=none", "--read-only", "--user=65532:65532", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=64", "--memory=128m", "--cpus=1"} {
		if !slices.Contains(create, fixed) {
			t.Fatalf("create argv %v missing %q", create, fixed)
		}
	}
}

type fakeRunner struct {
	commands  []Command
	responses [][]byte
	err       error
}

func (runner *fakeRunner) Run(_ context.Context, command Command) ([]byte, error) {
	command.Arguments = append([]string(nil), command.Arguments...)
	runner.commands = append(runner.commands, command)
	if runner.err != nil {
		return nil, runner.err
	}
	if len(runner.responses) == 0 {
		return nil, errors.New("unexpected inspector command")
	}
	response := runner.responses[0]
	runner.responses = runner.responses[1:]
	return append([]byte(nil), response...), nil
}
