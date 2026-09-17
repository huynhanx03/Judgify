package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func TestDurableIntakeWriterPersistsAggregateSpecJobAndSafeOutboxTogether(t *testing.T) {
	generated, mock := newIntakeMockClient(t)
	client := wrapSubmissionClient(generated)
	outbox := &recordingDurableIntakeOutbox{}
	writer, err := NewDurableIntakeWriter(
		client,
		outbox,
		recordingDurableIntakeAudit{},
		3,
	)
	if err != nil {
		t.Fatalf("NewDurableIntakeWriter() error = %v", err)
	}
	writer.newID = deterministicIntakeIDs(t, 4)
	accepted := validAcceptedSubmissionForWriter(t)

	mock.ExpectBegin()
	expectDurableIntakeRows(mock)
	mock.ExpectCommit()

	var result intake.Result
	err = platformtx.NewEntUnitOfWork(generated).Within(
		context.Background(),
		func(txCtx context.Context) error {
			var persistErr error
			result, persistErr = writer.Persist(txCtx, accepted)
			return persistErr
		},
	)
	if err != nil {
		t.Fatalf("Persist() error = %v", err)
	}
	if result.Generation != 1 ||
		result.AggregateVersion != 1 ||
		result.CorrelationID != accepted.Command.CorrelationID ||
		len(outbox.events) != 1 ||
		outbox.events[0].Type != submissioncontract.EventSubmissionQueuedV1 {
		t.Fatalf("result=%+v events=%+v", result, outbox.events)
	}
	if bytes.Contains(outbox.events[0].Payload, accepted.Command.Source) ||
		bytes.Contains(
			outbox.events[0].Payload,
			[]byte("checker-secret-marker"),
		) {
		t.Fatal("browser-safe queued event leaked source or checker configuration")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func TestDurableIntakeWriterOutboxFailureRollsBackAllRows(t *testing.T) {
	generated, mock := newIntakeMockClient(t)
	client := wrapSubmissionClient(generated)
	outboxErr := errors.New("outbox unavailable")
	writer, err := NewDurableIntakeWriter(
		client,
		&recordingDurableIntakeOutbox{err: outboxErr},
		recordingDurableIntakeAudit{},
		3,
	)
	if err != nil {
		t.Fatalf("NewDurableIntakeWriter() error = %v", err)
	}
	writer.newID = deterministicIntakeIDs(t, 4)

	mock.ExpectBegin()
	expectDurableIntakeRows(mock)
	mock.ExpectRollback()

	err = platformtx.NewEntUnitOfWork(generated).Within(
		context.Background(),
		func(txCtx context.Context) error {
			_, persistErr := writer.Persist(
				txCtx,
				validAcceptedSubmissionForWriter(t),
			)
			return persistErr
		},
	)
	if !errors.Is(err, outboxErr) {
		t.Fatalf("Persist() error = %v, want outbox error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func expectDurableIntakeRows(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`INSERT INTO submissions`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE user_stats`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE problems`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO judge_specs`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO judge_jobs`).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func validAcceptedSubmissionForWriter(
	t *testing.T,
) intake.AcceptedSubmission {
	t.Helper()
	command := intake.Command{
		UserID:         uuid.New(),
		ProblemID:      uuid.New(),
		RuntimeKey:     "go",
		Source:         []byte("package main\nfunc main() {}"),
		IdempotencyKey: "writer-test-key",
		CorrelationID:  "writer-test-cid",
	}
	requestHash, err := intake.HashCommand(command)
	if err != nil {
		t.Fatalf("HashCommand() error = %v", err)
	}
	sourceHash := sha256.Sum256(command.Source)
	return intake.AcceptedSubmission{
		Command:      command,
		RequestHash:  requestHash,
		SourceSHA256: sourceHash,
		SubmittedAt: time.Date(
			2026,
			time.July,
			28,
			10,
			0,
			0,
			0,
			time.UTC,
		),
		Problem: intake.ProblemSnapshot{
			ProblemID:         command.ProblemID,
			ProblemRevisionID: uuid.New(),
			TestsetRevisionID: uuid.New(),
			CheckerKey:        "exact",
			CheckerVersion:    "v1",
			CheckerConfig: json.RawMessage(
				`{"private":"checker-secret-marker"}`,
			),
			AllowedRuntimeKeys:     []string{"go"},
			RewardSnapshot:         json.RawMessage(`{"base_exp":100}`),
			RewardSnapshotChecksum: [32]byte{1},
			CPUTimeMilliseconds:    1000,
			WallTimeMilliseconds:   2000,
			MemoryKilobytes:        262144,
			OutputBytes:            1 << 20,
			ProcessCount:           16,
			TestCount:              10,
		},
		Capability: intake.CapabilitySnapshot{
			ReleaseID:                       "judge-release.2026.07.28.1",
			ApprovedReleaseManifestChecksum: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			Language:                        "go",
			JudgeSpecProtocol:               "judge-spec.v1",
			RuntimeProfileVersion:           "2026.07.1",
			CompilerVersion:                 "go1.25.10",
			RuntimeBootstrapVersion:         "go-module-workspace.v1",
			SandboxImageDigest:              "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			CompileCommandID:                "go.compile.v1",
			RunCommandID:                    "go.run.v1",
			ArtifactManifestVersion:         "go-artifact.v1",
			ArtifactManifestChecksum:        "sha256:2222222222222222222222222222222222222222222222222222222222222222",
			ArtifactManifest: json.RawMessage(
				`[{"path":"main","executable":true}]`,
			),
			SupervisorProtocolVersion:     "supervisor.v1",
			CapabilityProfileHash:         "sha256:3333333333333333333333333333333333333333333333333333333333333333",
			CheckerImplementationChecksum: "sha256:4444444444444444444444444444444444444444444444444444444444444444",
			CompileCPUTimeMilliseconds:    10_000,
			CompileWallTimeMilliseconds:   10_000,
			TotalAttemptWallMilliseconds:  30_000,
		},
	}
}

func deterministicIntakeIDs(
	t *testing.T,
	count int,
) func() (uuid.UUID, error) {
	t.Helper()
	ids := make([]uuid.UUID, count)
	for index := range ids {
		ids[index] = uuid.New()
	}
	next := 0
	return func() (uuid.UUID, error) {
		if next >= len(ids) {
			t.Fatal("durable intake requested too many identities")
		}
		result := ids[next]
		next++
		return result, nil
	}
}

type recordingDurableIntakeOutbox struct {
	events []eventing.Event
	err    error
}

func (outbox *recordingDurableIntakeOutbox) Append(
	ctx context.Context,
	events ...eventing.Event,
) error {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return err
	}
	outbox.events = append(outbox.events, events...)
	return outbox.err
}

type recordingDurableIntakeAudit struct{}

func (recordingDurableIntakeAudit) Append(
	ctx context.Context,
	entry audit.Entry,
) (uuid.UUID, error) {
	if _, err := platformtx.Client(ctx, nil); err != nil {
		return uuid.Nil, err
	}
	return entry.ID, nil
}
