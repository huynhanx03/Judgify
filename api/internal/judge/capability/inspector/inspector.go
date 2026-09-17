// Package inspector implements the production worker capability helper. It
// accepts validated identities and emits only fixed Docker argv; it never
// invokes a shell or interpolates user-authored source.
package inspector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/huynhanx03/judgify/internal/judge/checker"
)

const (
	dockerExecutable             = "/usr/local/bin/docker"
	inspectorContainerExecutable = "/usr/local/bin/judgify-capability-inspector"
	archiveProofPath             = "/tmp/judgify-capability-archive-proof"
)

var (
	ErrInspectionDenied = errors.New("capability inspector: denied")
	digestPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	identifierPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
)

type Command struct {
	Executable string
	Arguments  []string
}

type Runner interface {
	Run(ctx context.Context, command Command) ([]byte, error)
}

type Inspector struct {
	runner Runner
}

func New(runner Runner) *Inspector {
	return &Inspector{runner: runner}
}

type RuntimeManifestRequest struct {
	ImageDigest             string
	RuntimeKey              string
	ProfileVersion          string
	CompilerVersion         string
	RuntimeBootstrapVersion string
	CompileCommandID        string
	RunCommandID            string
	ManifestVersion         string
}

func (inspector *Inspector) RuntimeManifest(
	ctx context.Context,
	request RuntimeManifestRequest,
) (string, error) {
	if !inspector.valid(ctx) || !digestPattern.MatchString(request.ImageDigest) ||
		!identifierPattern.MatchString(request.RuntimeKey) ||
		!identifierPattern.MatchString(request.ProfileVersion) ||
		!identifierPattern.MatchString(request.CompilerVersion) ||
		!identifierPattern.MatchString(request.RuntimeBootstrapVersion) ||
		!identifierPattern.MatchString(request.CompileCommandID) ||
		!identifierPattern.MatchString(request.RunCommandID) ||
		!identifierPattern.MatchString(request.ManifestVersion) {
		return "", ErrInspectionDenied
	}
	data, err := inspector.runner.Run(ctx, Command{
		Executable: dockerExecutable,
		Arguments:  []string{"image", "inspect", "--format", "{{json .Config.Labels}}", request.ImageDigest},
	})
	if err != nil {
		return "", errors.Join(ErrInspectionDenied, err)
	}
	var labels map[string]string
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&labels); err != nil || labels == nil {
		return "", ErrInspectionDenied
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return "", ErrInspectionDenied
	}
	prefix := "io.judgify.runtime." + request.RuntimeKey + "."
	checksum := labels[prefix+"manifest-checksum"]
	if labels[prefix+"profile-version"] != request.ProfileVersion ||
		labels[prefix+"compiler-version"] != request.CompilerVersion ||
		labels[prefix+"runtime-bootstrap-version"] != request.RuntimeBootstrapVersion ||
		labels[prefix+"compile-command-id"] != request.CompileCommandID ||
		labels[prefix+"run-command-id"] != request.RunCommandID ||
		labels[prefix+"manifest-version"] != request.ManifestVersion ||
		!digestPattern.MatchString(checksum) {
		return "", ErrInspectionDenied
	}
	return checksum, nil
}

type CheckerSmokeRequest struct {
	CheckerKey         string
	Version            string
	SupervisorProtocol string
}

type CheckerSmokeReport struct {
	Passing                       bool   `json:"passing"`
	CheckerImplementationChecksum string `json:"checker_implementation_checksum"`
	SmokeChecksum                 string `json:"smoke_checksum"`
	GoldenChecksum                string `json:"golden_checksum"`
}

func (inspector *Inspector) CheckerSmoke(
	ctx context.Context,
	request CheckerSmokeRequest,
) (CheckerSmokeReport, error) {
	if !inspector.valid(ctx) || !identifierPattern.MatchString(request.CheckerKey) ||
		request.Version != "v1" || !identifierPattern.MatchString(request.SupervisorProtocol) {
		return CheckerSmokeReport{}, ErrInspectionDenied
	}
	return ComputeCheckerSmoke(request)
}

// ComputeCheckerSmoke returns the deterministic checker evidence embedded in
// both a release approval and a worker probe. Keeping this pure calculation in
// one place prevents packaging tooling and the sandbox inspector from
// assigning different identities to the same checker implementation.
func ComputeCheckerSmoke(
	request CheckerSmokeRequest,
) (CheckerSmokeReport, error) {
	if !identifierPattern.MatchString(request.CheckerKey) ||
		request.Version != "v1" || !identifierPattern.MatchString(request.SupervisorProtocol) {
		return CheckerSmokeReport{}, ErrInspectionDenied
	}
	identity := checker.Identity{Key: request.CheckerKey, Version: request.Version}
	implementation, implementationExists := checker.ImplementationChecksum(identity)
	cases, passes := executeGoldenCases(identity)
	if !implementationExists || !digestPattern.MatchString(implementation) ||
		len(cases) == 0 || !passes {
		return CheckerSmokeReport{}, ErrInspectionDenied
	}
	golden := checksum("judgify.checker.golden.v1", request.CheckerKey, strings.Join(cases, "\x00"))
	smoke := checksum(
		"judgify.checker.smoke.v1",
		implementation,
		golden,
		request.SupervisorProtocol,
		"passing",
	)
	return CheckerSmokeReport{
		Passing: true, CheckerImplementationChecksum: implementation,
		SmokeChecksum: smoke, GoldenChecksum: golden,
	}, nil
}

type SandboxRequest struct {
	ImageDigest string
	BootID      string
}

type SandboxReport struct {
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

func (report SandboxReport) Complete() bool {
	return report.NonRootUser && report.UserNamespace && report.ReadOnlyRoot &&
		report.NetworkDisabled && report.NoNewPrivileges && report.Seccomp &&
		report.CapabilitiesDropped && report.KillWait && report.CPUAccounting &&
		report.MemoryAccounting && report.PIDEnforcement && report.OOMAccounting &&
		report.ArchiveCopy && report.Removed
}

func (inspector *Inspector) Sandbox(ctx context.Context, request SandboxRequest) (report SandboxReport, err error) {
	if !inspector.valid(ctx) || !digestPattern.MatchString(request.ImageDigest) || !validUUID(request.BootID) {
		return SandboxReport{}, ErrInspectionDenied
	}
	name := "judge-capability-" + request.BootID
	created := false
	removed := false
	defer func() {
		if created && !removed {
			cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = inspector.runner.Run(cleanupContext, Command{
				Executable: dockerExecutable,
				Arguments:  []string{"container", "rm", "--force", name},
			})
		}
	}()

	createdOutput, err := inspector.runner.Run(ctx, Command{
		Executable: dockerExecutable,
		Arguments: []string{
			"container", "create", "--name", name,
			"--network=none", "--read-only", "--user=65532:65532", "--cap-drop=ALL",
			"--security-opt=no-new-privileges", "--security-opt=seccomp=builtin",
			"--pids-limit=64", "--memory=128m", "--cpus=1",
			"--tmpfs=/tmp:rw,noexec,nosuid,nodev,size=16m,uid=65532,gid=65532,mode=0700",
			request.ImageDigest, inspectorContainerExecutable, "sandbox-self-test",
		},
	})
	if err != nil || strings.TrimSpace(string(createdOutput)) == "" {
		return SandboxReport{}, errors.Join(ErrInspectionDenied, err)
	}
	created = true

	selfTest, err := inspector.runner.Run(ctx, Command{
		Executable: dockerExecutable,
		Arguments:  []string{"container", "start", "--attach", name},
	})
	if err != nil || json.Unmarshal(selfTest, &report) != nil {
		return SandboxReport{}, errors.Join(ErrInspectionDenied, err)
	}
	waited, err := inspector.runner.Run(ctx, Command{
		Executable: dockerExecutable,
		Arguments:  []string{"container", "wait", name},
	})
	report.KillWait = err == nil && strings.TrimSpace(string(waited)) == "0"
	archive, archiveErr := inspector.runner.Run(ctx, Command{
		Executable: dockerExecutable,
		Arguments:  []string{"container", "cp", name + ":" + archiveProofPath, "-"},
	})
	report.ArchiveCopy = archiveErr == nil && len(archive) > 0
	_, removeErr := inspector.runner.Run(ctx, Command{
		Executable: dockerExecutable,
		Arguments:  []string{"container", "rm", "--force", name},
	})
	removed = removeErr == nil
	report.Removed = removed
	if !report.Complete() {
		return SandboxReport{}, errors.Join(ErrInspectionDenied, err, archiveErr, removeErr)
	}
	return report, nil
}

func (inspector *Inspector) valid(ctx context.Context) bool {
	return inspector != nil && inspector.runner != nil && ctx != nil && ctx.Err() == nil
}

func executeGoldenCases(identity checker.Identity) ([]string, bool) {
	implementation, found := checker.Resolve(identity)
	if !found {
		return nil, false
	}
	policy := checker.DefaultPolicy()
	if identity.Key == checker.KeyFloat {
		policy.AbsoluteTolerance = 0.00001
		policy.RelativeTolerance = 0.00001
	}
	type goldenCase struct {
		name     string
		expected string
		actual   string
		accepted bool
	}
	var cases []goldenCase
	switch identity.Key {
	case checker.KeyExact:
		cases = []goldenCase{
			{name: "crlf-normalized", expected: "42\r\n", actual: "42\n", accepted: true},
			{name: "trailing-space-significant", expected: "42 \n", actual: "42\n", accepted: false},
		}
	case checker.KeyToken:
		cases = []goldenCase{
			{name: "ascii-whitespace-normalized", expected: "alpha  beta\n", actual: "alpha\tbeta", accepted: true},
			{name: "different-token", expected: "alpha gamma", actual: "alpha beta", accepted: false},
		}
	case checker.KeyFloat:
		cases = []goldenCase{
			{name: "within-tolerance", expected: "1", actual: "1.000001", accepted: true},
			{name: "outside-tolerance", expected: "1", actual: "1.1", accepted: false},
			{name: "non-finite-actual", expected: "1", actual: "NaN", accepted: false},
		}
	default:
		return nil, false
	}
	results := make([]string, 0, len(cases))
	for _, test := range cases {
		result, err := implementation.Check(
			context.Background(),
			strings.NewReader(test.expected),
			strings.NewReader(test.actual),
			policy,
		)
		if err != nil || result.Accepted != test.accepted {
			return nil, false
		}
		results = append(results, fmt.Sprintf("%s=%t:%s", test.name, result.Accepted, result.Code))
	}
	return results, true
}

func checksum(parts ...string) string {
	digest := sha256.New()
	for _, part := range parts {
		_, _ = fmt.Fprintf(digest, "%08x", len(part))
		_, _ = digest.Write([]byte(part))
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil))
}

func validUUID(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 ||
		len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}
	for _, part := range parts {
		for _, character := range part {
			if !strings.ContainsRune("0123456789abcdef", character) {
				return false
			}
		}
	}
	return value != "00000000-0000-0000-0000-000000000000"
}
