package capability

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrPlatformActivationDenied = errors.New("judge capability: platform activation denied")
	// ErrActivationHasNonTerminalJobs is returned when an operator attempts to
	// retire a capability tuple that is still referenced by work which can be
	// claimed or is currently being processed. Retiring in that state would
	// strand the work because the claim fence intentionally requires an active
	// activation.
	ErrActivationHasNonTerminalJobs = errors.New(
		"judge capability: activation has non-terminal jobs",
	)
)

const maximumActivationReasonBytes = 500

type ActivationState string

const (
	ActivationStateActive  ActivationState = "active"
	ActivationStateRetired ActivationState = "retired"
)

// ActivationRequest is an explicit operator command. Worker boot and probe
// code do not own or construct this command.
type ActivationRequest struct {
	ReleaseID                       string
	ApprovedReleaseManifestChecksum string
	ActorID                         uuid.UUID
	AuditID                         uuid.UUID
	Reason                          string
	Tuple                           CapabilityTuple
}

// ActivatedCapability is the durable relation later intake reads before
// emitting a JudgeSpec. Proof fields are filled by the store from a fresh,
// claims-enabled exact worker row; they are never supplied by the operator.
type ActivatedCapability struct {
	ID                              uuid.UUID
	ReleaseID                       string
	ApprovedReleaseManifestChecksum string
	ActorID                         uuid.UUID
	AuditID                         uuid.UUID
	Reason                          string
	Tuple                           CapabilityTuple

	CheckerImplementationChecksum string
	SmokeChecksum                 string
	GoldenChecksum                string

	ProofWorkerInstanceID uuid.UUID
	ProofBootID           uuid.UUID
	ProofEvidenceID       uuid.UUID
	State                 ActivationState
	ActivatedAt           time.Time
	RetiredAt             *time.Time
}

type RetirementRequest struct {
	ActivationID uuid.UUID
	ActorID      uuid.UUID
	AuditID      uuid.UUID
	Reason       string
}

// ActivationStore atomically proves one fresh, claims-enabled exact worker and
// creates the platform activation. Implementations must order candidate
// workers by heartbeat time and evidence by probe time; UUID ordering may only
// be a deterministic tie-breaker.
type ActivationStore interface {
	CreateIfFreshWorkerProves(
		ctx context.Context,
		activation ActivatedCapability,
		notBefore time.Time,
	) (ActivatedCapability, error)
	Retire(ctx context.Context, request RetirementRequest, retiredAt time.Time) error
}

func (service *ActivationService) Retire(
	ctx context.Context,
	request RetirementRequest,
	retiredAt time.Time,
) error {
	if service == nil || service.store == nil || ctx == nil || ctx.Err() != nil ||
		retiredAt.IsZero() || ValidateRetirementRequest(request) != nil {
		return ErrPlatformActivationDenied
	}
	if err := service.store.Retire(ctx, request, retiredAt.UTC()); err != nil {
		return errors.Join(ErrPlatformActivationDenied, err)
	}
	return nil
}

// ValidateRetirementRequest is shared by the application service and durable
// adapters so a direct storage call cannot persist an unaudited command.
func ValidateRetirementRequest(request RetirementRequest) error {
	if request.ActivationID == uuid.Nil || request.ActorID == uuid.Nil ||
		request.AuditID == uuid.Nil || validateAuditReason(request.Reason) != nil {
		return ErrPlatformActivationDenied
	}
	return nil
}

// ActivationReader is the intentionally narrow later-intake boundary. A
// missing, ambiguous, retired, expired, or storage-error result must deny API
// emission rather than select an arbitrary row.
type ActivationReader interface {
	Active(
		ctx context.Context,
		releaseID string,
		tuple CapabilityTuple,
	) (ActivatedCapability, bool, error)
}

type ActivationService struct {
	store     ActivationStore
	freshness time.Duration
}

func NewActivationService(store ActivationStore, freshness time.Duration) (*ActivationService, error) {
	if store == nil || freshness <= 0 {
		return nil, ErrPlatformActivationDenied
	}
	return &ActivationService{store: store, freshness: freshness}, nil
}

func (service *ActivationService) Activate(
	ctx context.Context,
	request ActivationRequest,
	now time.Time,
) (ActivatedCapability, error) {
	if service == nil || service.store == nil || ctx == nil || ctx.Err() != nil ||
		now.IsZero() || ValidateActivationRequest(request) != nil {
		return ActivatedCapability{}, ErrPlatformActivationDenied
	}
	id, err := uuid.NewV7()
	if err != nil {
		return ActivatedCapability{}, errors.Join(ErrPlatformActivationDenied, err)
	}
	now = now.UTC()
	activation := ActivatedCapability{
		ID: id, ReleaseID: request.ReleaseID,
		ApprovedReleaseManifestChecksum: request.ApprovedReleaseManifestChecksum,
		ActorID:                         request.ActorID, AuditID: request.AuditID, Reason: request.Reason,
		Tuple: request.Tuple,
		State: ActivationStateActive, ActivatedAt: now,
	}
	created, err := service.store.CreateIfFreshWorkerProves(ctx, activation, now.Add(-service.freshness))
	if err != nil || !sameCreatedActivation(activation, created) {
		return ActivatedCapability{}, errors.Join(ErrPlatformActivationDenied, err)
	}
	return created, nil
}

// ValidateActivationRequest is shared by the application service and durable
// adapters so direct adapter use cannot bypass the command's fail-closed
// release, audit, and exact-tuple invariants.
func ValidateActivationRequest(request ActivationRequest) error {
	if !capabilityIdentifierPattern.MatchString(request.ReleaseID) ||
		!capabilityDigestPattern.MatchString(request.ApprovedReleaseManifestChecksum) ||
		request.ActorID == uuid.Nil || request.AuditID == uuid.Nil ||
		validateAuditReason(request.Reason) != nil ||
		validateCapabilityTuple(request.Tuple) != nil {
		return ErrPlatformActivationDenied
	}
	return nil
}

func validateAuditReason(reason string) error {
	trimmed := strings.TrimSpace(reason)
	if trimmed != reason || !utf8.ValidString(reason) || len(reason) == 0 ||
		len(reason) > maximumActivationReasonBytes {
		return ErrPlatformActivationDenied
	}
	return nil
}

func sameCreatedActivation(want ActivatedCapability, got ActivatedCapability) bool {
	return got.ID == want.ID && got.ReleaseID == want.ReleaseID &&
		got.ApprovedReleaseManifestChecksum == want.ApprovedReleaseManifestChecksum &&
		got.ActorID == want.ActorID && got.AuditID == want.AuditID &&
		got.Reason == want.Reason && got.Tuple == want.Tuple &&
		want.CheckerImplementationChecksum == "" && want.SmokeChecksum == "" &&
		want.GoldenChecksum == "" &&
		got.CheckerImplementationChecksum == got.Tuple.CheckerImplementationChecksum &&
		capabilityDigestPattern.MatchString(got.CheckerImplementationChecksum) &&
		capabilityDigestPattern.MatchString(got.SmokeChecksum) &&
		capabilityDigestPattern.MatchString(got.GoldenChecksum) &&
		got.State == ActivationStateActive && got.ActivatedAt.Equal(want.ActivatedAt) &&
		got.RetiredAt == nil && got.ProofWorkerInstanceID != uuid.Nil &&
		got.ProofBootID != uuid.Nil && got.ProofEvidenceID != uuid.Nil
}
