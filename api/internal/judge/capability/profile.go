// Package capability owns worker-local capability proof and exact tuple
// matching. It has no Docker or Ent dependency.
package capability

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidCapability = errors.New("judge capability: invalid exact capability")
	ErrCapabilityDenied  = errors.New("judge capability: activation denied")
)

const (
	CurrentJudgeSpecProtocol  = "judge-spec.v1"
	CurrentSupervisorProtocol = "sandbox-supervisor.v1"
)

var (
	capabilityIdentifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	capabilityDigestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type CapabilityTuple struct {
	JudgeSpecProtocol             string
	RuntimeKey                    string
	RuntimeProfileVersion         string
	CompilerVersion               string
	RuntimeBootstrapVersion       string
	CompileCommandID              string
	RunCommandID                  string
	ImageDigest                   string
	ManifestVersion               string
	ManifestChecksum              string
	SupervisorProtocol            string
	CheckerKey                    string
	CheckerVersion                string
	CheckerImplementationChecksum string
	CapabilityProfileHash         string
}

type CapabilityEvidence struct {
	Tuple                         CapabilityTuple
	BootID                        uuid.UUID
	CheckerImplementationChecksum string
	SmokeChecksum                 string
	GoldenChecksum                string
	ProbedAt                      time.Time
	Passing                       bool
}

// CapabilityAdvertisement intentionally has one evidence tuple. The legacy
// list/range fields exist only so boundary decoders can reject list-style and
// range-style advertisements instead of expanding them into a Cartesian set.
type CapabilityAdvertisement struct {
	Evidence               CapabilityEvidence
	RuntimeKeys            []string
	RuntimeProfileVersions []string
	CheckerKeys            []string
	CheckerVersions        []string
	RuntimeVersionRange    string
	CheckerVersionRange    string
}

type WorkerInstance struct {
	ID            uuid.UUID
	WorkerKey     string
	BootID        uuid.UUID
	StartedAt     time.Time
	HeartbeatAt   time.Time
	ClaimsEnabled bool
	TotalCapacity int
	FreeCapacity  int
}

func NewWorkerInstance(id uuid.UUID, workerKey string, bootID uuid.UUID, startedAt time.Time) WorkerInstance {
	return WorkerInstance{
		ID: id, WorkerKey: workerKey, BootID: bootID,
		StartedAt: startedAt.UTC(), HeartbeatAt: startedAt.UTC(),
		ClaimsEnabled: false, TotalCapacity: 1, FreeCapacity: 0,
	}
}

// ValidatePassingWorker validates a durable worker boot identity without
// granting any claim eligibility.
func ValidatePassingWorker(worker WorkerInstance) error {
	if !validWorker(worker) {
		return ErrInvalidCapability
	}
	return nil
}

// ValidatePassingEvidence validates the exact row accepted by durable stores.
// Freshness and catalog membership remain registry policy; structural and
// boot binding checks are repeated here so direct adapter use fails closed.
func ValidatePassingEvidence(worker WorkerInstance, evidence CapabilityEvidence) error {
	if ValidatePassingWorker(worker) != nil || !evidence.Passing ||
		evidence.BootID != worker.BootID || evidence.ProbedAt.IsZero() ||
		evidence.ProbedAt.Before(worker.StartedAt) ||
		!capabilityDigestPattern.MatchString(evidence.CheckerImplementationChecksum) ||
		evidence.CheckerImplementationChecksum != evidence.Tuple.CheckerImplementationChecksum ||
		!capabilityDigestPattern.MatchString(evidence.SmokeChecksum) ||
		!capabilityDigestPattern.MatchString(evidence.GoldenChecksum) ||
		validateCapabilityTuple(evidence.Tuple) != nil {
		return ErrInvalidCapability
	}
	return nil
}

// ComputeCapabilityProfileHash content-addresses one complete compatibility
// row. The hash field itself is excluded from the digest.
func ComputeCapabilityProfileHash(tuple CapabilityTuple) (string, error) {
	if err := validateTupleComponents(tuple, false); err != nil {
		return "", err
	}
	digest := sha256.New()
	writeHashPart(digest, "judgify.capability-profile.v3")
	writeHashPart(digest, tuple.JudgeSpecProtocol)
	writeHashPart(digest, tuple.RuntimeKey)
	writeHashPart(digest, tuple.RuntimeProfileVersion)
	writeHashPart(digest, tuple.CompilerVersion)
	writeHashPart(digest, tuple.RuntimeBootstrapVersion)
	writeHashPart(digest, tuple.CompileCommandID)
	writeHashPart(digest, tuple.RunCommandID)
	writeHashPart(digest, tuple.ImageDigest)
	writeHashPart(digest, tuple.ManifestVersion)
	writeHashPart(digest, tuple.ManifestChecksum)
	writeHashPart(digest, tuple.SupervisorProtocol)
	writeHashPart(digest, tuple.CheckerKey)
	writeHashPart(digest, tuple.CheckerVersion)
	writeHashPart(digest, tuple.CheckerImplementationChecksum)
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), nil
}

func validateCapabilityTuple(tuple CapabilityTuple) error {
	if err := validateTupleComponents(tuple, true); err != nil {
		return err
	}
	expected, err := ComputeCapabilityProfileHash(tuple)
	if err != nil || tuple.CapabilityProfileHash != expected {
		return ErrInvalidCapability
	}
	return nil
}

func validateTupleComponents(tuple CapabilityTuple, requireHash bool) error {
	identifiers := [...]string{
		tuple.JudgeSpecProtocol,
		tuple.RuntimeKey,
		tuple.RuntimeProfileVersion,
		tuple.CompilerVersion,
		tuple.RuntimeBootstrapVersion,
		tuple.CompileCommandID,
		tuple.RunCommandID,
		tuple.ManifestVersion,
		tuple.SupervisorProtocol,
		tuple.CheckerKey,
		tuple.CheckerVersion,
	}
	for _, value := range identifiers {
		if !capabilityIdentifierPattern.MatchString(value) {
			return ErrInvalidCapability
		}
	}
	if !capabilityDigestPattern.MatchString(tuple.ImageDigest) ||
		!capabilityDigestPattern.MatchString(tuple.ManifestChecksum) ||
		!capabilityDigestPattern.MatchString(tuple.CheckerImplementationChecksum) ||
		(requireHash && !capabilityDigestPattern.MatchString(tuple.CapabilityProfileHash)) {
		return ErrInvalidCapability
	}
	return nil
}

func writeHashPart(digest hash.Hash, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}
