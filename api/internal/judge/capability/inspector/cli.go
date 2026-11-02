package inspector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
)

const maximumCommandOutput = 64 << 10

func RunCLI(ctx context.Context, arguments []string, stdout io.Writer, runner Runner) error {
	if ctx == nil || ctx.Err() != nil || stdout == nil || runner == nil || len(arguments) == 0 {
		return ErrInspectionDenied
	}
	inspector := New(runner)
	switch arguments[0] {
	case "sandbox":
		if len(arguments) != 11 || arguments[1] != "--image" ||
			!slices.Equal(arguments[3:10], []string{
				"--network=none", "--read-only", "--user=65532:65532", "--cap-drop=ALL",
				"--security-opt=no-new-privileges", "--security-opt=seccomp=builtin", "--boot-id",
			}) {
			return ErrInspectionDenied
		}
		report, err := inspector.Sandbox(ctx, SandboxRequest{ImageDigest: arguments[2], BootID: arguments[10]})
		if err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(report)
	case "runtime-manifest":
		if len(arguments) != 17 || arguments[1] != "--image" || arguments[3] != "--runtime" ||
			arguments[5] != "--profile-version" || arguments[7] != "--compiler-version" ||
			arguments[9] != "--runtime-bootstrap-version" || arguments[11] != "--compile-command-id" ||
			arguments[13] != "--run-command-id" || arguments[15] != "--manifest-version" {
			return ErrInspectionDenied
		}
		manifest, err := inspector.RuntimeManifest(ctx, RuntimeManifestRequest{
			ImageDigest: arguments[2], RuntimeKey: arguments[4],
			ProfileVersion: arguments[6], CompilerVersion: arguments[8],
			RuntimeBootstrapVersion: arguments[10], CompileCommandID: arguments[12],
			RunCommandID: arguments[14], ManifestVersion: arguments[16],
		})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, manifest)
		return err
	case "checker-smoke":
		if len(arguments) != 7 || arguments[1] != "--checker" || arguments[3] != "--version" ||
			arguments[5] != "--supervisor-protocol" {
			return ErrInspectionDenied
		}
		report, err := inspector.CheckerSmoke(ctx, CheckerSmokeRequest{
			CheckerKey: arguments[2], Version: arguments[4], SupervisorProtocol: arguments[6],
		})
		if err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(report)
	case "sandbox-self-test":
		if len(arguments) != 1 {
			return ErrInspectionDenied
		}
		report, err := SandboxSelfTest()
		if err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(report)
	default:
		return ErrInspectionDenied
	}
}

type OSRunner struct{}

func NewOSRunner() Runner {
	return OSRunner{}
}

func (OSRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || command.Executable != dockerExecutable {
		return nil, ErrInspectionDenied
	}
	stdout := &limitedBuffer{limit: maximumCommandOutput}
	stderr := &limitedBuffer{limit: maximumCommandOutput}
	execution := exec.CommandContext(ctx, command.Executable, command.Arguments...)
	execution.Stdout = stdout
	execution.Stderr = stderr
	if err := execution.Run(); err != nil {
		return nil, errors.Join(ErrInspectionDenied, err)
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}

type limitedBuffer struct {
	limit int
	data  bytes.Buffer
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	if len(data) > buffer.limit-buffer.data.Len() {
		return 0, ErrInspectionDenied
	}
	return buffer.data.Write(data)
}

func (buffer *limitedBuffer) Bytes() []byte {
	return buffer.data.Bytes()
}
