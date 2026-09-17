package operation

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidProjection = errors.New("operation: invalid safe projection")

// Projection is the only operation representation allowed across an HTTP
// boundary or inside a control receipt. It deliberately excludes the request,
// checkpoint, idempotency key, reason, session, lease owner, and lease token.
type Projection struct {
	ID              uuid.UUID        `json:"id"`
	Kind            string           `json:"kind"`
	Status          Status           `json:"status"`
	Total           int64            `json:"total"`
	Processed       int64            `json:"processed"`
	Succeeded       int64            `json:"succeeded"`
	Failed          int64            `json:"failed"`
	BatchSize       int              `json:"batch_size"`
	Version         int64            `json:"version"`
	Lease           *LeaseProjection `json:"lease,omitempty"`
	RequestedAction string           `json:"requested_action"`
	AttemptCount    uint32           `json:"attempt_count"`
	LastErrorCode   string           `json:"last_error_code,omitempty"`
	ActorUserID     uuid.UUID        `json:"actor_user_id"`
	AuditID         uuid.UUID        `json:"audit_id"`
	CorrelationID   string           `json:"correlation_id"`
	NotBefore       *time.Time       `json:"not_before,omitempty"`
	AvailableAt     time.Time        `json:"available_at"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	StartedAt       *time.Time       `json:"started_at,omitempty"`
	FinishedAt      *time.Time       `json:"finished_at,omitempty"`
}

type LeaseProjection struct {
	OperationID uuid.UUID `json:"operation_id"`
	Until       time.Time `json:"until"`
}

type ProjectionPage struct {
	Items      []Projection `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

func Project(value Operation) (Projection, error) {
	if err := Validate(value); err != nil {
		return Projection{}, errors.Join(ErrInvalidProjection, err)
	}
	projected := Projection{
		ID: value.ID, Kind: value.Kind, Status: value.Status,
		Total: value.Total, Processed: value.Processed,
		Succeeded: value.Succeeded, Failed: value.Failed,
		BatchSize: value.BatchSize, Version: value.Version,
		RequestedAction: value.RequestedAction,
		AttemptCount:    value.AttemptCount, LastErrorCode: value.LastErrorCode,
		ActorUserID: value.ActorUserID, AuditID: value.AuditID,
		CorrelationID: value.CorrelationID,
		NotBefore:     cloneTime(value.NotBefore),
		AvailableAt:   value.AvailableAt.UTC(),
		CreatedAt:     value.CreatedAt.UTC(), UpdatedAt: value.UpdatedAt.UTC(),
		StartedAt: cloneTime(value.StartedAt), FinishedAt: cloneTime(value.FinishedAt),
	}
	if value.Lease != nil {
		projected.Lease = &LeaseProjection{
			OperationID: value.ID,
			Until:       value.Lease.Until.UTC(),
		}
	}
	if err := ValidateProjection(projected); err != nil {
		return Projection{}, err
	}
	return projected, nil
}

func ProjectPage(value Page) (ProjectionPage, error) {
	items := make([]Projection, 0, len(value.Items))
	for _, operation := range value.Items {
		projected, err := Project(operation)
		if err != nil {
			return ProjectionPage{}, err
		}
		items = append(items, projected)
	}
	return ProjectionPage{
		Items:      items,
		NextCursor: value.NextCursor,
	}, nil
}

func ValidateProjection(value Projection) error {
	if value.ID == uuid.Nil || value.ActorUserID == uuid.Nil ||
		value.AuditID == uuid.Nil ||
		!validIdentifier(value.Kind, MaxKindBytes) ||
		!validStatus(value.Status) ||
		value.Total < 0 || value.Processed < 0 ||
		value.Succeeded < 0 || value.Failed < 0 ||
		value.Processed != value.Succeeded+value.Failed ||
		value.Processed > value.Total ||
		value.BatchSize < 1 || value.BatchSize > MaxBatchSize ||
		value.Version < 1 ||
		!validText(value.CorrelationID, MaxCorrelationIDBytes) ||
		!validNotBefore(value.NotBefore) ||
		value.AvailableAt.IsZero() || value.CreatedAt.IsZero() ||
		value.UpdatedAt.IsZero() ||
		(value.LastErrorCode != "" &&
			!validIdentifier(value.LastErrorCode, MaxFailureCodeBytes)) {
		return ErrInvalidProjection
	}
	switch value.RequestedAction {
	case "none":
	case "pause", "cancel":
		if value.Status != StatusRunning {
			return ErrInvalidProjection
		}
	default:
		return ErrInvalidProjection
	}
	if value.Lease != nil &&
		(value.Status != StatusRunning ||
			value.Lease.OperationID != value.ID ||
			value.Lease.Until.IsZero()) {
		return ErrInvalidProjection
	}
	if value.Status != StatusRunning && value.Lease != nil {
		return ErrInvalidProjection
	}
	if value.Status == StatusCompleted && value.Processed != value.Total {
		return ErrInvalidOperation
	}
	return nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func validateProjectionMatches(
	value Projection,
	operationID uuid.UUID,
	version int64,
	status Status,
) error {
	if err := ValidateProjection(value); err != nil ||
		value.ID != operationID || value.Version != version ||
		value.Status != status {
		return errors.Join(ErrInvalidProjection, err)
	}
	return nil
}
