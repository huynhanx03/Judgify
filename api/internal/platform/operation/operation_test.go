package operation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEncodeRequestProducesStableReplayHash(t *testing.T) {
	t.Parallel()

	type request struct {
		Selector string `json:"selector"`
		Limit    int    `json:"limit"`
	}
	firstPayload, firstHash, err := EncodeRequest(request{Selector: "active", Limit: 50})
	if err != nil {
		t.Fatalf("EncodeRequest(first): %v", err)
	}
	secondPayload, secondHash, err := EncodeRequest(request{Selector: "active", Limit: 50})
	if err != nil {
		t.Fatalf("EncodeRequest(second): %v", err)
	}
	if string(firstPayload) != string(secondPayload) || firstHash != secondHash {
		t.Fatal("equivalent typed requests did not produce one canonical representation")
	}
	_, changedHash, err := EncodeRequest(request{Selector: "active", Limit: 51})
	if err != nil {
		t.Fatalf("EncodeRequest(changed): %v", err)
	}
	if firstHash == changedHash {
		t.Fatal("changed request reused the original hash")
	}
	if _, _, err := EncodeRequest("raw callback"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("EncodeRequest(scalar) error = %v, want ErrInvalidRequest", err)
	}
}

func TestValidateBatchResultRequiresExactMonotonicProgress(t *testing.T) {
	t.Parallel()

	current := validTestOperation(t)
	current.Total = 3
	current.Processed = 1
	current.Succeeded = 1
	current.BatchSize = 2
	if err := ValidateBatchResult(current, BatchResult{
		Processed: 2, Succeeded: 1, Failed: 1,
		NextCheckpoint: []byte(`{"after":"item-3"}`),
		Failures:       []ItemFailure{{Reference: "item-3", Code: "invalid_state"}},
		Done:           true, FailureCode: "partial_failure",
	}); err != nil {
		t.Fatalf("valid terminal batch rejected: %v", err)
	}
	if err := ValidateBatchResult(current, BatchResult{
		Processed: 2, Succeeded: 2, Failed: 1,
		NextCheckpoint: []byte(`{"after":"item-3"}`),
		Done:           true,
	}); !errors.Is(err, ErrInvalidBatchResult) {
		t.Fatalf("inconsistent counters error = %v, want ErrInvalidBatchResult", err)
	}
	if err := ValidateBatchResult(current, BatchResult{
		Processed: 1, Succeeded: 1,
		NextCheckpoint: []byte(`{"after":"item-2"}`),
		Done:           true,
	}); !errors.Is(err, ErrInvalidBatchResult) {
		t.Fatalf("premature done error = %v, want ErrInvalidBatchResult", err)
	}
}

func TestRegisterJSONRejectsUnknownDurableRequestFields(t *testing.T) {
	t.Parallel()

	type request struct {
		Mode string `json:"mode"`
	}
	called := false
	registry := NewRegistry()
	if err := RegisterJSON(registry, "catalog.rebuild.v1", func(
		context.Context,
		request,
		BatchInput,
	) (BatchResult, error) {
		called = true
		return BatchResult{}, nil
	}); err != nil {
		t.Fatalf("RegisterJSON: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	executor, ok := registry.Lookup("catalog.rebuild.v1")
	if !ok {
		t.Fatal("registered executor was not found")
	}
	_, err := executor.ExecuteBatch(context.Background(), BatchInput{
		Request: []byte(`{"mode":"full","callback":"forbidden"}`),
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ExecuteBatch error = %v, want ErrInvalidRequest", err)
	}
	if called {
		t.Fatal("domain executor ran for a request with unknown fields")
	}
}

func validTestOperation(t *testing.T) Operation {
	t.Helper()
	request, hash, err := EncodeRequest(struct {
		Mode string `json:"mode"`
	}{Mode: "full"})
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	now := time.Now().UTC()
	userID := uuid.New()
	sessionID := uuid.New()
	value := Operation{
		ID: uuid.New(), Kind: "catalog.rebuild.v1",
		IdempotencyKey: uuid.New(), RequestHash: hash, Request: request,
		Status: StatusRunning, Total: 1, NextBatchSequence: 1,
		BatchSize: 1, Checkpoint: EmptyCheckpoint(), Version: 2,
		ActorUserID: userID, ActorSessionID: &sessionID, AuditID: uuid.New(),
		Reason: "rebuild after catalog change", CorrelationID: uuid.NewString(),
		AvailableAt: now, CreatedAt: now, UpdatedAt: now, StartedAt: &now,
		RequestedAction: "none",
	}
	return value
}
