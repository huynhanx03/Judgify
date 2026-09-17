package capability

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

func TestCapabilityProbeFailsClosedUnlessEveryHostCheckPasses(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	bootID := uuid.MustParse("99999999-9999-4999-8999-999999999999")
	tuple := initialCPP17Tuple(t)

	passingHost := newPassingProbeHost(t, tuple)
	probe, err := NewProbe(passingHost, func() time.Time { return now })
	if err != nil {
		t.Fatalf("construct capability probe: %v", err)
	}
	evidence, err := probe.Probe(context.Background(), tuple, bootID)
	if err != nil {
		t.Fatalf("probe complete host capability: %v", err)
	}
	if !evidence.Passing || evidence.Tuple != tuple || evidence.BootID != bootID ||
		!evidence.ProbedAt.Equal(now) ||
		evidence.CheckerImplementationChecksum != probeReleaseDigest("checker-exact") ||
		evidence.SmokeChecksum != probeReleaseDigest("checker-exact-smoke") ||
		evidence.GoldenChecksum != probeReleaseDigest("checker-exact-golden") {
		t.Fatalf("passing evidence = %+v", evidence)
	}
	for _, command := range passingHost.commands {
		base := filepath.Base(command.Executable)
		if slices.Contains([]string{"sh", "bash", "dash", "zsh"}, base) {
			t.Fatalf("probe invoked shell command: %+v", command)
		}
	}

	tests := []struct {
		name   string
		mutate func(*fakeProbeHost)
	}{
		{
			name: "rootless Docker missing",
			mutate: func(host *fakeProbeHost) {
				host.results[ProbeDockerSecurity] = probeResult(t, []string{"name=seccomp,profile=default", "name=userns"})
			},
		},
		{
			name: "cgroup controller missing",
			mutate: func(host *fakeProbeHost) {
				host.results[ProbeCgroupControllers] = HostCommandResult{Stdout: []byte("cpu memory\n")}
			},
		},
		{
			name: "cgroup delegation missing",
			mutate: func(host *fakeProbeHost) {
				host.failures[ProbeCgroupDelegation] = errors.New("not delegated")
			},
		},
		{
			name: "floating or different image",
			mutate: func(host *fakeProbeHost) {
				host.results[ProbeImageDigest] = HostCommandResult{Stdout: []byte(`["judgify/runtime:latest"]`)}
			},
		},
		{
			name: "sandbox isolation missing",
			mutate: func(host *fakeProbeHost) {
				report := passingSandboxReport()
				report.NetworkDisabled = false
				host.results[ProbeSandboxSemantics] = probeResult(t, report)
			},
		},
		{
			name: "kill semantics missing",
			mutate: func(host *fakeProbeHost) {
				report := passingSandboxReport()
				report.KillWait = false
				host.results[ProbeSandboxSemantics] = probeResult(t, report)
			},
		},
		{
			name: "accounting missing",
			mutate: func(host *fakeProbeHost) {
				report := passingSandboxReport()
				report.CPUAccounting = false
				host.results[ProbeSandboxSemantics] = probeResult(t, report)
			},
		},
		{
			name: "runtime manifest mismatch",
			mutate: func(host *fakeProbeHost) {
				host.results[ProbeRuntimeManifest] = HostCommandResult{Stdout: []byte("sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff\n")}
			},
		},
		{
			name: "checker golden smoke failed",
			mutate: func(host *fakeProbeHost) {
				report := passingCheckerReport(tuple)
				report.Passing = false
				host.results[ProbeCheckerSmoke] = probeResult(t, report)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host := newPassingProbeHost(t, tuple)
			test.mutate(host)
			probe, err := NewProbe(host, func() time.Time { return now })
			if err != nil {
				t.Fatalf("construct capability probe: %v", err)
			}
			evidence, err := probe.Probe(context.Background(), tuple, bootID)
			if err == nil {
				t.Fatal("incomplete host capability passed probe")
			}
			if evidence.Passing {
				t.Fatal("failed probe returned passing evidence")
			}
			if evidence.Tuple != tuple || evidence.BootID != bootID || !evidence.ProbedAt.Equal(now) {
				t.Fatalf("failed evidence lost identity: %+v", evidence)
			}
		})
	}
}

func TestCapabilityProfileHashBindsEveryExecutableIdentity(t *testing.T) {
	base := initialCPP17Tuple(t)
	for name, mutate := range map[string]func(*CapabilityTuple){
		"compiler version":          func(tuple *CapabilityTuple) { tuple.CompilerVersion = "gcc-15.2.1" },
		"runtime bootstrap version": func(tuple *CapabilityTuple) { tuple.RuntimeBootstrapVersion = "alpine3.24-musl" },
		"compile command":           func(tuple *CapabilityTuple) { tuple.CompileCommandID = "cpp17.compile.v2" },
		"run command":               func(tuple *CapabilityTuple) { tuple.RunCommandID = "cpp17.run.v2" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			hash, err := ComputeCapabilityProfileHash(changed)
			if err != nil {
				t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
			}
			if hash == base.CapabilityProfileHash {
				t.Fatal("executable identity change did not alter capability hash")
			}
			if validateCapabilityTuple(changed) == nil {
				t.Fatal("tuple carrying a hash from a different executable identity passed")
			}
		})
	}
}

func TestContainsExactDigestParsesCanonicalDockerRepoDigests(t *testing.T) {
	digest := probeReleaseDigest("cpp17-image")
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "canonical repository digest", data: `["registry.example/judgify/cpp17@` + digest + `"]`, want: true},
		{name: "one exact digest among repositories", data: `["mirror.example/judgify/cpp17@` + probeReleaseDigest("other-image") + `","registry.example/judgify/cpp17@` + digest + `"]`, want: true},
		{name: "bare digest is not Docker RepoDigests", data: `["` + digest + `"]`},
		{name: "tag is not immutable", data: `["registry.example/judgify/cpp17:latest"]`},
		{name: "digest prefix is not exact", data: `["registry.example/judgify/cpp17@` + digest[:len(digest)-1] + `"]`},
		{name: "empty repository", data: `["@` + digest + `"]`},
		{name: "multiple separators", data: `["registry.example/judgify/cpp17@@` + digest + `"]`},
		{name: "malformed JSON", data: `["registry.example/judgify/cpp17@` + digest + `"`},
		{name: "trailing JSON", data: `["registry.example/judgify/cpp17@` + digest + `"] []`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := containsExactDigest([]byte(test.data), digest); got != test.want {
				t.Fatalf("containsExactDigest(%q, %q) = %t, want %t", test.data, digest, got, test.want)
			}
		})
	}
}

func TestDelegatedCgroupV2DirectoryParsesOnlyCanonicalUnifiedMembership(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "delegated service subtree", data: "0::/system.slice/judgify-worker.service\n", want: "/sys/fs/cgroup/system.slice/judgify-worker.service"},
		{name: "unified root", data: "0::/\n", want: "/sys/fs/cgroup"},
		{name: "legacy v1", data: "2:cpu,memory:/judgify\n"},
		{name: "hybrid membership", data: "0::/judgify\n2:cpu:/judgify\n"},
		{name: "relative path", data: "0::judgify\n"},
		{name: "parent traversal", data: "0::/judgify/../other\n"},
		{name: "noncanonical separators", data: "0::/judgify//worker\n"},
		{name: "wrong hierarchy id", data: "1::/judgify\n"},
		{name: "named controller", data: "0:name=systemd:/judgify\n"},
		{name: "empty input", data: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := delegatedCgroupV2Directory([]byte(test.data))
			if ok != (test.want != "") || got != test.want {
				t.Fatalf("delegatedCgroupV2Directory(%q) = (%q, %t), want (%q, %t)", test.data, got, ok, test.want, test.want != "")
			}
		})
	}
}

func TestDecodeStrictJSONRejectsDuplicateObjectKeysAtEveryDepth(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "top level", data: `{"passing":true,"passing":false,"checker_implementation_checksum":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","smoke_checksum":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","golden_checksum":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}`},
		{name: "nested object", data: `{"outer":{"value":1,"value":2}}`},
		{name: "object nested in array", data: `[{"value":1,"value":2}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var decoded any
			if err := decodeStrictJSON([]byte(test.data), &decoded); err == nil {
				t.Fatal("duplicate JSON object key was accepted")
			}
		})
	}
}

func TestCapabilityProbeChecksTheActualDelegatedCgroupV2Subtree(t *testing.T) {
	tuple := initialCPP17Tuple(t)
	host := newPassingProbeHost(t, tuple)
	host.results[ProbeCgroupMembership] = HostCommandResult{Stdout: []byte("0::/system.slice/judgify-worker.service\n")}
	probe, err := NewProbe(host, func() time.Time {
		return time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("construct capability probe: %v", err)
	}
	if _, err := probe.Probe(
		context.Background(),
		tuple,
		uuid.MustParse("99999999-9999-4999-8999-999999999999"),
	); err != nil {
		t.Fatalf("probe delegated cgroup: %v", err)
	}

	wantControllers := []string{"/sys/fs/cgroup/system.slice/judgify-worker.service/cgroup.controllers"}
	wantDelegation := []string{"-w", "/sys/fs/cgroup/system.slice/judgify-worker.service/cgroup.subtree_control"}
	var gotMembership, gotControllers, gotDelegation []string
	for _, command := range host.commands {
		switch command.Check {
		case ProbeCgroupMembership:
			gotMembership = command.Arguments
		case ProbeCgroupControllers:
			gotControllers = command.Arguments
		case ProbeCgroupDelegation:
			gotDelegation = command.Arguments
		}
	}
	if !slices.Equal(gotMembership, []string{"/proc/self/cgroup"}) ||
		!slices.Equal(gotControllers, wantControllers) ||
		!slices.Equal(gotDelegation, wantDelegation) {
		t.Fatalf("cgroup commands = membership %v, controllers %v, delegation %v", gotMembership, gotControllers, gotDelegation)
	}
}

func TestCapabilityRegistryActivatesOnlyFreshPassingWholeTuple(t *testing.T) {
	now := time.Date(2026, 7, 28, 9, 30, 0, 0, time.UTC)
	worker := NewWorkerInstance(
		uuid.MustParse("88888888-8888-4888-8888-888888888888"),
		"judge-worker-a",
		uuid.MustParse("99999999-9999-4999-8999-999999999999"),
		now,
	)
	validTuple := initialCPP17Tuple(t)

	tests := []struct {
		name   string
		worker WorkerInstance
		claim  CapabilityAdvertisement
	}{
		{
			name:   "stale heartbeat",
			worker: withHeartbeat(worker, now.Add(-2*time.Minute)),
			claim:  passingAdvertisement(validTuple, worker.BootID, now),
		},
		{
			name:   "different boot",
			worker: worker,
			claim: passingAdvertisement(
				validTuple,
				uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
				now,
			),
		},
		{
			name:   "stale probe",
			worker: worker,
			claim:  passingAdvertisement(validTuple, worker.BootID, now.Add(-2*time.Minute)),
		},
		{
			name:   "failing probe",
			worker: worker,
			claim: func() CapabilityAdvertisement {
				claim := passingAdvertisement(validTuple, worker.BootID, now)
				claim.Evidence.Passing = false
				return claim
			}(),
		},
		{
			name:   "separate runtime list",
			worker: worker,
			claim: func() CapabilityAdvertisement {
				claim := passingAdvertisement(validTuple, worker.BootID, now)
				claim.RuntimeKeys = []string{"cpp17", "go"}
				return claim
			}(),
		},
		{
			name:   "version range",
			worker: worker,
			claim: func() CapabilityAdvertisement {
				claim := passingAdvertisement(validTuple, worker.BootID, now)
				claim.RuntimeVersionRange = ">=2026.06"
				return claim
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := initialCatalog(t)
			store := newMemoryEvidenceStore()
			registry, err := NewRegistry(
				catalog,
				store,
				time.Minute,
				[]CapabilityTuple{validTuple},
			)
			if err != nil {
				t.Fatalf("construct capability registry: %v", err)
			}
			if err := registry.RegisterBoot(context.Background(), worker); err != nil {
				t.Fatalf("register worker boot: %v", err)
			}
			if store.worker(worker.ID).ClaimsEnabled {
				t.Fatal("worker started claims-enabled")
			}

			if err := registry.Activate(context.Background(), test.worker, test.claim, now); err == nil {
				t.Fatal("invalid capability advertisement activated")
			}
			if store.worker(worker.ID).ClaimsEnabled {
				t.Fatal("failed activation enabled claims")
			}
			if len(store.evidence) != 0 {
				t.Fatalf("failed activation persisted %d evidence rows", len(store.evidence))
			}
		})
	}

	store := newMemoryEvidenceStore()
	registry, err := NewRegistry(
		initialCatalog(t),
		store,
		time.Minute,
		[]CapabilityTuple{validTuple},
	)
	if err != nil {
		t.Fatalf("construct capability registry: %v", err)
	}
	if err := registry.RegisterBoot(context.Background(), worker); err != nil {
		t.Fatalf("register worker boot: %v", err)
	}
	if registry.CanClaim(worker.ID, worker.BootID, validTuple, now) {
		t.Fatal("claims-disabled worker matched before evidence")
	}
	if err := registry.Activate(
		context.Background(),
		worker,
		passingAdvertisement(validTuple, worker.BootID, now),
		now,
	); err != nil {
		t.Fatalf("activate valid exact tuple: %v", err)
	}
	if !store.worker(worker.ID).ClaimsEnabled {
		t.Fatal("fresh passing evidence did not enable claims")
	}
	if len(store.evidence) != 1 || store.evidence[0].Tuple != validTuple {
		t.Fatalf("persisted evidence = %+v, want one exact tuple", store.evidence)
	}
	if !registry.CanClaim(worker.ID, worker.BootID, validTuple, now) {
		t.Fatal("activated exact tuple did not match")
	}
}

func TestCapabilityRegistryBindsSmokeEvidenceToSelectedCheckerImplementation(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 30, 0, 0, time.UTC)
	worker := NewWorkerInstance(
		uuid.MustParse("88888888-8888-4888-8888-888888888888"),
		"judge-worker-a",
		uuid.MustParse("99999999-9999-4999-8999-999999999999"),
		now,
	)
	tuple := initialCPP17Tuple(t)

	tests := []struct {
		name   string
		mutate func(*CapabilityEvidence)
	}{
		{
			name: "different checker implementation",
			mutate: func(evidence *CapabilityEvidence) {
				evidence.CheckerImplementationChecksum = probeReleaseDigest("different-checker")
			},
		},
		{
			name: "missing checker implementation",
			mutate: func(evidence *CapabilityEvidence) {
				evidence.CheckerImplementationChecksum = ""
			},
		},
		{
			name: "missing golden corpus",
			mutate: func(evidence *CapabilityEvidence) {
				evidence.GoldenChecksum = ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryEvidenceStore()
			registry, err := NewRegistry(
				initialCatalog(t),
				store,
				time.Minute,
				[]CapabilityTuple{tuple},
			)
			if err != nil {
				t.Fatalf("construct capability registry: %v", err)
			}
			if err := registry.RegisterBoot(context.Background(), worker); err != nil {
				t.Fatalf("register worker boot: %v", err)
			}
			advertisement := passingAdvertisement(tuple, worker.BootID, now)
			test.mutate(&advertisement.Evidence)
			if err := registry.Activate(context.Background(), worker, advertisement, now); err == nil {
				t.Fatal("checker evidence not bound to the selected implementation activated")
			}
			if store.worker(worker.ID).ClaimsEnabled || len(store.evidence) != 0 {
				t.Fatal("rejected checker evidence left active durable state")
			}
		})
	}
}

func TestCapabilityRegistryRejectsMissingInactiveAndAlteredComponents(t *testing.T) {
	now := time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC)
	worker := NewWorkerInstance(
		uuid.MustParse("88888888-8888-4888-8888-888888888888"),
		"judge-worker-a",
		uuid.MustParse("99999999-9999-4999-8999-999999999999"),
		now,
	)

	tests := []struct {
		name    string
		catalog func(*testing.T) *problemservice.RuntimeRegistry
		tuple   func(*testing.T) CapabilityTuple
	}{
		{
			name:    "missing runtime",
			catalog: initialCatalog,
			tuple: func(t *testing.T) CapabilityTuple {
				tuple := initialCPP17Tuple(t)
				tuple.RuntimeKey = "ruby3"
				return rehashTuple(t, tuple)
			},
		},
		{
			name: "inactive runtime",
			catalog: func(t *testing.T) *problemservice.RuntimeRegistry {
				profiles := probeRuntimeProfilesForTest(t)
				profiles[0].Active = false
				return catalogFrom(t, profiles, probeCheckerProfilesForTest(t))
			},
			tuple: initialCPP17Tuple,
		},
		{
			name:    "floating image tag",
			catalog: initialCatalog,
			tuple: func(t *testing.T) CapabilityTuple {
				tuple := initialCPP17Tuple(t)
				tuple.ImageDigest = "judgify/runtime:latest"
				return tuple
			},
		},
		{
			name:    "different manifest",
			catalog: initialCatalog,
			tuple: func(t *testing.T) CapabilityTuple {
				tuple := initialCPP17Tuple(t)
				tuple.ManifestChecksum = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
				return rehashTuple(t, tuple)
			},
		},
		{
			name:    "missing checker",
			catalog: initialCatalog,
			tuple: func(t *testing.T) CapabilityTuple {
				tuple := initialCPP17Tuple(t)
				tuple.CheckerKey = "custom"
				return rehashTuple(t, tuple)
			},
		},
		{
			name: "inactive checker",
			catalog: func(t *testing.T) *problemservice.RuntimeRegistry {
				checkers := probeCheckerProfilesForTest(t)
				checkers[0].Active = false
				return catalogFrom(t, probeRuntimeProfilesForTest(t), checkers)
			},
			tuple: initialCPP17Tuple,
		},
		{
			name:    "profile hash does not bind changed field",
			catalog: initialCatalog,
			tuple: func(t *testing.T) CapabilityTuple {
				tuple := initialCPP17Tuple(t)
				tuple.SupervisorProtocol = "sandbox-supervisor.v2"
				return tuple
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryEvidenceStore()
			registry, err := NewRegistry(
				test.catalog(t),
				store,
				time.Minute,
				[]CapabilityTuple{initialCPP17Tuple(t)},
			)
			if err != nil {
				if !errors.Is(err, ErrInvalidCapability) {
					t.Fatalf("construct capability registry: %v", err)
				}
				return
			}
			if err := registry.RegisterBoot(context.Background(), worker); err != nil {
				t.Fatalf("register worker boot: %v", err)
			}
			if err := registry.Activate(
				context.Background(),
				worker,
				passingAdvertisement(test.tuple(t), worker.BootID, now),
				now,
			); err == nil {
				t.Fatal("incomplete or altered component activated")
			}
			if store.worker(worker.ID).ClaimsEnabled || len(store.evidence) != 0 {
				t.Fatal("rejected component left active durable state")
			}
		})
	}
}

func TestCapabilityRegistryKeepsNAndNMinusOneAsIndependentWholeRows(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	worker := NewWorkerInstance(
		uuid.MustParse("88888888-8888-4888-8888-888888888888"),
		"judge-worker-a",
		uuid.MustParse("99999999-9999-4999-8999-999999999999"),
		now,
	)
	profiles := probeRuntimeProfilesForTest(t)
	previous := profiles[0]
	previous.ID = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1")
	previous.ProfileVersion = "2026.06.1"
	previous.ImageDigest = probeReleaseDigest("previous-cpp17-image")
	previous.ManifestVersion = "cpp17-artifact.v0"
	previous.ManifestChecksum = probeReleaseDigest("previous-cpp17-manifest")
	profiles = append(profiles, previous)
	catalog := catalogFrom(t, profiles, probeCheckerProfilesForTest(t))

	currentTuple := initialCPP17Tuple(t)
	previousTuple := currentTuple
	previousTuple.JudgeSpecProtocol = "judge-spec.v0"
	previousTuple.RuntimeProfileVersion = previous.ProfileVersion
	previousTuple.ImageDigest = previous.ImageDigest
	previousTuple.ManifestVersion = previous.ManifestVersion
	previousTuple.ManifestChecksum = previous.ManifestChecksum
	previousTuple.SupervisorProtocol = "sandbox-supervisor.v0"
	previousTuple.CheckerKey = "token"
	previousTuple.CheckerImplementationChecksum =
		probeReleaseDigest("checker-token")
	previousTuple = rehashTuple(t, previousTuple)

	store := newMemoryEvidenceStore()
	registry, err := NewRegistry(
		catalog,
		store,
		time.Minute,
		[]CapabilityTuple{currentTuple, previousTuple},
	)
	if err != nil {
		t.Fatalf("construct capability registry: %v", err)
	}
	if err := registry.RegisterBoot(context.Background(), worker); err != nil {
		t.Fatalf("register worker boot: %v", err)
	}
	if err := registry.Activate(
		context.Background(),
		worker,
		passingAdvertisement(currentTuple, worker.BootID, now),
		now,
	); err == nil {
		t.Fatal("partial evidence set unexpectedly enabled claims")
	}
	if store.worker(worker.ID).ClaimsEnabled || len(store.evidence) != 0 {
		t.Fatal("partial evidence set changed durable state")
	}
	if err := registry.ActivateSet(
		context.Background(),
		worker,
		[]CapabilityAdvertisement{
			passingAdvertisement(currentTuple, worker.BootID, now),
			passingAdvertisement(previousTuple, worker.BootID, now),
		},
		now,
	); err != nil {
		t.Fatalf("activate complete evidence set: %v", err)
	}
	if len(store.evidence) != 2 || store.evidence[0].Tuple != currentTuple || store.evidence[1].Tuple != previousTuple {
		t.Fatalf("persisted rows = %+v, want two independent whole rows", store.evidence)
	}
	if !registry.CanClaim(worker.ID, worker.BootID, currentTuple, now) ||
		!registry.CanClaim(worker.ID, worker.BootID, previousTuple, now) {
		t.Fatal("N or N-1 whole tuple did not match")
	}

	synthetic := currentTuple
	synthetic.RuntimeProfileVersion = previousTuple.RuntimeProfileVersion
	synthetic.ImageDigest = previousTuple.ImageDigest
	synthetic.ManifestVersion = previousTuple.ManifestVersion
	synthetic.ManifestChecksum = previousTuple.ManifestChecksum
	synthetic = rehashTuple(t, synthetic)
	if registry.CanClaim(worker.ID, worker.BootID, synthetic, now) {
		t.Fatal("synthetic Cartesian combination matched independent rows")
	}
}

func TestCapabilityRegistryKeepsBootBoundEvidenceAfterFreshnessWindow(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 30, 0, 0, time.UTC)
	worker := NewWorkerInstance(
		uuid.MustParse("77777777-7777-4777-8777-777777777777"),
		"judge-worker-immutable-proof",
		uuid.MustParse("66666666-6666-4666-8666-666666666666"),
		now.Add(-2*time.Minute),
	)
	tuple := initialCPP17Tuple(t)
	store := newMemoryEvidenceStore()
	registry, err := NewRegistry(
		initialCatalog(t),
		store,
		30*time.Second,
		[]CapabilityTuple{tuple},
	)
	if err != nil {
		t.Fatalf("construct capability registry: %v", err)
	}
	if err := registry.RegisterBoot(context.Background(), worker); err != nil {
		t.Fatalf("register worker boot: %v", err)
	}
	if err := registry.ActivateSet(
		context.Background(),
		worker,
		[]CapabilityAdvertisement{
			passingAdvertisement(
				tuple,
				worker.BootID,
				worker.StartedAt.Add(time.Second),
			),
		},
		now,
	); err != nil {
		t.Fatalf("activate immutable boot evidence: %v", err)
	}
	if !store.worker(worker.ID).ClaimsEnabled || len(store.evidence) != 1 {
		t.Fatal("boot-bound evidence set was not committed atomically")
	}
	if registry.CanClaim(worker.ID, worker.BootID, tuple, now) {
		t.Fatal("stale worker heartbeat remained claim-eligible")
	}
}

func TestCapabilityRegistryBindsEvidenceToTheRegisteredBootTimeline(t *testing.T) {
	now := time.Date(2026, 7, 28, 13, 0, 0, 0, time.UTC)
	worker := NewWorkerInstance(
		uuid.MustParse("88888888-8888-4888-8888-888888888888"),
		"judge-worker-a",
		uuid.MustParse("99999999-9999-4999-8999-999999999999"),
		now.Add(-2*time.Minute),
	)
	tuple := initialCPP17Tuple(t)
	freshWorker := NewWorkerInstance(worker.ID, worker.WorkerKey, worker.BootID, now.Add(-30*time.Second))
	freshWorker.HeartbeatAt = now.Add(-10 * time.Second)

	tests := []struct {
		name       string
		registered WorkerInstance
		activation WorkerInstance
		probedAt   time.Time
	}{
		{
			name:       "caller forges a fresh heartbeat",
			registered: worker,
			activation: func() WorkerInstance {
				forged := worker
				forged.HeartbeatAt = now
				return forged
			}(),
			probedAt: now,
		},
		{
			name:       "probe predates its boot",
			registered: freshWorker,
			activation: freshWorker,
			probedAt:   freshWorker.StartedAt.Add(-time.Second),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryEvidenceStore()
			registry, err := NewRegistry(
				initialCatalog(t),
				store,
				time.Minute,
				[]CapabilityTuple{tuple},
			)
			if err != nil {
				t.Fatalf("construct capability registry: %v", err)
			}
			if err := registry.RegisterBoot(context.Background(), test.registered); err != nil {
				t.Fatalf("register worker boot: %v", err)
			}
			if err := registry.Activate(
				context.Background(),
				test.activation,
				passingAdvertisement(tuple, worker.BootID, test.probedAt),
				now,
			); err == nil {
				t.Fatal("unbound worker boot timeline activated")
			}
			if store.worker(worker.ID).ClaimsEnabled || len(store.evidence) != 0 {
				t.Fatal("rejected timeline left active durable state")
			}
		})
	}
}

func initialCatalog(t *testing.T) *problemservice.RuntimeRegistry {
	t.Helper()
	return catalogFrom(t, probeRuntimeProfilesForTest(t), probeCheckerProfilesForTest(t))
}

func probeRuntimeProfilesForTest(t *testing.T) []problementity.RuntimeProfile {
	t.Helper()
	runtimes, _ := probeLaunchProfilesForTest(t)
	return runtimes
}

func probeCheckerProfilesForTest(t *testing.T) []problementity.CheckerProfile {
	t.Helper()
	_, checkers := probeLaunchProfilesForTest(t)
	return checkers
}

func probeLaunchProfilesForTest(
	t *testing.T,
) ([]problementity.RuntimeProfile, []problementity.CheckerProfile) {
	t.Helper()
	manifest := problemservice.ApprovedReleaseManifest{
		ReleaseID: "judge-release.2026.07.28.1", Approved: true,
		ApprovedBy: uuid.MustParse("15f933ae-b67a-4d90-b58b-af97135bc5dc"),
		AuditID:    uuid.MustParse("0ce7e055-3bf7-450a-85bf-ef05e92cc777"),
		ApprovedAt: time.Date(2026, time.July, 28, 9, 30, 0, 0, time.UTC),
		Runtimes: []problemservice.RuntimeRelease{
			{
				RuntimeKey: "cpp17", ProfileVersion: "2026.07.1",
				CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
				CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
				ImageDigest:     probeReleaseDigest("cpp17-image"),
				ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: probeReleaseDigest("cpp17-manifest"),
			},
			{
				RuntimeKey: "python3", ProfileVersion: "2026.07.1",
				CompilerVersion: "cpython-3.12.14", RuntimeBootstrapVersion: "cpython-3.12.14",
				CompileCommandID: "python3.compile.v1", RunCommandID: "python3.run.v1",
				ImageDigest:     probeReleaseDigest("python3-image"),
				ManifestVersion: "python3-artifact.v1", ManifestChecksum: probeReleaseDigest("python3-manifest"),
			},
			{
				RuntimeKey: "java17", ProfileVersion: "2026.07.1",
				CompilerVersion: "javac-17.0.20", RuntimeBootstrapVersion: "openjdk-17.0.20",
				CompileCommandID: "java17.compile.v1", RunCommandID: "java17.run.v1",
				ImageDigest:     probeReleaseDigest("java17-image"),
				ManifestVersion: "java17-artifact.v1", ManifestChecksum: probeReleaseDigest("java17-manifest"),
			},
			{
				RuntimeKey: "go", ProfileVersion: "2026.07.1",
				CompilerVersion: "go1.25.10", RuntimeBootstrapVersion: "go-module-workspace.v1",
				CompileCommandID: "go.compile.v1", RunCommandID: "go.run.v1",
				ImageDigest:     probeReleaseDigest("go-image"),
				ManifestVersion: "go-artifact.v1", ManifestChecksum: probeReleaseDigest("go-manifest"),
			},
		},
		Checkers: []problemservice.CheckerRelease{
			{CheckerKey: "exact", Version: "v1", ImplementationChecksum: probeReleaseDigest("checker-exact")},
			{CheckerKey: "token", Version: "v1", ImplementationChecksum: probeReleaseDigest("checker-token")},
			{CheckerKey: "float", Version: "v1", ImplementationChecksum: probeReleaseDigest("checker-float")},
		},
		Capabilities: []problemservice.CapabilityRelease{{
			JudgeSpecProtocol: "judge-spec.v1",
			RuntimeKey:        "cpp17", RuntimeProfileVersion: "2026.07.1",
			CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
			CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
			ImageDigest:     probeReleaseDigest("cpp17-image"),
			ManifestVersion: "cpp17-artifact.v1", ManifestChecksum: probeReleaseDigest("cpp17-manifest"),
			SupervisorProtocol: "supervisor.v1", CheckerKey: "exact", CheckerVersion: "v1",
			CheckerImplementationChecksum: probeReleaseDigest("checker-exact"),
			CapabilityProfileHash:         probeReleaseDigest("cpp17-exact-capability"),
		}},
	}
	checksum, err := problemservice.ComputeReleaseManifestChecksum(manifest)
	if err != nil {
		t.Fatalf("ComputeReleaseManifestChecksum() error = %v", err)
	}
	manifest.ApprovalChecksum = checksum
	runtimes, checkers, err := problemservice.BuildLaunchProfiles(manifest)
	if err != nil {
		t.Fatalf("BuildLaunchProfiles() error = %v", err)
	}
	return runtimes, checkers
}

func probeReleaseDigest(label string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(label)))
}

func catalogFrom(
	t *testing.T,
	runtimes []problementity.RuntimeProfile,
	checkers []problementity.CheckerProfile,
) *problemservice.RuntimeRegistry {
	t.Helper()
	registry, err := problemservice.NewRuntimeRegistry(runtimes, checkers)
	if err != nil {
		t.Fatalf("construct initial profile catalog: %v", err)
	}
	return registry
}

func initialCPP17Tuple(t *testing.T) CapabilityTuple {
	t.Helper()
	profile := probeRuntimeProfilesForTest(t)[0]
	tuple := CapabilityTuple{
		JudgeSpecProtocol:             "judge-spec.v1",
		RuntimeKey:                    "cpp17",
		RuntimeProfileVersion:         "2026.07.1",
		CompilerVersion:               profile.CompilerVersion,
		RuntimeBootstrapVersion:       profile.RuntimeBootstrapVersion,
		CompileCommandID:              profile.CompileCommandID,
		RunCommandID:                  profile.RunCommandID,
		ImageDigest:                   profile.ImageDigest,
		ManifestVersion:               profile.ManifestVersion,
		ManifestChecksum:              profile.ManifestChecksum,
		SupervisorProtocol:            "sandbox-supervisor.v1",
		CheckerKey:                    "exact",
		CheckerVersion:                "v1",
		CheckerImplementationChecksum: probeReleaseDigest("checker-exact"),
		CapabilityProfileHash:         "",
	}
	hash, err := ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("compute capability profile hash: %v", err)
	}
	tuple.CapabilityProfileHash = hash
	return tuple
}

func rehashTuple(t *testing.T, tuple CapabilityTuple) CapabilityTuple {
	t.Helper()
	tuple.CapabilityProfileHash = ""
	hash, err := ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("rehash capability tuple: %v", err)
	}
	tuple.CapabilityProfileHash = hash
	return tuple
}

func passingAdvertisement(
	tuple CapabilityTuple,
	bootID uuid.UUID,
	probedAt time.Time,
) CapabilityAdvertisement {
	return CapabilityAdvertisement{Evidence: CapabilityEvidence{
		Tuple:                         tuple,
		BootID:                        bootID,
		CheckerImplementationChecksum: probeReleaseDigest("checker-" + tuple.CheckerKey),
		SmokeChecksum:                 probeReleaseDigest("checker-" + tuple.CheckerKey + "-smoke"),
		GoldenChecksum:                probeReleaseDigest("checker-" + tuple.CheckerKey + "-golden"),
		ProbedAt:                      probedAt,
		Passing:                       true,
	}}
}

func withHeartbeat(worker WorkerInstance, heartbeat time.Time) WorkerInstance {
	worker.HeartbeatAt = heartbeat
	return worker
}

type fakeProbeHost struct {
	results  map[ProbeCheck]HostCommandResult
	failures map[ProbeCheck]error
	commands []HostCommand
}

func newPassingProbeHost(t *testing.T, tuple CapabilityTuple) *fakeProbeHost {
	t.Helper()
	return &fakeProbeHost{
		results: map[ProbeCheck]HostCommandResult{
			ProbeDockerSecurity: probeResult(t, []string{
				"name=rootless",
				"name=seccomp,profile=default",
				"name=userns",
			}),
			ProbeCgroupMembership:  {Stdout: []byte("0::/judgify-worker\n")},
			ProbeCgroupControllers: {Stdout: []byte("cpu memory pids io\n")},
			ProbeCgroupDelegation:  {},
			ProbeImageDigest:       probeResult(t, []string{"registry.example/judgify/runtime@" + tuple.ImageDigest}),
			ProbeSandboxSemantics:  probeResult(t, passingSandboxReport()),
			ProbeRuntimeManifest:   {Stdout: []byte(tuple.ManifestChecksum + "\n")},
			ProbeCheckerSmoke:      probeResult(t, passingCheckerReport(tuple)),
		},
		failures: make(map[ProbeCheck]error),
	}
}

func passingCheckerReport(tuple CapabilityTuple) checkerSmokeReport {
	return checkerSmokeReport{
		Passing:                       true,
		CheckerImplementationChecksum: probeReleaseDigest("checker-" + tuple.CheckerKey),
		SmokeChecksum:                 probeReleaseDigest("checker-" + tuple.CheckerKey + "-smoke"),
		GoldenChecksum:                probeReleaseDigest("checker-" + tuple.CheckerKey + "-golden"),
	}
}

func (host *fakeProbeHost) Run(_ context.Context, command HostCommand) (HostCommandResult, error) {
	command.Arguments = append([]string(nil), command.Arguments...)
	host.commands = append(host.commands, command)
	if err := host.failures[command.Check]; err != nil {
		return HostCommandResult{}, err
	}
	result, exists := host.results[command.Check]
	if !exists {
		return HostCommandResult{}, errors.New("unexpected probe command")
	}
	return result, nil
}

func probeResult(t *testing.T, value any) HostCommandResult {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal probe result: %v", err)
	}
	return HostCommandResult{Stdout: data}
}

func passingSandboxReport() sandboxSemanticsReport {
	return sandboxSemanticsReport{
		NonRootUser: true, UserNamespace: true, ReadOnlyRoot: true,
		NetworkDisabled: true, NoNewPrivileges: true, Seccomp: true,
		CapabilitiesDropped: true, KillWait: true, CPUAccounting: true,
		MemoryAccounting: true, PIDEnforcement: true, OOMAccounting: true,
		ArchiveCopy: true, Removed: true,
	}
}

type memoryEvidenceStore struct {
	workers  map[uuid.UUID]WorkerInstance
	evidence []CapabilityEvidence
}

func newMemoryEvidenceStore() *memoryEvidenceStore {
	return &memoryEvidenceStore{workers: make(map[uuid.UUID]WorkerInstance)}
}

func (store *memoryEvidenceStore) RegisterBoot(_ context.Context, worker WorkerInstance) error {
	worker.ClaimsEnabled = false
	store.workers[worker.ID] = worker
	return nil
}

func (store *memoryEvidenceStore) DisableClaims(_ context.Context, workerID uuid.UUID, bootID uuid.UUID) error {
	worker, exists := store.workers[workerID]
	if !exists || worker.BootID != bootID {
		return errors.New("worker boot not registered")
	}
	worker.ClaimsEnabled = false
	store.workers[workerID] = worker
	return nil
}

func (store *memoryEvidenceStore) RecordPassingEvidenceSetAndEnableClaims(
	_ context.Context,
	worker WorkerInstance,
	evidence []CapabilityEvidence,
) error {
	current, exists := store.workers[worker.ID]
	if !exists || current.BootID != worker.BootID || current.ClaimsEnabled ||
		len(evidence) == 0 {
		return errors.New("worker is not claims-disabled on the registered boot")
	}
	store.evidence = append(store.evidence, evidence...)
	current.ClaimsEnabled = true
	store.workers[worker.ID] = current
	return nil
}

func (store *memoryEvidenceStore) worker(id uuid.UUID) WorkerInstance {
	return store.workers[id]
}
