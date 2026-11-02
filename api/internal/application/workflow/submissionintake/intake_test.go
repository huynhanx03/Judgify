package submissionintake

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	contestentity "github.com/huynhanx03/judgify/internal/contest/domain"
	contestports "github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestRequestHashUsesExactSourceAndExcludesIdempotencyAndCID(t *testing.T) {
	t.Parallel()
	command := validCommand()
	first, err := HashCommand(command)
	require.NoError(t, err)

	command.IdempotencyKey = "another-key"
	command.CorrelationID = "another-cid"
	second, err := HashCommand(command)
	require.NoError(t, err)
	require.Equal(t, first, second)

	command.Source = append(command.Source, '\n')
	third, err := HashCommand(command)
	require.NoError(t, err)
	require.NotEqual(t, first, third)
}

func TestRequestHashIsLengthDelimitedAcrossOptionalContestIdentity(t *testing.T) {
	t.Parallel()
	command := validCommand()
	ordinary, err := HashCommand(command)
	require.NoError(t, err)

	contestID := uuid.New()
	contestProblemID := uuid.New()
	command.ContestID = &contestID
	command.ContestProblemID = &contestProblemID
	official, err := HashCommand(command)
	require.NoError(t, err)
	require.NotEqual(t, ordinary, official)
}

func TestRequestHashBindsEveryClientControlledSubmissionMeaning(t *testing.T) {
	baseline := validCommand()
	want, err := HashCommand(baseline)
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(*Command)
	}{
		{
			name: "problem",
			mutate: func(command *Command) {
				command.ProblemID = uuid.New()
			},
		},
		{
			name: "runtime",
			mutate: func(command *Command) {
				command.RuntimeKey = "cpp17"
			},
		},
		{
			name: "source",
			mutate: func(command *Command) {
				command.Source = []byte("package main\nfunc main() { println(1) }")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := baseline
			changed.Source = append([]byte(nil), baseline.Source...)
			test.mutate(&changed)
			got, hashErr := HashCommand(changed)
			require.NoError(t, hashErr)
			require.NotEqual(t, want, got)
		})
	}
}

func TestRequestHashBindsBothContestIdentities(t *testing.T) {
	baseline := validCommand()
	contestID := uuid.New()
	contestProblemID := uuid.New()
	baseline.ContestID = &contestID
	baseline.ContestProblemID = &contestProblemID
	want, err := HashCommand(baseline)
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(*Command)
	}{
		{
			name: "contest",
			mutate: func(command *Command) {
				changed := uuid.New()
				command.ContestID = &changed
			},
		},
		{
			name: "contest problem",
			mutate: func(command *Command) {
				changed := uuid.New()
				command.ContestProblemID = &changed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := baseline
			test.mutate(&changed)
			got, hashErr := HashCommand(changed)
			require.NoError(t, hashErr)
			require.NotEqual(t, want, got)
		})
	}
}

func TestExactReplayReturnsBeforeMutableAdmissionChecks(t *testing.T) {
	t.Parallel()
	want := Result{
		SubmissionID: uuid.New(), Generation: 1, Phase: entity.PhaseQueued,
		AggregateVersion: 1, SubmittedAt: time.Now().UTC(),
		CorrelationID: "original-cid",
	}
	order := []string{}
	gate := &gateFake{replay: &want, order: &order}
	workflow := validWorkflow(t, gate, &order)

	got, err := workflow.Execute(context.Background(), validCommand())
	require.NoError(t, err)
	require.True(t, got.ExactReplay)
	require.Equal(t, want.SubmissionID, got.SubmissionID)
	require.Equal(t, want.CorrelationID, got.CorrelationID)
	require.Equal(t, []string{"gate"}, order)
}

func TestNewOrdinaryCommandUsesCanonicalParticipantOrderAndOneTransaction(t *testing.T) {
	t.Parallel()
	order := []string{}
	workflow := validWorkflow(t, &gateFake{order: &order}, &order)

	_, err := workflow.Execute(context.Background(), validCommand())
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{"gate", "identity", "admission", "problem", "capability", "clock", "writer"},
		order,
	)
}

func TestContestCommandUsesReservationTimeAndVerifiesExactSnapshot(t *testing.T) {
	t.Parallel()
	order := []string{}
	contest := &contestFake{order: &order}
	problem := &problemFake{order: &order}
	workflow, err := New(Dependencies{
		Transactions: unitOfWorkFake{}, Gate: &gateFake{order: &order},
		Identity: identityFake{order: &order}, Admission: admissionFake{order: &order},
		Contest: contest, Problem: problem, Capability: capabilityFake{order: &order},
		Clock: clockFake{order: &order}, Writer: writerFake{order: &order},
	})
	require.NoError(t, err)
	command := validCommand()
	contestID := uuid.New()
	contestProblemID := uuid.New()
	command.ContestID = &contestID
	command.ContestProblemID = &contestProblemID

	result, executeErr := workflow.Execute(context.Background(), command)
	require.NoError(t, executeErr)
	require.Equal(t, contest.submittedAt, result.SubmittedAt)
	require.Equal(
		t,
		[]string{"gate", "identity", "admission", "contest", "problem", "capability", "writer"},
		order,
	)
	require.NotNil(t, problem.request.ExpectedProblemRevisionID)
	require.NotNil(t, problem.request.ExpectedTestsetRevisionID)
	require.Equal(t, contest.checkerConfig, problem.request.ExpectedCheckerConfig)
	require.Equal(t, contest.allowedRuntimes, problem.request.ExpectedAllowedRuntimeKeys)
	require.NotNil(t, problem.request.ExpectedSnapshotChecksum)
}

func TestCommandRejectsContestIdentityHalfPresenceAndUnsafeKey(t *testing.T) {
	t.Parallel()
	command := validCommand()
	contestID := uuid.New()
	command.ContestID = &contestID
	_, err := HashCommand(command)
	require.ErrorIs(t, err, ErrInvalidCommand)

	command.ContestID = nil
	command.IdempotencyKey = " key"
	_, err = HashCommand(command)
	require.ErrorIs(t, err, ErrInvalidCommand)

	command.IdempotencyKey = "key\nforged"
	_, err = HashCommand(command)
	require.ErrorIs(t, err, ErrInvalidCommand)
}

func TestWorkflowClassifiesContestAdmissionWithoutLeakingStorageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "closed", err: contestentity.ErrContestNotRunning,
			want: ErrContestUnavailable,
		},
		{
			name: "banned", err: contestentity.ErrContestRegistrationBanned,
			want: ErrContestParticipationDenied,
		},
		{
			name: "full", err: contestentity.ErrContestRegistrationFull,
			want: ErrContestCapacityExhausted,
		},
		{
			name: "problem", err: contestentity.ErrContestProblemUnavailable,
			want: ErrContestProblemUnavailable,
		},
		{name: "storage", err: errors.New("postgres unavailable"), want: ErrPersistence},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			order := []string{}
			workflow, err := New(Dependencies{
				Transactions: unitOfWorkFake{}, Gate: &gateFake{order: &order},
				Identity: identityFake{order: &order}, Admission: admissionFake{order: &order},
				Contest: &contestFake{order: &order, err: test.err},
				Problem: &problemFake{order: &order}, Capability: capabilityFake{order: &order},
				Clock: clockFake{order: &order}, Writer: writerFake{order: &order},
			})
			require.NoError(t, err)
			command := validCommand()
			contestID := uuid.New()
			contestProblemID := uuid.New()
			command.ContestID = &contestID
			command.ContestProblemID = &contestProblemID

			_, executeErr := workflow.Execute(context.Background(), command)
			require.ErrorIs(t, executeErr, test.want)
		})
	}
}

func validCommand() Command {
	return Command{
		UserID: uuid.New(), ProblemID: uuid.New(),
		RuntimeKey: "go", Source: []byte("package main\nfunc main() {}"),
		IdempotencyKey: "submit-1", CorrelationID: "cid-1",
	}
}

type unitOfWorkFake struct{}

func (unitOfWorkFake) Within(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	return fn(ctx)
}

type gateFake struct {
	replay *Result
	err    error
	order  *[]string
}

func (fake *gateFake) LockAndReplay(
	_ context.Context,
	_ CommandIdentity,
) (*Result, error) {
	*fake.order = append(*fake.order, "gate")
	return fake.replay, fake.err
}

type identityFake struct {
	order *[]string
	err   error
}

func (fake identityFake) RequireEligible(context.Context, uuid.UUID) error {
	*fake.order = append(*fake.order, "identity")
	return fake.err
}

type admissionFake struct {
	order *[]string
	err   error
}

func (fake admissionFake) Reserve(context.Context, uuid.UUID) error {
	*fake.order = append(*fake.order, "admission")
	return fake.err
}

type contestFake struct {
	order           *[]string
	err             error
	submittedAt     time.Time
	checkerConfig   json.RawMessage
	allowedRuntimes []string
}

func (fake *contestFake) ReserveContestSubmission(
	context.Context,
	contestports.ReserveContestSubmissionRequest,
) (contestports.ContestSubmissionReservation, error) {
	*fake.order = append(*fake.order, "contest")
	if fake.err != nil {
		return contestports.ContestSubmissionReservation{}, fake.err
	}
	if fake.submittedAt.IsZero() {
		fake.submittedAt = time.Now().UTC()
	}
	if len(fake.checkerConfig) == 0 {
		fake.checkerConfig = json.RawMessage(`{}`)
	}
	if len(fake.allowedRuntimes) == 0 {
		fake.allowedRuntimes = []string{"go"}
	}
	return contestports.ContestSubmissionReservation{
		ProblemRevisionID: uuid.New(), TestsetRevisionID: uuid.New(),
		SubmittedAt: fake.submittedAt, ContestSubmissionSequence: 1,
		ScoringPolicy: contestentity.ScoringPolicyICPC, ScoringPolicyVersion: 1,
		Lane: "contest", CheckerKey: "exact", CheckerVersion: "v1",
		CheckerConfig:           append(json.RawMessage(nil), fake.checkerConfig...),
		AllowedRuntimeKeys:      append([]string(nil), fake.allowedRuntimes...),
		ProblemSnapshotChecksum: [32]byte{1},
	}, nil
}

type problemFake struct {
	order   *[]string
	err     error
	request ProblemRequest
}

func (fake *problemFake) Resolve(
	_ context.Context,
	request ProblemRequest,
) (ProblemSnapshot, error) {
	*fake.order = append(*fake.order, "problem")
	fake.request = request
	if fake.err != nil {
		return ProblemSnapshot{}, fake.err
	}
	return validProblemSnapshot(request), nil
}

type capabilityFake struct {
	order *[]string
	err   error
}

func (fake capabilityFake) Resolve(
	context.Context,
	CapabilityRequest,
) (CapabilitySnapshot, error) {
	*fake.order = append(*fake.order, "capability")
	if fake.err != nil {
		return CapabilitySnapshot{}, fake.err
	}
	return validCapabilitySnapshot(), nil
}

type clockFake struct {
	order *[]string
	now   time.Time
	err   error
}

func (fake clockFake) Now(context.Context) (time.Time, error) {
	*fake.order = append(*fake.order, "clock")
	if fake.now.IsZero() {
		fake.now = time.Now().UTC()
	}
	return fake.now, fake.err
}

type writerFake struct {
	order *[]string
	err   error
}

func (fake writerFake) Persist(
	_ context.Context,
	write AcceptedSubmission,
) (Result, error) {
	*fake.order = append(*fake.order, "writer")
	if fake.err != nil {
		return Result{}, fake.err
	}
	var sequence *int64
	if write.Contest != nil {
		value := write.Contest.ContestSubmissionSequence
		sequence = &value
	}
	return Result{
		SubmissionID: uuid.New(), Generation: 1, Phase: entity.PhaseQueued,
		AggregateVersion: 1, SubmittedAt: write.SubmittedAt,
		ContestSequence: sequence, CorrelationID: write.Command.CorrelationID,
	}, nil
}

func validWorkflow(t *testing.T, gate CommandGate, order *[]string) *Workflow {
	t.Helper()
	workflow, err := New(Dependencies{
		Transactions: unitOfWorkFake{}, Gate: gate,
		Identity: identityFake{order: order}, Admission: admissionFake{order: order},
		Contest: &contestFake{order: order}, Problem: &problemFake{order: order},
		Capability: capabilityFake{order: order}, Clock: clockFake{order: order},
		Writer: writerFake{order: order},
	})
	require.NoError(t, err)
	return workflow
}

func validProblemSnapshot(request ProblemRequest) ProblemSnapshot {
	revisionID := uuid.New()
	testsetID := uuid.New()
	checkerConfig := json.RawMessage(`{}`)
	allowedRuntimes := []string{"go"}
	if request.ExpectedProblemRevisionID != nil {
		revisionID = *request.ExpectedProblemRevisionID
		testsetID = *request.ExpectedTestsetRevisionID
		checkerConfig = append(json.RawMessage(nil), request.ExpectedCheckerConfig...)
		allowedRuntimes = append([]string(nil), request.ExpectedAllowedRuntimeKeys...)
	}
	return ProblemSnapshot{
		ProblemID: request.ProblemID, ProblemRevisionID: revisionID,
		TestsetRevisionID: testsetID, CheckerKey: "exact", CheckerVersion: "v1",
		CheckerConfig: checkerConfig, AllowedRuntimeKeys: allowedRuntimes,
		RewardSnapshot:         json.RawMessage(`{"base_exp":100}`),
		RewardSnapshotChecksum: [32]byte{1}, CPUTimeMilliseconds: 1000,
		WallTimeMilliseconds: 2000, MemoryKilobytes: 262144,
		OutputBytes: 1 << 20, ProcessCount: 16, TestCount: 10,
	}
}

func validCapabilitySnapshot() CapabilitySnapshot {
	return CapabilitySnapshot{
		ReleaseID:                       "judge-release.2026.07.28.1",
		ApprovedReleaseManifestChecksum: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		Language:                        "go",
		JudgeSpecProtocol:               "judge-spec.v1",
		RuntimeProfileVersion:           "2026.07.1", CompilerVersion: "go1.24.5",
		RuntimeBootstrapVersion: "go-module-workspace.v1",
		SandboxImageDigest:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CompileCommandID:        "go.compile.v1", RunCommandID: "go.run.v1",
		ArtifactManifestVersion:       "go-artifact.v1",
		ArtifactManifestChecksum:      "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ArtifactManifest:              json.RawMessage(`[{"path":"main","executable":true}]`),
		SupervisorProtocolVersion:     "supervisor.v1",
		CapabilityProfileHash:         "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		CheckerImplementationChecksum: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		CompileCPUTimeMilliseconds:    10_000,
		CompileWallTimeMilliseconds:   10_000,
	}
}
