// Package operation owns the durable, domain-neutral control plane for
// bounded background work. Domains own selection and mutation semantics and
// register typed executors at process startup; this package persists only
// canonical requests, leases, checkpoints, progress, and safe failure codes.
package operation

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxKindBytes             = 128
	MaxReasonBytes           = 1024
	MaxCorrelationIDBytes    = 128
	MaxRequestBytes          = 64 << 10
	MaxCheckpointBytes       = 8 << 10
	MaxFailureSummaryItems   = 100
	MaxFailureReferenceBytes = 128
	MaxFailureCodeBytes      = 64
	MaxBatchSize             = 1000
)

var (
	ErrInvalidOperation   = errors.New("operation: invalid operation")
	ErrInvalidRequest     = errors.New("operation: invalid canonical request")
	ErrInvalidCheckpoint  = errors.New("operation: invalid checkpoint")
	ErrInvalidBatchResult = errors.New("operation: invalid batch result")
	ErrConflict           = errors.New("operation: conflict")
	ErrNotFound           = errors.New("operation: not found")
	ErrLeaseLost          = errors.New("operation: lease lost")
	ErrExecutorNotFound   = errors.New("operation: executor not registered")

	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type ControlAction string

const (
	ControlPause  ControlAction = "pause"
	ControlResume ControlAction = "resume"
	ControlCancel ControlAction = "cancel"
)

type Lease struct {
	OperationID uuid.UUID `json:"operation_id"`
	Token       uuid.UUID `json:"-"`
	Owner       string    `json:"-"`
	Until       time.Time `json:"until"`
}

// ItemFailure is deliberately small and safe to expose to an authorized
// operator. Reference is a domain-owned opaque identifier, never raw content.
type ItemFailure struct {
	Reference string `json:"reference"`
	Code      string `json:"code"`
}

type Operation struct {
	ID                uuid.UUID         `json:"id"`
	Kind              string            `json:"kind"`
	IdempotencyKey    uuid.UUID         `json:"-"`
	RequestHash       [sha256.Size]byte `json:"-"`
	Request           json.RawMessage   `json:"-"`
	Status            Status            `json:"status"`
	Total             int64             `json:"total"`
	Processed         int64             `json:"processed"`
	Succeeded         int64             `json:"succeeded"`
	Failed            int64             `json:"failed"`
	NextBatchSequence int64             `json:"-"`
	BatchSize         int               `json:"batch_size"`
	Checkpoint        json.RawMessage   `json:"-"`
	Version           int64             `json:"version"`
	Lease             *Lease            `json:"lease,omitempty"`
	RequestedAction   string            `json:"requested_action"`
	AttemptCount      uint32            `json:"attempt_count"`
	LastErrorCode     string            `json:"last_error_code,omitempty"`
	ActorUserID       uuid.UUID         `json:"actor_user_id"`
	ActorSessionID    *uuid.UUID        `json:"-"`
	AuditID           uuid.UUID         `json:"audit_id"`
	Reason            string            `json:"-"`
	CorrelationID     string            `json:"correlation_id"`
	NotBefore         *time.Time        `json:"not_before,omitempty"`
	AvailableAt       time.Time         `json:"available_at"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	FinishedAt        *time.Time        `json:"finished_at,omitempty"`
}

type CreateCommand struct {
	ID             uuid.UUID
	Kind           string
	IdempotencyKey uuid.UUID
	RequestHash    [sha256.Size]byte
	Request        json.RawMessage
	Total          int64
	BatchSize      int
	Checkpoint     json.RawMessage
	ActorUserID    uuid.UUID
	ActorSessionID *uuid.UUID
	AuditID        uuid.UUID
	Reason         string
	CorrelationID  string
	NotBefore      *time.Time
}

type CreateResult struct {
	Operation Operation
	Replay    bool
}

type ClaimCommand struct {
	Owner         string
	LeaseDuration time.Duration
}

type BatchInput struct {
	OperationID uuid.UUID
	Sequence    int64
	Request     json.RawMessage
	Checkpoint  json.RawMessage
	Limit       int
}

type BatchResult struct {
	Processed      int64
	Succeeded      int64
	Failed         int64
	NextCheckpoint json.RawMessage
	Failures       []ItemFailure
	Done           bool
	// FailureCode makes a fully checkpointed operation terminal-failed. It is
	// useful when item-level failures must be retained rather than rolled back.
	FailureCode string
}

type RetryDirective struct {
	Code        string
	AvailableAt time.Time
}

type ControlCommand struct {
	OperationID     uuid.UUID
	Action          ControlAction
	ExpectedVersion int64
	ActorUserID     uuid.UUID
	ActorSessionID  uuid.UUID
	IdempotencyKey  uuid.UUID
	Reason          string
}

type ControlResult struct {
	Operation Projection `json:"operation"`
	AuditID   uuid.UUID  `json:"audit_id"`
	Replay    bool       `json:"replay"`
}

type ListFilter struct {
	Status   Status
	Kind     string
	ActorID  uuid.UUID
	BeforeAt time.Time
	BeforeID uuid.UUID
	Limit    int
}

type Page struct {
	Items      []Operation `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

// ValidateListFilter keeps transport adapters and repositories on one
// canonical validation contract.
func ValidateListFilter(filter ListFilter) error {
	if filter.Limit < 1 || filter.Limit > 100 ||
		(filter.Status != "" && !validStatus(filter.Status)) ||
		(filter.Kind != "" && !validIdentifier(filter.Kind, MaxKindBytes)) ||
		((filter.BeforeAt.IsZero()) != (filter.BeforeID == uuid.Nil)) {
		return ErrInvalidOperation
	}
	return nil
}

// EncodeRequest creates the only request representation accepted by Create.
// Struct-backed application commands therefore have one stable persisted byte
// form and one exact replay hash.
func EncodeRequest(value any) (json.RawMessage, [sha256.Size]byte, error) {
	if value == nil {
		return nil, [sha256.Size]byte{}, ErrInvalidRequest
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, [sha256.Size]byte{}, errors.Join(ErrInvalidRequest, err)
	}
	if err := validateJSONObject(payload, MaxRequestBytes); err != nil {
		return nil, [sha256.Size]byte{}, errors.Join(ErrInvalidRequest, err)
	}
	return append(json.RawMessage(nil), payload...), sha256.Sum256(payload), nil
}

func EmptyCheckpoint() json.RawMessage {
	return json.RawMessage(`{}`)
}

func ValidateCreate(command CreateCommand) error {
	if command.ID == uuid.Nil || command.IdempotencyKey == uuid.Nil ||
		command.ActorUserID == uuid.Nil || command.AuditID == uuid.Nil {
		return ErrInvalidOperation
	}
	return ValidateCreateIntent(command)
}

// ValidateCreateIntent validates every replay-bound input before an audit ID
// exists. LockStart uses this exact tuple to serialize concurrent retries.
func ValidateCreateIntent(command CreateCommand) error {
	if command.ID == uuid.Nil || command.IdempotencyKey == uuid.Nil ||
		command.ActorUserID == uuid.Nil ||
		!validIdentifier(command.Kind, MaxKindBytes) ||
		!validText(command.Reason, MaxReasonBytes) ||
		!validText(command.CorrelationID, MaxCorrelationIDBytes) ||
		command.Total < 0 || command.BatchSize < 1 || command.BatchSize > MaxBatchSize ||
		command.RequestHash == ([sha256.Size]byte{}) ||
		(command.ActorSessionID != nil && *command.ActorSessionID == uuid.Nil) ||
		!validNotBefore(command.NotBefore) {
		return ErrInvalidOperation
	}
	if err := validateJSONObject(command.Request, MaxRequestBytes); err != nil {
		return errors.Join(ErrInvalidRequest, err)
	}
	if sha256.Sum256(command.Request) != command.RequestHash {
		return ErrInvalidRequest
	}
	if err := validateJSONObject(command.Checkpoint, MaxCheckpointBytes); err != nil {
		return err
	}
	return nil
}

func ValidateBatchResult(current Operation, result BatchResult) error {
	if err := Validate(current); err != nil ||
		result.Processed < 0 || result.Succeeded < 0 || result.Failed < 0 ||
		result.Processed != result.Succeeded+result.Failed ||
		result.Processed > int64(current.BatchSize) ||
		current.Processed > current.Total-result.Processed ||
		len(result.Failures) > MaxFailureSummaryItems ||
		(result.Done && current.Processed+result.Processed != current.Total) ||
		(result.FailureCode != "" &&
			(!result.Done || !validIdentifier(result.FailureCode, MaxFailureCodeBytes))) ||
		(!result.Done && result.Processed == 0) {
		return ErrInvalidBatchResult
	}
	if err := validateJSONObject(result.NextCheckpoint, MaxCheckpointBytes); err != nil {
		return ErrInvalidBatchResult
	}
	for _, failure := range result.Failures {
		if !validText(failure.Reference, MaxFailureReferenceBytes) ||
			!validIdentifier(failure.Code, MaxFailureCodeBytes) {
			return ErrInvalidBatchResult
		}
	}
	return nil
}

func Validate(value Operation) error {
	if value.ID == uuid.Nil || value.IdempotencyKey == uuid.Nil ||
		value.ActorUserID == uuid.Nil || value.AuditID == uuid.Nil ||
		!validIdentifier(value.Kind, MaxKindBytes) ||
		value.RequestHash == ([sha256.Size]byte{}) ||
		sha256.Sum256(value.Request) != value.RequestHash ||
		value.Total < 0 || value.Processed < 0 || value.Succeeded < 0 ||
		value.Failed < 0 || value.Processed != value.Succeeded+value.Failed ||
		value.Processed > value.Total || value.NextBatchSequence < 1 ||
		value.BatchSize < 1 || value.BatchSize > MaxBatchSize ||
		value.Version < 1 || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() ||
		!validStatus(value.Status) || !validText(value.Reason, MaxReasonBytes) ||
		!validText(value.CorrelationID, MaxCorrelationIDBytes) ||
		!validNotBefore(value.NotBefore) || value.AvailableAt.IsZero() {
		return ErrInvalidOperation
	}
	if err := validateJSONObject(value.Request, MaxRequestBytes); err != nil {
		return ErrInvalidOperation
	}
	if err := validateJSONObject(value.Checkpoint, MaxCheckpointBytes); err != nil {
		return ErrInvalidOperation
	}
	if value.Lease != nil {
		if value.Status != StatusRunning || value.Lease.OperationID != value.ID ||
			value.Lease.Token == uuid.Nil || !validIdentifier(value.Lease.Owner, 128) ||
			value.Lease.Until.IsZero() {
			return ErrInvalidOperation
		}
	}
	switch value.RequestedAction {
	case "none", "pause", "cancel":
	default:
		return ErrInvalidOperation
	}
	if value.Status != StatusRunning && value.RequestedAction != "none" {
		return ErrInvalidOperation
	}
	if value.Status != StatusRunning && value.Lease != nil {
		return ErrInvalidOperation
	}
	if value.Status == StatusCompleted && value.Processed != value.Total {
		return ErrInvalidOperation
	}
	return nil
}

func validNotBefore(value *time.Time) bool {
	if value == nil {
		return true
	}
	if value.IsZero() || value.Location() != time.UTC {
		return false
	}
	minimum := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	maximum := time.Date(2200, time.January, 1, 0, 0, 0, 0, time.UTC)
	return !value.Before(minimum) && value.Before(maximum)
}

func ControlRequestHash(command ControlCommand) ([sha256.Size]byte, error) {
	if command.OperationID == uuid.Nil || command.ExpectedVersion < 1 ||
		command.ActorUserID == uuid.Nil || command.ActorSessionID == uuid.Nil ||
		command.IdempotencyKey == uuid.Nil ||
		!validText(command.Reason, MaxReasonBytes) {
		return [sha256.Size]byte{}, ErrInvalidOperation
	}
	switch command.Action {
	case ControlPause, ControlResume, ControlCancel:
	default:
		return [sha256.Size]byte{}, ErrInvalidOperation
	}
	payload, err := json.Marshal(struct {
		OperationID     string        `json:"operation_id"`
		Action          ControlAction `json:"action"`
		ExpectedVersion int64         `json:"expected_version"`
		ActorUserID     string        `json:"actor_user_id"`
		ActorSessionID  string        `json:"actor_session_id"`
		Reason          string        `json:"reason"`
	}{
		OperationID: command.OperationID.String(), Action: command.Action,
		ExpectedVersion: command.ExpectedVersion,
		ActorUserID:     command.ActorUserID.String(),
		ActorSessionID:  command.ActorSessionID.String(),
		Reason:          command.Reason,
	})
	if err != nil {
		return [sha256.Size]byte{}, errors.Join(ErrInvalidOperation, err)
	}
	return sha256.Sum256(payload), nil
}

func validStatus(value Status) bool {
	switch value {
	case StatusPending, StatusRunning, StatusPaused, StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

func validIdentifier(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value && identifierPattern.MatchString(value)
}

func validText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value
}

func validateJSONObject(value []byte, maximum int) error {
	if len(value) < 2 || len(value) > maximum || !json.Valid(value) {
		return ErrInvalidCheckpoint
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil || object == nil {
		return ErrInvalidCheckpoint
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrInvalidCheckpoint
	}
	return nil
}
