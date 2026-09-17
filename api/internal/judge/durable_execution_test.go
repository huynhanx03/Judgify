package judge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/judge/assets"
	"github.com/huynhanx03/judgify/internal/judge/capability"
	"github.com/huynhanx03/judgify/internal/judge/checker"
	"github.com/huynhanx03/judgify/internal/judge/executor"
	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	submissionports "github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	"github.com/huynhanx03/judgify/pkg/judging"
)

func TestDurableLeaseExecutorFinalizesAcceptedCasesWithExactPhaseVersions(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	firstCase := leaseExecutionCase(&plan.Tests[0], "2 3\n", "5\n")
	secondCase := leaseExecutionCase(&plan.Tests[1], "7 8\n", "15\n")
	repository := &leaseExecutionRepository{
		plan: plan,
		cases: map[uuid.UUID]submissionports.JudgeCasePayload{
			plan.Tests[0].RevisionID: firstCase,
			plan.Tests[1].RevisionID: secondCase,
		},
	}
	jobs := &leaseExecutionJobs{}
	compiler := &leaseExecutionCompiler{
		outcome: executor.CompileOutcome{
			Status: executor.CompileStatusSucceeded,
			Execution: trustedCaseExecution(
				"compile-container",
				"compile-cgroup",
				20*time.Millisecond,
				16<<20,
			),
			Artifact: &executor.CompiledArtifact{
				AttemptID:        lease.Attempt.ID,
				ManifestChecksum: profile.ManifestChecksum,
				RuntimeProfile:   profile,
			},
		},
	}
	cases := &leaseExecutionCases{results: []executor.CaseResult{
		{
			TestcaseID: plan.Tests[0].RevisionID,
			Execution:  trustedCaseExecution("container-1", "cgroup-1", 7*time.Millisecond, 4<<20),
			Output:     []byte("5\n"),
		},
		{
			TestcaseID: plan.Tests[1].RevisionID,
			Execution:  trustedCaseExecution("container-2", "cgroup-2", 9*time.Millisecond, 5<<20),
			Output:     []byte("15\n"),
		},
	}}
	finalizer := &leaseExecutionFinalizer{}
	releaser := &leaseExecutionReleaser{}
	worker := newLeaseExecutionSubject(
		t,
		repository,
		profile,
		jobs,
		compiler,
		cases,
		finalizer,
		releaser,
	)

	if err := worker.ExecuteLease(leaseExecutionContext(lease), lease); err != nil {
		t.Fatalf("ExecuteLease() error = %v", err)
	}
	if got, want := jobs.transitions, []leaseExecutionTransition{
		{expectedVersion: 10, phase: submissionentity.PhaseCompiling},
		{expectedVersion: 11, phase: submissionentity.PhaseRunning},
		{expectedVersion: 12, phase: submissionentity.PhaseFinalizing},
	}; !leaseExecutionTransitionsEqual(got, want) {
		t.Fatalf("phase transitions = %#v, want %#v", got, want)
	}
	if jobs.retryCalls != 0 {
		t.Fatalf("retry calls = %d, want none", jobs.retryCalls)
	}
	if len(cases.requests) != 2 ||
		!bytes.Equal(cases.requests[0].Input, []byte("2 3\n")) ||
		!bytes.Equal(cases.requests[1].Input, []byte("7 8\n")) {
		t.Fatalf("testcase requests = %#v, want only decoded inputs", cases.requests)
	}
	if cases.requests[0].Identity != cases.requests[1].Identity {
		t.Fatal("all fresh boundaries must retain the same durable attempt identity")
	}
	if finalizer.calls != 1 || finalizer.request.ExpectedSubmissionVersion != 13 {
		t.Fatalf(
			"finalizer calls/version = %d/%d, want 1/13",
			finalizer.calls,
			finalizer.request.ExpectedSubmissionVersion,
		)
	}
	judgement := finalizer.request.Judgement
	if judgement.Verdict != submissionentity.VerdictAccepted ||
		judgement.PassedCount != 2 || judgement.TotalCount != 2 ||
		judgement.WinningAttemptID == nil ||
		*judgement.WinningAttemptID != lease.Attempt.ID {
		t.Fatalf("judgement = %#v, want accepted 2/2 by current attempt", judgement)
	}
	if len(finalizer.request.TestResults) != 2 ||
		finalizer.request.TestResults[0].CaseOrder != 0 ||
		finalizer.request.TestResults[0].ExecutionOrder != 0 ||
		finalizer.request.TestResults[1].CaseOrder != 1 ||
		finalizer.request.TestResults[1].ExecutionOrder != 1 {
		t.Fatalf(
			"test results = %#v, want exact group/global ordering",
			finalizer.request.TestResults,
		)
	}
	if releaser.calls != 1 {
		t.Fatalf("artifact release calls = %d, want one before finalization", releaser.calls)
	}
}

func TestDurableLeaseExecutorFinalizesCompileErrorWithoutRunningCases(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	repository := &leaseExecutionRepository{plan: plan}
	jobs := &leaseExecutionJobs{}
	compiler := &leaseExecutionCompiler{outcome: executor.CompileOutcome{
		Status:     executor.CompileStatusRejected,
		Execution:  trustedCaseExecution("compile-container", "compile-cgroup", 10*time.Millisecond, 16<<20),
		Diagnostic: []byte("main.cpp:1: error: expected ';'\x00secret"),
	}}
	compiler.outcome.Execution.ExitCode = 1
	cases := &leaseExecutionCases{}
	finalizer := &leaseExecutionFinalizer{}
	worker := newLeaseExecutionSubject(
		t,
		repository,
		profile,
		jobs,
		compiler,
		cases,
		finalizer,
		&leaseExecutionReleaser{},
	)

	if err := worker.ExecuteLease(leaseExecutionContext(lease), lease); err != nil {
		t.Fatalf("ExecuteLease() error = %v", err)
	}
	if got, want := jobs.transitions, []leaseExecutionTransition{
		{expectedVersion: 10, phase: submissionentity.PhaseCompiling},
		{expectedVersion: 11, phase: submissionentity.PhaseFinalizing},
	}; !leaseExecutionTransitionsEqual(got, want) {
		t.Fatalf("phase transitions = %#v, want %#v", got, want)
	}
	if len(cases.requests) != 0 || finalizer.calls != 1 {
		t.Fatalf("case/finalizer calls = %d/%d, want 0/1", len(cases.requests), finalizer.calls)
	}
	judgement := finalizer.request.Judgement
	if judgement.Verdict != submissionentity.VerdictCompileError ||
		judgement.PassedCount != 0 || judgement.TotalCount != len(plan.Tests) ||
		judgement.CompileDiagnosticSummary == nil ||
		bytes.Contains([]byte(*judgement.CompileDiagnosticSummary), []byte{0}) {
		t.Fatalf("compile judgement = %#v, want bounded safe compile error", judgement)
	}
}

func TestDurableLeaseExecutorRejectsIncompleteCompileEvidence(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	jobs := &leaseExecutionJobs{}
	finalizer := &leaseExecutionFinalizer{}
	execution := trustedCaseExecution(
		"compile-container",
		"compile-cgroup",
		10*time.Millisecond,
		16<<20,
	)
	execution.ExitCode = 1
	execution.AccountingComplete = false
	worker := newLeaseExecutionSubject(
		t,
		&leaseExecutionRepository{plan: plan},
		profile,
		jobs,
		&leaseExecutionCompiler{outcome: executor.CompileOutcome{
			Status:     executor.CompileStatusRejected,
			Execution:  execution,
			Diagnostic: []byte("untrusted diagnostic"),
		}},
		&leaseExecutionCases{},
		finalizer,
		&leaseExecutionReleaser{},
	)

	if err := worker.ExecuteLease(leaseExecutionContext(lease), lease); err != nil {
		t.Fatalf("ExecuteLease() error = %v, want durable retry handled", err)
	}
	if jobs.retryCalls != 1 ||
		jobs.incident.Classification != "integrity" ||
		jobs.incident.Code != "compile_evidence_invalid" {
		t.Fatalf("retry = calls:%d incident:%#v", jobs.retryCalls, jobs.incident)
	}
	if finalizer.calls != 0 {
		t.Fatalf("finalizer calls = %d, untrusted evidence cannot create a verdict", finalizer.calls)
	}
}

func TestDurableLeaseExecutorRejectsCompiledArtifactOutsideExactLeaseContract(
	t *testing.T,
) {
	plan, lease, profile := leaseExecutionFixture(t)
	tests := []struct {
		name   string
		mutate func(*executor.CompiledArtifact)
	}{
		{
			name: "attempt",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.AttemptID = testutil.ID(200)
			},
		},
		{
			name: "manifest",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.ManifestChecksum =
					"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
		},
		{
			name: "runtime key",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.RuntimeProfile.Key = "python3"
			},
		},
		{
			name: "runtime profile version",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.RuntimeProfile.ProfileVersion = "unexpected"
			},
		},
		{
			name: "sandbox image",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.RuntimeProfile.ImageDigest =
					"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			},
		},
		{
			name: "compile command",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.RuntimeProfile.CompileCommandID = "unexpected"
			},
		},
		{
			name: "run command",
			mutate: func(artifact *executor.CompiledArtifact) {
				artifact.RuntimeProfile.RunCommandID = "unexpected"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := &executor.CompiledArtifact{
				AttemptID:        lease.Attempt.ID,
				ManifestChecksum: profile.ManifestChecksum,
				RuntimeProfile:   profile,
			}
			test.mutate(artifact)
			jobs := &leaseExecutionJobs{}
			finalizer := &leaseExecutionFinalizer{}
			worker := newLeaseExecutionSubject(
				t,
				&leaseExecutionRepository{plan: plan},
				profile,
				jobs,
				&leaseExecutionCompiler{outcome: executor.CompileOutcome{
					Status: executor.CompileStatusSucceeded,
					Execution: trustedCaseExecution(
						"compile-container",
						"compile-cgroup",
						10*time.Millisecond,
						16<<20,
					),
					Artifact: artifact,
				}},
				&leaseExecutionCases{},
				finalizer,
				&leaseExecutionReleaser{},
			)

			if err := worker.ExecuteLease(leaseExecutionContext(lease), lease); err != nil {
				t.Fatalf("ExecuteLease() error = %v, want durable retry handled", err)
			}
			if jobs.retryCalls != 1 ||
				jobs.incident.Classification != "integrity" ||
				jobs.incident.Code != "compiled_artifact_mismatch" {
				t.Fatalf("retry = calls:%d incident:%#v", jobs.retryCalls, jobs.incident)
			}
			if finalizer.calls != 0 {
				t.Fatalf(
					"finalizer calls = %d, mismatched artifact cannot create a verdict",
					finalizer.calls,
				)
			}
		})
	}
}

func TestDurableLeaseExecutorRecordsInfrastructureRetryWithoutInventingVerdict(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	repository := &leaseExecutionRepository{plan: plan}
	jobs := &leaseExecutionJobs{}
	compiler := &leaseExecutionCompiler{
		err: errors.New("docker engine unavailable"),
	}
	finalizer := &leaseExecutionFinalizer{}
	worker := newLeaseExecutionSubject(
		t,
		repository,
		profile,
		jobs,
		compiler,
		&leaseExecutionCases{},
		finalizer,
		&leaseExecutionReleaser{},
	)

	if err := worker.ExecuteLease(leaseExecutionContext(lease), lease); err != nil {
		t.Fatalf("ExecuteLease() error = %v, want durable retry handled", err)
	}
	if jobs.retryCalls != 1 ||
		jobs.incident.Classification != "infrastructure" ||
		jobs.incident.Code != "compile_boundary_failed" {
		t.Fatalf(
			"retry = calls:%d incident:%#v, want classified compile infrastructure failure",
			jobs.retryCalls,
			jobs.incident,
		)
	}
	if finalizer.calls != 0 {
		t.Fatalf("finalizer calls = %d, infrastructure failure cannot invent a verdict", finalizer.calls)
	}
}

func TestDurableLeaseExecutorHonorsGroupStopWithoutSkippingLaterGroups(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	third := plan.Tests[1]
	third.RevisionID = testutil.ID(90)
	third.TestCaseID = testutil.ID(91)
	third.GroupID = testutil.ID(92)
	third.GroupIndex = 1
	third.OrderIndex = 0
	third.StopOnFailure = false
	plan.Tests[0].StopOnFailure = true
	plan.Tests[1].StopOnFailure = true
	plan.Tests = append(plan.Tests, third)
	plan.Limits.TotalAttemptWallMilliseconds =
		plan.Limits.CompileWallTimeMilliseconds +
			plan.Limits.WallTimeMilliseconds*len(plan.Tests)
	firstCase := leaseExecutionCase(&plan.Tests[0], "first\n", "right\n")
	thirdCase := leaseExecutionCase(&plan.Tests[2], "third\n", "third\n")
	repository := &leaseExecutionRepository{
		plan: plan,
		cases: map[uuid.UUID]submissionports.JudgeCasePayload{
			plan.Tests[0].RevisionID: firstCase,
			plan.Tests[2].RevisionID: thirdCase,
		},
	}
	jobs := &leaseExecutionJobs{}
	compiler := &leaseExecutionCompiler{outcome: executor.CompileOutcome{
		Status: executor.CompileStatusSucceeded,
		Execution: trustedCaseExecution(
			"compile-container",
			"compile-cgroup",
			20*time.Millisecond,
			16<<20,
		),
		Artifact: &executor.CompiledArtifact{
			AttemptID:        lease.Attempt.ID,
			ManifestChecksum: profile.ManifestChecksum,
			RuntimeProfile:   profile,
		},
	}}
	cases := &leaseExecutionCases{results: []executor.CaseResult{
		{
			TestcaseID: plan.Tests[0].RevisionID,
			Execution:  trustedCaseExecution("container-1", "cgroup-1", time.Millisecond, 1<<20),
			Output:     []byte("wrong\n"),
		},
		{
			TestcaseID: plan.Tests[2].RevisionID,
			Execution:  trustedCaseExecution("container-2", "cgroup-2", time.Millisecond, 1<<20),
			Output:     []byte("third\n"),
		},
	}}
	finalizer := &leaseExecutionFinalizer{}
	worker := newLeaseExecutionSubject(
		t,
		repository,
		profile,
		jobs,
		compiler,
		cases,
		finalizer,
		&leaseExecutionReleaser{},
	)

	if err := worker.ExecuteLease(leaseExecutionContext(lease), lease); err != nil {
		t.Fatalf("ExecuteLease() error = %v", err)
	}
	if len(cases.requests) != 2 ||
		cases.requests[0].TestcaseID != plan.Tests[0].RevisionID ||
		cases.requests[1].TestcaseID != plan.Tests[2].RevisionID {
		t.Fatalf("executed cases = %#v, want first then next group", cases.requests)
	}
	if finalizer.request.Judgement.Verdict != submissionentity.VerdictWrongAnswer ||
		finalizer.request.Judgement.PassedCount != 1 ||
		finalizer.request.Judgement.TotalCount != 3 ||
		len(finalizer.request.TestResults) != 2 {
		t.Fatalf("partial group judgement = %#v", finalizer.request)
	}
}

func TestDurableLeaseExecutorCancellationReleasesArtifactWithoutRetryOrVerdict(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	firstCase := leaseExecutionCase(&plan.Tests[0], "2 3\n", "5\n")
	repository := &leaseExecutionRepository{
		plan: plan,
		cases: map[uuid.UUID]submissionports.JudgeCasePayload{
			plan.Tests[0].RevisionID: firstCase,
		},
	}
	jobs := &leaseExecutionJobs{}
	compiler := &leaseExecutionCompiler{outcome: executor.CompileOutcome{
		Status: executor.CompileStatusSucceeded,
		Execution: trustedCaseExecution(
			"compile-container", "compile-cgroup", 20*time.Millisecond, 16<<20,
		),
		Artifact: &executor.CompiledArtifact{
			AttemptID:        lease.Attempt.ID,
			ManifestChecksum: profile.ManifestChecksum,
			RuntimeProfile:   profile,
		},
	}}
	finalizer := &leaseExecutionFinalizer{}
	releaser := &leaseExecutionReleaser{}
	worker := newLeaseExecutionSubject(
		t,
		repository,
		profile,
		jobs,
		compiler,
		&leaseExecutionCases{err: submissionports.ErrLeaseCancelled},
		finalizer,
		releaser,
	)

	err := worker.ExecuteLease(leaseExecutionContext(lease), lease)
	if !errors.Is(err, submissionports.ErrLeaseCancelled) {
		t.Fatalf("ExecuteLease() error = %v, want lease cancelled", err)
	}
	if jobs.retryCalls != 0 || finalizer.calls != 0 || releaser.calls != 1 {
		t.Fatalf(
			"retry/finalizer/release = %d/%d/%d, want 0/0/1",
			jobs.retryCalls,
			finalizer.calls,
			releaser.calls,
		)
	}
}

func TestDurableLeaseExecutorDoesNotRetryWhenFinalizationIsFenced(t *testing.T) {
	plan, lease, profile := leaseExecutionFixture(t)
	jobs := &leaseExecutionJobs{}
	finalizer := &leaseExecutionFinalizer{err: submissionports.ErrFinalizationConflict}
	rejectedExecution := trustedCaseExecution(
		"compile-container", "compile-cgroup", time.Millisecond, 16<<20,
	)
	rejectedExecution.ExitCode = 1
	worker := newLeaseExecutionSubject(
		t,
		&leaseExecutionRepository{plan: plan},
		profile,
		jobs,
		&leaseExecutionCompiler{outcome: executor.CompileOutcome{
			Status:    executor.CompileStatusRejected,
			Execution: rejectedExecution,
		}},
		&leaseExecutionCases{},
		finalizer,
		&leaseExecutionReleaser{},
	)

	err := worker.ExecuteLease(leaseExecutionContext(lease), lease)
	if !errors.Is(err, submissionports.ErrFinalizationConflict) {
		t.Fatalf("ExecuteLease() error = %v, want finalization conflict", err)
	}
	if jobs.retryCalls != 0 || finalizer.calls != 1 {
		t.Fatalf("retry/finalizer calls = %d/%d, want 0/1", jobs.retryCalls, finalizer.calls)
	}
}

func newLeaseExecutionSubject(
	t *testing.T,
	repository *leaseExecutionRepository,
	profile problementity.RuntimeProfile,
	jobs *leaseExecutionJobs,
	compiler *leaseExecutionCompiler,
	cases *leaseExecutionCases,
	finalizer *leaseExecutionFinalizer,
	releaser *leaseExecutionReleaser,
) *LeaseExecutor {
	t.Helper()
	worker, err := NewDurableLeaseExecutor(
		LeaseExecutorDependencies{
			Plans:     repository,
			Cases:     repository,
			Catalog:   leaseExecutionCatalog{profile: profile},
			Jobs:      jobs,
			Compiler:  compiler,
			Testcases: cases,
			Assets:    assets.Decoder{},
			Finalizer: finalizer,
			Artifacts: releaser,
		},
		LeaseExecutorConfig{
			MaximumSourceBytes: judging.MaximumSourceCodeBytes,
			FileBytes:          256 << 20,
			TmpfsBytes:         256 << 20,
			DiagnosticBytes:    16 << 10,
			OpenFiles:          128,
			CleanupTimeout:     time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewDurableLeaseExecutor() error = %v", err)
	}
	base := time.Date(2026, time.July, 29, 2, 0, 0, 0, time.UTC)
	tick := 0
	worker.now = func() time.Time {
		value := base.Add(time.Duration(tick) * time.Millisecond)
		tick++
		return value
	}
	worker.newID = func() (uuid.UUID, error) {
		return testutil.ID(99), nil
	}
	return worker
}

func leaseExecutionFixture(
	t *testing.T,
) (submissionports.JudgePlan, submissionports.JobLease, problementity.RuntimeProfile) {
	t.Helper()
	imageDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	manifestDigest := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	profile := problementity.RuntimeProfile{
		ID:                      testutil.ID(30),
		Key:                     "cpp17",
		Language:                "cpp",
		DisplayName:             "C++17",
		ProfileVersion:          "2026.07.1",
		CompilerVersion:         "gcc-15.2.0",
		RuntimeBootstrapVersion: "alpine3.23-musl",
		ImageDigest:             imageDigest,
		CompileCommandID:        "cpp17.compile.v1",
		RunCommandID:            "cpp17.run.v1",
		Compile: problementity.CommandTemplate{
			Executable: "/usr/bin/g++",
			Arguments: []problementity.CommandArgument{
				{Kind: problementity.ArgumentWorkerPath, Value: "source"},
				{Kind: problementity.ArgumentLiteral, Value: "-o"},
				{Kind: problementity.ArgumentWorkerPath, Value: "artifact"},
			},
		},
		Run:                   problementity.CommandTemplate{Executable: "/workspace/artifact/main"},
		DefaultCompileTimeout: 10 * time.Second,
		ManifestVersion:       "cpp17-artifact.v1",
		ManifestChecksum:      manifestDigest,
		Artifacts: []problementity.RuntimeArtifact{{
			Path: "main", Executable: true, MaximumBytes: 32 << 20,
			MaximumFiles: 1, Required: true,
		}},
		Workspace: problementity.RuntimeWorkspace{
			SourcePath: "source/main.cpp",
			Values: []problementity.RuntimeWorkspaceValue{
				{Name: "source", Path: "/workspace/source/main.cpp"},
				{Name: "artifact", Path: "/workspace/artifact/main"},
			},
		},
		Active: true,
	}
	manifest, err := json.Marshal(profile.Artifacts)
	if err != nil {
		t.Fatalf("marshal artifact manifest: %v", err)
	}
	checkerChecksum, exists := checker.ImplementationChecksum(checker.Identity{
		Key:     checker.KeyExact,
		Version: checker.VersionV1,
	})
	if !exists {
		t.Fatal("exact checker implementation checksum is unavailable")
	}
	plan := submissionports.JudgePlan{
		JobID:             testutil.ID(1),
		SubmissionID:      testutil.ID(2),
		JudgeSpecID:       testutil.ID(3),
		Generation:        1,
		UserID:            testutil.ID(4),
		ProblemID:         testutil.ID(5),
		ProblemRevisionID: testutil.ID(6),
		TestsetRevisionID: testutil.ID(7),
		Language:          "cpp",
		JudgeSpecProtocol: "judge-spec.v1",
		Release: submissionports.ReleaseSnapshot{
			ID:                            "judge-release.test",
			ApprovedManifestChecksum:      manifestDigest,
			CheckerImplementationChecksum: checkerChecksum,
		},
		Runtime: submissionports.RuntimeSnapshot{
			Key:                      profile.Key,
			ProfileVersion:           profile.ProfileVersion,
			CompilerVersion:          profile.CompilerVersion,
			BootstrapVersion:         profile.RuntimeBootstrapVersion,
			SandboxImageDigest:       profile.ImageDigest,
			CompileCommandID:         profile.CompileCommandID,
			RunCommandID:             profile.RunCommandID,
			ArtifactManifestVersion:  profile.ManifestVersion,
			ArtifactManifestChecksum: profile.ManifestChecksum,
			ArtifactManifest:         manifest,
			SupervisorProtocol:       "sandbox-supervisor.v1",
		},
		Checker: submissionports.CheckerSnapshot{
			Key: "exact", Version: "v1",
			ImplementationChecksum: checkerChecksum,
			Config:                 json.RawMessage(`{}`),
		},
		RewardFacts: json.RawMessage(`{"base_experience":100}`),
		Limits: submissionports.JudgeLimits{
			CPUTimeMilliseconds:          1000,
			WallTimeMilliseconds:         2000,
			MemoryKilobytes:              64 << 10,
			OutputBytes:                  1 << 20,
			ProcessCount:                 16,
			CompileCPUTimeMilliseconds:   5000,
			CompileWallTimeMilliseconds:  10_000,
			TotalAttemptWallMilliseconds: 14_000,
		},
		SourceCode:      "#include <iostream>\nint main() { return 0; }\n",
		SpecChecksum:    sha256.Sum256([]byte("spec")),
		RewardChecksum:  sha256.Sum256([]byte("reward")),
		TestsetChecksum: sha256.Sum256([]byte("testset")),
		OriginCID:       testutil.ID(8).String(),
		Tests: []submissionports.JudgeTestSpec{
			{
				RevisionID:     testutil.ID(10),
				TestCaseID:     testutil.ID(11),
				GroupID:        testutil.ID(12),
				GroupIndex:     0,
				OrderIndex:     0,
				Kind:           "hidden",
				InputAssetID:   testutil.ID(13),
				OutputAssetID:  testutil.ID(14),
				InputEncoding:  "identity",
				OutputEncoding: "identity",
			},
			{
				RevisionID:     testutil.ID(15),
				TestCaseID:     testutil.ID(16),
				GroupID:        testutil.ID(12),
				GroupIndex:     0,
				OrderIndex:     1,
				Kind:           "hidden",
				InputAssetID:   testutil.ID(17),
				OutputAssetID:  testutil.ID(18),
				InputEncoding:  "identity",
				OutputEncoding: "identity",
			},
		},
	}
	plan.SourceSHA256 = sha256.Sum256([]byte(plan.SourceCode))
	for index := range plan.Tests {
		plan.Tests[index].InputSHA256 = sha256.Sum256(
			[]byte("default-input-" + plan.Tests[index].RevisionID.String()),
		)
		plan.Tests[index].OutputSHA256 = sha256.Sum256(
			[]byte("default-output-" + plan.Tests[index].RevisionID.String()),
		)
		plan.Tests[index].InputBytes = 1
		plan.Tests[index].OutputBytes = 1
	}
	tuple := capability.CapabilityTuple{
		JudgeSpecProtocol:             plan.JudgeSpecProtocol,
		RuntimeKey:                    profile.Key,
		RuntimeProfileVersion:         profile.ProfileVersion,
		CompilerVersion:               profile.CompilerVersion,
		RuntimeBootstrapVersion:       profile.RuntimeBootstrapVersion,
		CompileCommandID:              profile.CompileCommandID,
		RunCommandID:                  profile.RunCommandID,
		ImageDigest:                   profile.ImageDigest,
		ManifestVersion:               profile.ManifestVersion,
		ManifestChecksum:              profile.ManifestChecksum,
		SupervisorProtocol:            plan.Runtime.SupervisorProtocol,
		CheckerKey:                    plan.Checker.Key,
		CheckerVersion:                plan.Checker.Version,
		CheckerImplementationChecksum: plan.Checker.ImplementationChecksum,
	}
	tuple.CapabilityProfileHash, err = capability.ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("compute capability hash: %v", err)
	}
	plan.Runtime.CapabilityProfileHash = tuple.CapabilityProfileHash
	lease := submissionports.JobLease{
		Job: submissionentity.JudgeJob{
			ID:           plan.JobID,
			SubmissionID: plan.SubmissionID,
			Generation:   plan.Generation,
			JudgeSpecID:  plan.JudgeSpecID,
			Lane:         "regular",
			State:        submissionentity.JobLeased,
			Attempts:     1,
			MaxAttempts:  5,
			OriginCID:    plan.OriginCID,
		},
		Attempt: submissionentity.JudgeAttempt{
			ID:            testutil.ID(20),
			JobID:         plan.JobID,
			SubmissionID:  plan.SubmissionID,
			Generation:    plan.Generation,
			JudgeSpecID:   plan.JudgeSpecID,
			AttemptNumber: 1,
			WorkerID:      testutil.ID(21),
			WorkerBootID:  testutil.ID(22),
			State:         submissionentity.AttemptCreated,
			StartedAt:     time.Date(2026, time.July, 29, 1, 59, 0, 0, time.UTC),
		},
		JudgeSpec: submissionports.JudgeSpecClaim{
			ID:                            plan.JudgeSpecID,
			SubmissionID:                  plan.SubmissionID,
			Generation:                    plan.Generation,
			UserID:                        plan.UserID,
			ProblemRevisionID:             plan.ProblemRevisionID,
			TestsetRevisionID:             plan.TestsetRevisionID,
			JudgeSpecProtocol:             plan.JudgeSpecProtocol,
			RuntimeKey:                    plan.Runtime.Key,
			RuntimeProfileVersion:         plan.Runtime.ProfileVersion,
			CompilerVersion:               plan.Runtime.CompilerVersion,
			RuntimeBootstrapVersion:       plan.Runtime.BootstrapVersion,
			CompileCommandID:              plan.Runtime.CompileCommandID,
			RunCommandID:                  plan.Runtime.RunCommandID,
			SandboxImageDigest:            plan.Runtime.SandboxImageDigest,
			ArtifactManifestVersion:       plan.Runtime.ArtifactManifestVersion,
			ArtifactManifestChecksum:      plan.Runtime.ArtifactManifestChecksum,
			SupervisorProtocol:            plan.Runtime.SupervisorProtocol,
			CheckerKey:                    plan.Checker.Key,
			CheckerVersion:                plan.Checker.Version,
			ReleaseID:                     plan.Release.ID,
			ApprovedManifestChecksum:      plan.Release.ApprovedManifestChecksum,
			CheckerImplementationChecksum: plan.Release.CheckerImplementationChecksum,
			CapabilityProfileHash:         plan.Runtime.CapabilityProfileHash,
			SpecChecksum:                  append([]byte(nil), plan.SpecChecksum[:]...),
			RewardSnapshotChecksum:        append([]byte(nil), plan.RewardChecksum[:]...),
		},
		LeaseOwner:        "worker-a",
		LeaseToken:        bytes.Repeat([]byte{7}, sha256.Size),
		LeaseUntil:        time.Date(2026, time.July, 29, 2, 1, 0, 0, time.UTC),
		SubmissionVersion: 10,
		CreatedAt:         time.Date(2026, time.July, 29, 1, 58, 0, 0, time.UTC),
	}
	return plan, lease, profile
}

func leaseExecutionCase(
	spec *submissionports.JudgeTestSpec,
	input string,
	expected string,
) submissionports.JudgeCasePayload {
	inputBytes := []byte(input)
	expectedBytes := []byte(expected)
	inputChecksum := sha256.Sum256(inputBytes)
	expectedChecksum := sha256.Sum256(expectedBytes)
	spec.InputSHA256 = inputChecksum
	spec.OutputSHA256 = expectedChecksum
	spec.InputBytes = int64(len(inputBytes))
	spec.OutputBytes = int64(len(expectedBytes))
	return submissionports.JudgeCasePayload{
		TestcaseRevisionID: spec.RevisionID,
		Input: submissionports.JudgeAssetSnapshot{
			ID:               spec.InputAssetID,
			SHA256:           inputChecksum,
			Encoding:         spec.InputEncoding,
			Content:          inputBytes,
			UncompressedSize: int64(len(inputBytes)),
		},
		Expected: submissionports.JudgeAssetSnapshot{
			ID:               spec.OutputAssetID,
			SHA256:           expectedChecksum,
			Encoding:         spec.OutputEncoding,
			Content:          expectedBytes,
			UncompressedSize: int64(len(expectedBytes)),
		},
	}
}

func trustedCaseExecution(
	containerID string,
	cgroupID string,
	wallTime time.Duration,
	memoryBytes int64,
) supervisor.ExecuteResult {
	return supervisor.ExecuteResult{
		ContainerID:        containerID,
		CgroupID:           cgroupID,
		ExitCode:           0,
		WallTime:           wallTime,
		CPUTime:            wallTime / 2,
		PeakMemoryBytes:    memoryBytes,
		AccountingComplete: true,
		CleanupComplete:    true,
	}
}

func leaseExecutionContext(lease submissionports.JobLease) context.Context {
	return correlation.WithContext(context.Background(), lease.Job.OriginCID)
}

type leaseExecutionRepository struct {
	plan  submissionports.JudgePlan
	cases map[uuid.UUID]submissionports.JudgeCasePayload
}

func (repository *leaseExecutionRepository) ReadJudgePlan(
	context.Context,
	submissionports.JobLease,
) (submissionports.JudgePlan, error) {
	return repository.plan, nil
}

func (repository *leaseExecutionRepository) ReadJudgeCase(
	_ context.Context,
	_ submissionports.JobLease,
	testcaseRevisionID uuid.UUID,
) (submissionports.JudgeCasePayload, error) {
	value, exists := repository.cases[testcaseRevisionID]
	if !exists {
		return submissionports.JudgeCasePayload{}, submissionports.ErrJudgeCaseIntegrity
	}
	return value, nil
}

type leaseExecutionCatalog struct {
	profile problementity.RuntimeProfile
}

func (catalog leaseExecutionCatalog) RuntimeProfile(
	key string,
	version string,
) (problementity.RuntimeProfile, bool) {
	if key != catalog.profile.Key || version != catalog.profile.ProfileVersion {
		return problementity.RuntimeProfile{}, false
	}
	return catalog.profile, true
}

type leaseExecutionTransition struct {
	expectedVersion int64
	phase           submissionentity.Phase
}

type leaseExecutionJobs struct {
	transitions []leaseExecutionTransition
	retryCalls  int
	incident    submissionports.InfrastructureIncident
}

func (jobs *leaseExecutionJobs) TransitionPhase(
	_ context.Context,
	lease submissionports.JobLease,
	expectedVersion int64,
	phase submissionentity.Phase,
) (submissionentity.SubmissionState, error) {
	jobs.transitions = append(jobs.transitions, leaseExecutionTransition{
		expectedVersion: expectedVersion,
		phase:           phase,
	})
	return submissionentity.SubmissionState{
		ID:               lease.Job.SubmissionID,
		LatestGeneration: lease.Job.Generation,
		LatestPhase:      phase,
		AggregateVersion: expectedVersion + 1,
	}, nil
}

func (jobs *leaseExecutionJobs) RecordRetry(
	_ context.Context,
	_ submissionports.JobLease,
	incident submissionports.InfrastructureIncident,
) error {
	jobs.retryCalls++
	jobs.incident = incident
	return nil
}

type leaseExecutionCompiler struct {
	outcome  executor.CompileOutcome
	err      error
	requests []executor.CompileRequest
}

func (compiler *leaseExecutionCompiler) Compile(
	_ context.Context,
	request executor.CompileRequest,
) (executor.CompileOutcome, error) {
	compiler.requests = append(compiler.requests, request)
	return compiler.outcome, compiler.err
}

type leaseExecutionCases struct {
	results  []executor.CaseResult
	requests []executor.TestcaseRequest
	err      error
}

func (cases *leaseExecutionCases) Run(
	_ context.Context,
	request executor.TestcaseRequest,
) (executor.CaseResult, error) {
	captured := request
	captured.Input = append([]byte(nil), request.Input...)
	cases.requests = append(cases.requests, captured)
	if cases.err != nil {
		return executor.CaseResult{}, cases.err
	}
	if len(cases.results) == 0 {
		return executor.CaseResult{}, errors.New("unexpected testcase execution")
	}
	result := cases.results[0]
	cases.results = cases.results[1:]
	return result, nil
}

type leaseExecutionFinalizer struct {
	calls   int
	request submissionports.FinalizationRequest
	err     error
}

func (finalizer *leaseExecutionFinalizer) FinalizeContestantEvaluation(
	_ context.Context,
	request submissionports.FinalizationRequest,
) (submissionports.FinalizationResult, error) {
	finalizer.calls++
	finalizer.request = request
	result := submissionports.FinalizationResult{
		SubmissionID:     request.Lease.Job.SubmissionID,
		JudgementID:      request.Judgement.ID,
		AggregateVersion: request.ExpectedSubmissionVersion + 1,
		FinalizedAt:      request.Judgement.CompletedAt,
		State: submissionentity.SubmissionState{
			ID:               request.Lease.Job.SubmissionID,
			LatestGeneration: request.Lease.Job.Generation,
			LatestPhase:      submissionentity.PhaseTerminal,
			LatestOutcome:    string(request.Judgement.Verdict),
			AggregateVersion: request.ExpectedSubmissionVersion + 1,
		},
	}
	return result, finalizer.err
}

type leaseExecutionReleaser struct {
	calls int
}

func (releaser *leaseExecutionReleaser) Release(
	context.Context,
	*executor.CompiledArtifact,
) error {
	releaser.calls++
	return nil
}

func leaseExecutionTransitionsEqual(
	left []leaseExecutionTransition,
	right []leaseExecutionTransition,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
