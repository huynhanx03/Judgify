// Package releasemanifest builds and verifies the finite, reviewed judge
// release artifact used by both API and worker startup. It is operator-only:
// no request handler or mutable database row can mint executable identities.
package releasemanifest

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
	"github.com/huynhanx03/judgify/internal/judge/capability/inspector"
	"github.com/huynhanx03/judgify/internal/judge/runtimecatalog"
	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

const (
	DraftSchemaVersion  = 1
	maximumCapabilities = 64
)

var (
	ErrInvalidDraft    = errors.New("release manifest authoring: invalid draft")
	ErrInvalidApproval = errors.New("release manifest authoring: invalid approval")
	digestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type RuntimeEvidence struct {
	RuntimeKey              string `json:"runtime_key"`
	ProfileVersion          string `json:"profile_version"`
	CompilerVersion         string `json:"compiler_version"`
	RuntimeBootstrapVersion string `json:"runtime_bootstrap_version"`
	CompileCommandID        string `json:"compile_command_id"`
	RunCommandID            string `json:"run_command_id"`
	ManifestVersion         string `json:"manifest_version"`
	ManifestChecksum        string `json:"manifest_checksum"`
}

type CheckerEvidence struct {
	CheckerKey             string `json:"checker_key"`
	Version                string `json:"version"`
	ImplementationChecksum string `json:"implementation_checksum"`
}

// CapabilitySelection is intentionally one explicit runtime/checker pairing.
// The authoring path never infers a Cartesian product.
type CapabilitySelection struct {
	RuntimeKey string `json:"runtime_key"`
	CheckerKey string `json:"checker_key"`
}

type Draft struct {
	SchemaVersion      int                   `json:"schema_version"`
	ReleaseID          string                `json:"release_id"`
	JudgeSpecProtocol  string                `json:"judge_spec_protocol"`
	SupervisorProtocol string                `json:"supervisor_protocol"`
	Runtimes           []RuntimeEvidence     `json:"runtimes"`
	Checkers           []CheckerEvidence     `json:"checkers"`
	Capabilities       []CapabilitySelection `json:"capabilities"`
}

type Approval struct {
	SandboxImageDigest string
	ApprovedBy         uuid.UUID
	AuditID            uuid.UUID
	ApprovedAt         time.Time
}

type Summary struct {
	ReleaseID          string    `json:"release_id"`
	ApprovalChecksum   string    `json:"approval_checksum"`
	ApprovedBy         uuid.UUID `json:"approved_by"`
	AuditID            uuid.UUID `json:"audit_id"`
	ApprovedAt         time.Time `json:"approved_at"`
	SandboxImageDigest string    `json:"sandbox_image_digest"`
	RuntimeCount       int       `json:"runtime_count"`
	CheckerCount       int       `json:"checker_count"`
	CapabilityCount    int       `json:"capability_count"`
}

func NewDraft(releaseID string) (Draft, error) {
	template, err := problemservice.ReleaseManifestTemplate(releaseID)
	if err != nil {
		return Draft{}, errors.Join(ErrInvalidDraft, err)
	}
	draft := Draft{
		SchemaVersion: DraftSchemaVersion, ReleaseID: template.ReleaseID,
		JudgeSpecProtocol:  capability.CurrentJudgeSpecProtocol,
		SupervisorProtocol: capability.CurrentSupervisorProtocol,
		Runtimes:           make([]RuntimeEvidence, 0, len(template.Runtimes)),
		Checkers:           make([]CheckerEvidence, 0, len(template.Checkers)),
		Capabilities:       []CapabilitySelection{},
	}
	for _, runtime := range template.Runtimes {
		draft.Runtimes = append(draft.Runtimes, runtimeEvidence(runtime))
	}
	for _, checker := range template.Checkers {
		evidence, evidenceErr := inspector.ComputeCheckerSmoke(inspector.CheckerSmokeRequest{
			CheckerKey:         checker.CheckerKey,
			Version:            checker.Version,
			SupervisorProtocol: capability.CurrentSupervisorProtocol,
		})
		if evidenceErr != nil {
			return Draft{}, errors.Join(ErrInvalidDraft, evidenceErr)
		}
		draft.Checkers = append(draft.Checkers, CheckerEvidence{
			CheckerKey: checker.CheckerKey, Version: checker.Version,
			ImplementationChecksum: evidence.CheckerImplementationChecksum,
		})
	}
	canonicalizeDraft(&draft)
	return draft, nil
}

func BuildApproved(
	ctx context.Context,
	draft Draft,
	approval Approval,
) (problemservice.ApprovedReleaseManifest, Summary, error) {
	template, runtimes, checkers, err := validateDraft(draft, true)
	if err != nil {
		return problemservice.ApprovedReleaseManifest{}, Summary{}, err
	}
	if ctx == nil || ctx.Err() != nil || approval.ApprovedBy == uuid.Nil ||
		approval.AuditID == uuid.Nil || approval.ApprovedAt.IsZero() ||
		!validDigest(approval.SandboxImageDigest) {
		return problemservice.ApprovedReleaseManifest{}, Summary{}, ErrInvalidApproval
	}

	for index := range template.Runtimes {
		template.Runtimes[index].ImageDigest = approval.SandboxImageDigest
	}
	for index := range template.Checkers {
		template.Checkers[index].ImplementationChecksum =
			checkers[template.Checkers[index].CheckerKey].ImplementationChecksum
	}

	seen := make(map[string]struct{}, len(draft.Capabilities))
	template.Capabilities = make([]problemservice.CapabilityRelease, 0, len(draft.Capabilities))
	for _, selected := range draft.Capabilities {
		pair := selected.RuntimeKey + "\x00" + selected.CheckerKey
		if _, duplicate := seen[pair]; duplicate {
			return problemservice.ApprovedReleaseManifest{}, Summary{}, ErrInvalidDraft
		}
		seen[pair] = struct{}{}
		runtime, runtimeExists := runtimes[selected.RuntimeKey]
		checker, checkerExists := checkers[selected.CheckerKey]
		if !runtimeExists || !checkerExists {
			return problemservice.ApprovedReleaseManifest{}, Summary{}, ErrInvalidDraft
		}
		tuple := capability.CapabilityTuple{
			JudgeSpecProtocol:             draft.JudgeSpecProtocol,
			RuntimeKey:                    runtime.RuntimeKey,
			RuntimeProfileVersion:         runtime.ProfileVersion,
			CompilerVersion:               runtime.CompilerVersion,
			RuntimeBootstrapVersion:       runtime.RuntimeBootstrapVersion,
			CompileCommandID:              runtime.CompileCommandID,
			RunCommandID:                  runtime.RunCommandID,
			ImageDigest:                   approval.SandboxImageDigest,
			ManifestVersion:               runtime.ManifestVersion,
			ManifestChecksum:              runtime.ManifestChecksum,
			SupervisorProtocol:            draft.SupervisorProtocol,
			CheckerKey:                    checker.CheckerKey,
			CheckerVersion:                checker.Version,
			CheckerImplementationChecksum: checker.ImplementationChecksum,
		}
		tuple.CapabilityProfileHash, err = capability.ComputeCapabilityProfileHash(tuple)
		if err != nil {
			return problemservice.ApprovedReleaseManifest{}, Summary{}, errors.Join(
				ErrInvalidDraft,
				err,
			)
		}
		template.Capabilities = append(template.Capabilities, capabilityRelease(tuple))
	}

	template.Approved = true
	template.ApprovedBy = approval.ApprovedBy
	template.AuditID = approval.AuditID
	template.ApprovedAt = approval.ApprovedAt.UTC()
	canonicalizeManifest(&template)
	template.ApprovalChecksum, err = problemservice.ComputeReleaseManifestChecksum(template)
	if err != nil {
		return problemservice.ApprovedReleaseManifest{}, Summary{}, errors.Join(
			ErrInvalidApproval,
			err,
		)
	}
	catalog, err := runtimecatalog.LoadApprovedRelease(template)
	if err != nil || catalog.ReleaseID() != template.ReleaseID ||
		len(catalog.ApprovedTuples()) != len(template.Capabilities) {
		return problemservice.ApprovedReleaseManifest{}, Summary{}, errors.Join(
			ErrInvalidApproval,
			err,
		)
	}
	return template, summary(template, approval.SandboxImageDigest), nil
}

func Verify(
	ctx context.Context,
	manifest problemservice.ApprovedReleaseManifest,
	expectedSandboxImageDigest string,
) (Summary, error) {
	if ctx == nil || ctx.Err() != nil || !validDigest(expectedSandboxImageDigest) {
		return Summary{}, ErrInvalidApproval
	}
	catalog, err := runtimecatalog.LoadApprovedRelease(manifest)
	if err != nil || catalog.ReleaseID() != manifest.ReleaseID {
		return Summary{}, errors.Join(ErrInvalidApproval, err)
	}
	for _, runtime := range manifest.Runtimes {
		if runtime.ImageDigest != expectedSandboxImageDigest {
			return Summary{}, ErrInvalidApproval
		}
	}
	if len(catalog.ApprovedTuples()) != len(manifest.Capabilities) {
		return Summary{}, ErrInvalidApproval
	}
	return summary(manifest, expectedSandboxImageDigest), nil
}

func ValidateDraft(draft Draft) error {
	_, _, _, err := validateDraft(draft, false)
	return err
}

func validateDraft(
	draft Draft,
	requireCapabilities bool,
) (
	problemservice.ApprovedReleaseManifest,
	map[string]RuntimeEvidence,
	map[string]CheckerEvidence,
	error,
) {
	if draft.SchemaVersion != DraftSchemaVersion ||
		draft.JudgeSpecProtocol != capability.CurrentJudgeSpecProtocol ||
		draft.SupervisorProtocol != capability.CurrentSupervisorProtocol ||
		len(draft.Capabilities) > maximumCapabilities ||
		(requireCapabilities && len(draft.Capabilities) == 0) {
		return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
	}
	template, err := problemservice.ReleaseManifestTemplate(draft.ReleaseID)
	if err != nil || len(draft.Runtimes) != len(template.Runtimes) ||
		len(draft.Checkers) != len(template.Checkers) {
		return problemservice.ApprovedReleaseManifest{}, nil, nil, errors.Join(
			ErrInvalidDraft,
			err,
		)
	}

	expectedRuntimes := make(map[string]RuntimeEvidence, len(template.Runtimes))
	for _, runtime := range template.Runtimes {
		expectedRuntimes[runtime.RuntimeKey] = runtimeEvidence(runtime)
	}
	runtimes := make(map[string]RuntimeEvidence, len(draft.Runtimes))
	for _, runtime := range draft.Runtimes {
		expected, exists := expectedRuntimes[runtime.RuntimeKey]
		if !exists || runtime != expected {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		if _, duplicate := runtimes[runtime.RuntimeKey]; duplicate {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		runtimes[runtime.RuntimeKey] = runtime
	}

	expectedCheckers := make(map[string]CheckerEvidence, len(template.Checkers))
	for _, checker := range template.Checkers {
		evidence, evidenceErr := inspector.ComputeCheckerSmoke(inspector.CheckerSmokeRequest{
			CheckerKey:         checker.CheckerKey,
			Version:            checker.Version,
			SupervisorProtocol: draft.SupervisorProtocol,
		})
		if evidenceErr != nil {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, errors.Join(
				ErrInvalidDraft,
				evidenceErr,
			)
		}
		expectedCheckers[checker.CheckerKey] = CheckerEvidence{
			CheckerKey: checker.CheckerKey, Version: checker.Version,
			ImplementationChecksum: evidence.CheckerImplementationChecksum,
		}
	}
	checkers := make(map[string]CheckerEvidence, len(draft.Checkers))
	for _, checker := range draft.Checkers {
		expected, exists := expectedCheckers[checker.CheckerKey]
		if !exists || checker != expected {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		if _, duplicate := checkers[checker.CheckerKey]; duplicate {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		checkers[checker.CheckerKey] = checker
	}

	selected := make(map[string]struct{}, len(draft.Capabilities))
	for _, pair := range draft.Capabilities {
		identity := pair.RuntimeKey + "\x00" + pair.CheckerKey
		if _, runtimeExists := runtimes[pair.RuntimeKey]; !runtimeExists {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		if _, checkerExists := checkers[pair.CheckerKey]; !checkerExists {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		if _, duplicate := selected[identity]; duplicate {
			return problemservice.ApprovedReleaseManifest{}, nil, nil, ErrInvalidDraft
		}
		selected[identity] = struct{}{}
	}
	return template, runtimes, checkers, nil
}

func runtimeEvidence(runtime problemservice.RuntimeRelease) RuntimeEvidence {
	return RuntimeEvidence{
		RuntimeKey: runtime.RuntimeKey, ProfileVersion: runtime.ProfileVersion,
		CompilerVersion:         runtime.CompilerVersion,
		RuntimeBootstrapVersion: runtime.RuntimeBootstrapVersion,
		CompileCommandID:        runtime.CompileCommandID,
		RunCommandID:            runtime.RunCommandID,
		ManifestVersion:         runtime.ManifestVersion,
		ManifestChecksum:        runtime.ManifestChecksum,
	}
}

func capabilityRelease(tuple capability.CapabilityTuple) problemservice.CapabilityRelease {
	return problemservice.CapabilityRelease{
		JudgeSpecProtocol:             tuple.JudgeSpecProtocol,
		RuntimeKey:                    tuple.RuntimeKey,
		RuntimeProfileVersion:         tuple.RuntimeProfileVersion,
		CompilerVersion:               tuple.CompilerVersion,
		RuntimeBootstrapVersion:       tuple.RuntimeBootstrapVersion,
		CompileCommandID:              tuple.CompileCommandID,
		RunCommandID:                  tuple.RunCommandID,
		ImageDigest:                   tuple.ImageDigest,
		ManifestVersion:               tuple.ManifestVersion,
		ManifestChecksum:              tuple.ManifestChecksum,
		SupervisorProtocol:            tuple.SupervisorProtocol,
		CheckerKey:                    tuple.CheckerKey,
		CheckerVersion:                tuple.CheckerVersion,
		CheckerImplementationChecksum: tuple.CheckerImplementationChecksum,
		CapabilityProfileHash:         tuple.CapabilityProfileHash,
	}
}

func canonicalizeDraft(draft *Draft) {
	slices.SortFunc(draft.Runtimes, func(left, right RuntimeEvidence) int {
		return strings.Compare(left.RuntimeKey, right.RuntimeKey)
	})
	slices.SortFunc(draft.Checkers, func(left, right CheckerEvidence) int {
		return strings.Compare(left.CheckerKey, right.CheckerKey)
	})
	slices.SortFunc(draft.Capabilities, func(left, right CapabilitySelection) int {
		if compared := strings.Compare(left.RuntimeKey, right.RuntimeKey); compared != 0 {
			return compared
		}
		return strings.Compare(left.CheckerKey, right.CheckerKey)
	})
}

func canonicalizeManifest(manifest *problemservice.ApprovedReleaseManifest) {
	slices.SortFunc(manifest.Runtimes, func(left, right problemservice.RuntimeRelease) int {
		if compared := strings.Compare(left.RuntimeKey, right.RuntimeKey); compared != 0 {
			return compared
		}
		return strings.Compare(left.ProfileVersion, right.ProfileVersion)
	})
	slices.SortFunc(manifest.Checkers, func(left, right problemservice.CheckerRelease) int {
		if compared := strings.Compare(left.CheckerKey, right.CheckerKey); compared != 0 {
			return compared
		}
		return strings.Compare(left.Version, right.Version)
	})
	slices.SortFunc(manifest.Capabilities, func(left, right problemservice.CapabilityRelease) int {
		return strings.Compare(left.CapabilityProfileHash, right.CapabilityProfileHash)
	})
}

func validDigest(value string) bool {
	if !digestPattern.MatchString(value) {
		return false
	}
	body := strings.TrimPrefix(value, "sha256:")
	for index := 1; index < len(body); index++ {
		if body[index] != body[0] {
			return true
		}
	}
	return false
}

func summary(
	manifest problemservice.ApprovedReleaseManifest,
	imageDigest string,
) Summary {
	return Summary{
		ReleaseID:          manifest.ReleaseID,
		ApprovalChecksum:   manifest.ApprovalChecksum,
		ApprovedBy:         manifest.ApprovedBy,
		AuditID:            manifest.AuditID,
		ApprovedAt:         manifest.ApprovedAt.UTC(),
		SandboxImageDigest: imageDigest,
		RuntimeCount:       len(manifest.Runtimes),
		CheckerCount:       len(manifest.Checkers),
		CapabilityCount:    len(manifest.Capabilities),
	}
}

func (summary Summary) String() string {
	return fmt.Sprintf(
		"release=%s checksum=%s runtimes=%d checkers=%d capabilities=%d",
		summary.ReleaseID,
		summary.ApprovalChecksum,
		summary.RuntimeCount,
		summary.CheckerCount,
		summary.CapabilityCount,
	)
}
