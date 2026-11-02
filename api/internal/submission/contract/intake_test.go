package contract

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestJudgeSpecChecksumIsDeterministicAcrossJSONFormatting(t *testing.T) {
	input := validJudgeSpecChecksumInputForTest()
	first, err := ComputeJudgeSpecChecksum(input)
	if err != nil {
		t.Fatalf("ComputeJudgeSpecChecksum() error = %v", err)
	}
	input.ArtifactManifest = json.RawMessage(
		`[ { "executable": true, "path": "main" } ]`,
	)
	input.CheckerConfig = json.RawMessage(`{ "epsilon": 0.001 }`)
	input.RewardSnapshot = json.RawMessage(`{ "base_exp": 100 }`)
	second, err := ComputeJudgeSpecChecksum(input)
	if err != nil {
		t.Fatalf("ComputeJudgeSpecChecksum() reformatted error = %v", err)
	}
	if first != second {
		t.Fatalf("canonical checksum changed: %x != %x", first, second)
	}
}

func TestJudgeSpecChecksumBindsEveryExecutionIdentity(t *testing.T) {
	original := validJudgeSpecChecksumInputForTest()
	want, err := ComputeJudgeSpecChecksum(original)
	if err != nil {
		t.Fatalf("ComputeJudgeSpecChecksum() error = %v", err)
	}
	mutations := []struct {
		name   string
		mutate func(*JudgeSpecChecksumInput)
	}{
		{"runtime", func(value *JudgeSpecChecksumInput) { value.RuntimeKey = "go.next" }},
		{"profile", func(value *JudgeSpecChecksumInput) { value.RuntimeProfileVersion = "2026.07.2" }},
		{"image", func(value *JudgeSpecChecksumInput) {
			value.SandboxImageDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"checker", func(value *JudgeSpecChecksumInput) { value.CheckerVersion = "v2" }},
		{"release approval", func(value *JudgeSpecChecksumInput) {
			value.ApprovedReleaseManifestChecksum = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		}},
		{"checker implementation", func(value *JudgeSpecChecksumInput) {
			value.CheckerImplementationChecksum = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{"limits", func(value *JudgeSpecChecksumInput) { value.WallTimeMilliseconds++ }},
		{"source", func(value *JudgeSpecChecksumInput) { value.SourceSHA256[1]++ }},
		{"problem revision", func(value *JudgeSpecChecksumInput) { value.ProblemRevisionID = uuid.New() }},
		{"manifest", func(value *JudgeSpecChecksumInput) {
			value.ArtifactManifest = json.RawMessage(`[{"path":"other","executable":true}]`)
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := original
			mutation.mutate(&changed)
			got, err := ComputeJudgeSpecChecksum(changed)
			if err != nil {
				t.Fatalf("ComputeJudgeSpecChecksum() error = %v", err)
			}
			if got == want {
				t.Fatal("execution identity mutation did not change checksum")
			}
		})
	}
}

func TestJudgeSpecChecksumRejectsMalformedOrTrailingJSON(t *testing.T) {
	for _, value := range []json.RawMessage{
		json.RawMessage(``),
		json.RawMessage(`[]`),
		json.RawMessage(`[{"path":"main"}] trailing`),
		json.RawMessage(`{"path":"main"}`),
	} {
		input := validJudgeSpecChecksumInputForTest()
		input.ArtifactManifest = value
		if _, err := ComputeJudgeSpecChecksum(input); err == nil {
			t.Fatalf("artifact manifest %q unexpectedly accepted", value)
		}
	}
}

func TestJudgeSpecChecksumRejectsMalformedIdentifiersAndDigests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*JudgeSpecChecksumInput)
	}{
		{name: "runtime identifier", mutate: func(value *JudgeSpecChecksumInput) {
			value.RuntimeKey = "Go latest"
		}},
		{name: "release identifier", mutate: func(value *JudgeSpecChecksumInput) {
			value.ReleaseID = " release"
		}},
		{name: "image digest", mutate: func(value *JudgeSpecChecksumInput) {
			value.SandboxImageDigest = "sha256:1234"
		}},
		{name: "approval digest", mutate: func(value *JudgeSpecChecksumInput) {
			value.ApprovedReleaseManifestChecksum = "sha256:xyz"
		}},
		{name: "checker implementation digest", mutate: func(value *JudgeSpecChecksumInput) {
			value.CheckerImplementationChecksum = "sha256:1234"
		}},
		{name: "profile digest", mutate: func(value *JudgeSpecChecksumInput) {
			value.CapabilityProfileHash = "sha256:1234"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validJudgeSpecChecksumInputForTest()
			test.mutate(&input)
			if _, err := ComputeJudgeSpecChecksum(input); err == nil {
				t.Fatal("malformed immutable execution identity was accepted")
			}
		})
	}
}

func validJudgeSpecChecksumInputForTest() JudgeSpecChecksumInput {
	return JudgeSpecChecksumInput{
		SubmissionID:                    uuid.New(),
		UserID:                          uuid.New(),
		ProblemID:                       uuid.New(),
		Generation:                      1,
		ProblemRevisionID:               uuid.New(),
		TestsetRevisionID:               uuid.New(),
		Language:                        "go",
		RuntimeKey:                      "go",
		RuntimeProfileVersion:           "2026.07.1",
		CompilerVersion:                 "go1.25.10",
		RuntimeBootstrapVersion:         "go-module-workspace.v1",
		SandboxImageDigest:              "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		CompileCommandID:                "go.compile.v1",
		RunCommandID:                    "go.run.v1",
		ArtifactManifestVersion:         "go-artifact.v1",
		ArtifactManifestChecksum:        "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		ArtifactManifest:                json.RawMessage(`[{"path":"main","executable":true}]`),
		SupervisorProtocol:              "supervisor.v1",
		CheckerKey:                      "exact",
		CheckerVersion:                  "v1",
		CheckerConfig:                   json.RawMessage(`{"epsilon":0.001}`),
		ReleaseID:                       "judge-release.2026.07.28.1",
		ApprovedReleaseManifestChecksum: "sha256:3333333333333333333333333333333333333333333333333333333333333333",
		CheckerImplementationChecksum:   "sha256:4444444444444444444444444444444444444444444444444444444444444444",
		CapabilityProfileHash:           "sha256:5555555555555555555555555555555555555555555555555555555555555555",
		RewardSnapshot:                  json.RawMessage(`{"base_exp":100}`),
		RewardSnapshotChecksum:          [32]byte{1},
		CPUTimeMilliseconds:             1000,
		WallTimeMilliseconds:            2000,
		MemoryKilobytes:                 262144,
		OutputBytes:                     1 << 20,
		ProcessCount:                    16,
		CompileCPUMilliseconds:          5000,
		CompileWallMilliseconds:         10000,
		TotalWallMilliseconds:           30000,
		SourceSHA256:                    [32]byte{2},
		JudgeSpecProtocol:               "judge-spec.v1",
	}
}
