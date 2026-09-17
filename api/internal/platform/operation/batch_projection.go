package operation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BatchProjection is the safe, immutable execution receipt exposed to an
// authorized operator. Checkpoints, lease tokens, and domain request payloads
// intentionally never cross the HTTP boundary.
type BatchProjection struct {
	OperationID uuid.UUID     `json:"operation_id"`
	Sequence    int64         `json:"sequence"`
	Processed   int64         `json:"processed"`
	Succeeded   int64         `json:"succeeded"`
	Failed      int64         `json:"failed"`
	Failures    []ItemFailure `json:"failures"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
}

type BatchProjectionPage struct {
	Items      []BatchProjection `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type BatchListFilter struct {
	BeforeSequence int64
	Limit          int
}

// ValidateBatchListFilter is shared by transports and durable readers.
func ValidateBatchListFilter(filter BatchListFilter) error {
	if filter.BeforeSequence < 0 || filter.Limit < 1 || filter.Limit > 100 {
		return ErrInvalidOperation
	}
	return nil
}

type OperationBatchReader interface {
	ListBatches(context.Context, uuid.UUID, BatchListFilter) (BatchProjectionPage, error)
}

func validateBatchProjection(value BatchProjection) error {
	if value.OperationID == uuid.Nil || value.Sequence < 1 ||
		value.Processed < 0 || value.Succeeded < 0 || value.Failed < 0 ||
		value.Processed != value.Succeeded+value.Failed ||
		len(value.Failures) > MaxFailureSummaryItems || value.StartedAt.IsZero() ||
		value.CompletedAt.IsZero() || value.CompletedAt.Before(value.StartedAt) {
		return ErrInvalidProjection
	}
	for _, failure := range value.Failures {
		if !validText(failure.Reference, MaxFailureReferenceBytes) ||
			!validIdentifier(failure.Code, MaxFailureCodeBytes) {
			return ErrInvalidProjection
		}
	}
	return nil
}
