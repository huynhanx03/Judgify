package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

func TestIntakeGateRejectsSameKeyForChangedSemanticPayload(t *testing.T) {
	generated, mock := newIntakeMockClient(t)
	gate, err := NewIntakeGate(wrapSubmissionClient(generated), 3)
	if err != nil {
		t.Fatalf("NewIntakeGate() error = %v", err)
	}
	baseline := intake.Command{
		UserID:         uuid.MustParse("00000000-0000-7000-8000-000000000031"),
		ProblemID:      uuid.MustParse("00000000-0000-7000-8000-000000000021"),
		RuntimeKey:     "go",
		Source:         []byte("package main\nfunc main() {}"),
		IdempotencyKey: "semantic-command",
		CorrelationID:  "semantic-cid",
	}
	storedHash, err := intake.HashCommand(baseline)
	if err != nil {
		t.Fatalf("HashCommand(baseline) error = %v", err)
	}
	changed := baseline
	changed.RuntimeKey = "cpp17"
	changed.Source = []byte("int main() { return 0; }")
	changedHash, err := intake.HashCommand(changed)
	if err != nil {
		t.Fatalf("HashCommand(changed) error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock`).
		WillReturnRows(
			sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil),
		)
	mock.ExpectQuery(`SELECT id, request_hash, submitted_at`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_hash", "submitted_at",
			"contest_submission_sequence", "origin_cid",
		}).AddRow(
			uuid.MustParse("00000000-0000-7000-8000-000000000041"),
			storedHash[:],
			time.Date(2026, time.July, 29, 9, 0, 0, 0, time.UTC),
			nil,
			baseline.CorrelationID,
		))
	mock.ExpectRollback()

	err = platformtx.NewEntUnitOfWork(generated).Within(
		context.Background(),
		func(ctx context.Context) error {
			_, gateErr := gate.LockAndReplay(ctx, intake.CommandIdentity{
				UserID:         baseline.UserID,
				IdempotencyKey: baseline.IdempotencyKey,
				RequestHash:    changedHash,
			})
			return gateErr
		},
	)
	if !errors.Is(err, intake.ErrIdempotencyConflict) {
		t.Fatalf("LockAndReplay() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}
