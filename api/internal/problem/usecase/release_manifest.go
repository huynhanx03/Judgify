package usecase

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"regexp"
	"slices"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

var ErrInvalidReleaseManifest = errors.New("runtime release manifest: invalid or unapproved")

var (
	releaseIdentifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	releaseDigestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type RuntimeRelease struct {
	RuntimeKey              string `json:"runtime_key"`
	ProfileVersion          string `json:"profile_version"`
	CompilerVersion         string `json:"compiler_version"`
	RuntimeBootstrapVersion string `json:"runtime_bootstrap_version"`
	CompileCommandID        string `json:"compile_command_id"`
	RunCommandID            string `json:"run_command_id"`
	ImageDigest             string `json:"image_digest"`
	ManifestVersion         string `json:"manifest_version"`
	ManifestChecksum        string `json:"manifest_checksum"`
}

type CheckerRelease struct {
	CheckerKey             string `json:"checker_key"`
	Version                string `json:"version"`
	ImplementationChecksum string `json:"implementation_checksum"`
}

// CapabilityRelease is one complete, reviewed worker capability. It is
// deliberately a tuple rather than a runtime list plus a checker list: a
// release author must explicitly approve every supported pairing.
//
// This is a release-manifest contract, not a worker runtime contract. The
// judge runtime catalog converts it to capability.CapabilityTuple at the
// composition boundary, keeping Problem's core free from worker concerns.
type CapabilityRelease struct {
	JudgeSpecProtocol             string `json:"judge_spec_protocol"`
	RuntimeKey                    string `json:"runtime_key"`
	RuntimeProfileVersion         string `json:"runtime_profile_version"`
	CompilerVersion               string `json:"compiler_version"`
	RuntimeBootstrapVersion       string `json:"runtime_bootstrap_version"`
	CompileCommandID              string `json:"compile_command_id"`
	RunCommandID                  string `json:"run_command_id"`
	ImageDigest                   string `json:"image_digest"`
	ManifestVersion               string `json:"manifest_version"`
	ManifestChecksum              string `json:"manifest_checksum"`
	SupervisorProtocol            string `json:"supervisor_protocol"`
	CheckerKey                    string `json:"checker_key"`
	CheckerVersion                string `json:"checker_version"`
	CheckerImplementationChecksum string `json:"checker_implementation_checksum"`
	CapabilityProfileHash         string `json:"capability_profile_hash"`
}

type ApprovedReleaseManifest struct {
	ReleaseID        string              `json:"release_id"`
	Approved         bool                `json:"approved"`
	ApprovedBy       uuid.UUID           `json:"approved_by"`
	AuditID          uuid.UUID           `json:"audit_id"`
	ApprovedAt       time.Time           `json:"approved_at"`
	ApprovalChecksum string              `json:"approval_checksum"`
	Runtimes         []RuntimeRelease    `json:"runtimes"`
	Checkers         []CheckerRelease    `json:"checkers"`
	Capabilities     []CapabilityRelease `json:"capabilities"`
}

// ComputeReleaseManifestChecksum binds the audit identity, every runtime and
// checker identity, and the explicit capability allow-list in canonical order.
// ApprovalChecksum itself is omitted.
func ComputeReleaseManifestChecksum(manifest ApprovedReleaseManifest) (string, error) {
	if manifest.ReleaseID == "" || manifest.ApprovedBy == uuid.Nil ||
		manifest.AuditID == uuid.Nil || manifest.ApprovedAt.IsZero() {
		return "", ErrInvalidReleaseManifest
	}
	runtimes := append([]RuntimeRelease(nil), manifest.Runtimes...)
	checkers := append([]CheckerRelease(nil), manifest.Checkers...)
	capabilities := append([]CapabilityRelease(nil), manifest.Capabilities...)
	slices.SortFunc(runtimes, func(left, right RuntimeRelease) int {
		if left.RuntimeKey != right.RuntimeKey {
			if left.RuntimeKey < right.RuntimeKey {
				return -1
			}
			return 1
		}
		if left.ProfileVersion < right.ProfileVersion {
			return -1
		}
		if left.ProfileVersion > right.ProfileVersion {
			return 1
		}
		return 0
	})
	slices.SortFunc(checkers, func(left, right CheckerRelease) int {
		if left.CheckerKey != right.CheckerKey {
			if left.CheckerKey < right.CheckerKey {
				return -1
			}
			return 1
		}
		if left.Version < right.Version {
			return -1
		}
		if left.Version > right.Version {
			return 1
		}
		return 0
	})
	slices.SortFunc(capabilities, compareCapabilityReleases)
	digest := sha256.New()
	writeReleasePart(digest, "judgify.approved-release-manifest.v4")
	writeReleasePart(digest, manifest.ReleaseID)
	writeReleasePart(digest, manifest.ApprovedBy.String())
	writeReleasePart(digest, manifest.AuditID.String())
	writeReleasePart(digest, manifest.ApprovedAt.UTC().Format(time.RFC3339Nano))
	if manifest.Approved {
		writeReleasePart(digest, "approved")
	} else {
		writeReleasePart(digest, "unapproved")
	}
	for _, runtime := range runtimes {
		for _, value := range []string{
			runtime.RuntimeKey,
			runtime.ProfileVersion,
			runtime.CompilerVersion,
			runtime.RuntimeBootstrapVersion,
			runtime.CompileCommandID,
			runtime.RunCommandID,
			runtime.ImageDigest,
			runtime.ManifestVersion,
			runtime.ManifestChecksum,
		} {
			writeReleasePart(digest, value)
		}
	}
	for _, checker := range checkers {
		for _, value := range []string{checker.CheckerKey, checker.Version, checker.ImplementationChecksum} {
			writeReleasePart(digest, value)
		}
	}
	for _, capability := range capabilities {
		for _, value := range capabilityReleaseParts(capability) {
			writeReleasePart(digest, value)
		}
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), nil
}

// BuildLaunchProfiles is the production constructor. There is no zero-input
// path to active runtime/checker identities.
func BuildLaunchProfiles(
	manifest ApprovedReleaseManifest,
) ([]entity.RuntimeProfile, []entity.CheckerProfile, error) {
	if !manifest.Approved || !releaseIdentifierPattern.MatchString(manifest.ReleaseID) ||
		manifest.ApprovedBy == uuid.Nil || manifest.AuditID == uuid.Nil || manifest.ApprovedAt.IsZero() ||
		!validReleaseDigest(manifest.ApprovalChecksum) {
		return nil, nil, ErrInvalidReleaseManifest
	}
	wantChecksum, err := ComputeReleaseManifestChecksum(manifest)
	if err != nil || wantChecksum != manifest.ApprovalChecksum {
		return nil, nil, ErrInvalidReleaseManifest
	}

	runtimes := initialRuntimeTemplates()
	checkers := initialCheckerTemplates()
	if len(runtimes) != 4 || len(manifest.Runtimes) != len(runtimes) ||
		len(manifest.Checkers) != len(checkers) || len(manifest.Capabilities) == 0 ||
		len(manifest.Capabilities) > maximumReleaseCapabilities {
		return nil, nil, ErrInvalidReleaseManifest
	}
	runtimeReleases := make(map[string]RuntimeRelease, len(manifest.Runtimes))
	seenRuntimeManifests := make(map[string]struct{}, len(manifest.Runtimes))
	for _, release := range manifest.Runtimes {
		identity := release.RuntimeKey + "\x00" + release.ProfileVersion
		if _, duplicate := runtimeReleases[identity]; duplicate ||
			!releaseIdentifierPattern.MatchString(release.RuntimeKey) ||
			!releaseIdentifierPattern.MatchString(release.ProfileVersion) ||
			!releaseIdentifierPattern.MatchString(release.CompilerVersion) ||
			!releaseIdentifierPattern.MatchString(release.RuntimeBootstrapVersion) ||
			!releaseIdentifierPattern.MatchString(release.CompileCommandID) ||
			!releaseIdentifierPattern.MatchString(release.RunCommandID) ||
			!releaseIdentifierPattern.MatchString(release.ManifestVersion) ||
			!validReleaseDigest(release.ImageDigest) || !validReleaseDigest(release.ManifestChecksum) {
			return nil, nil, ErrInvalidReleaseManifest
		}
		// The launch image deliberately contains all four runtimes, so the
		// immutable image digest is shared. Each runtime artifact contract must
		// still have its own checksum.
		if _, duplicate := seenRuntimeManifests[release.ManifestChecksum]; duplicate {
			return nil, nil, ErrInvalidReleaseManifest
		}
		seenRuntimeManifests[release.ManifestChecksum] = struct{}{}
		runtimeReleases[identity] = release
	}
	for index := range runtimes {
		profile := &runtimes[index]
		release, exists := runtimeReleases[profile.Key+"\x00"+profile.ProfileVersion]
		if !exists ||
			release.CompilerVersion != profile.CompilerVersion ||
			release.RuntimeBootstrapVersion != profile.RuntimeBootstrapVersion ||
			release.CompileCommandID != profile.CompileCommandID ||
			release.RunCommandID != profile.RunCommandID ||
			release.ManifestVersion != profile.ManifestVersion {
			return nil, nil, ErrInvalidReleaseManifest
		}
		profile.ImageDigest = release.ImageDigest
		profile.ManifestChecksum = release.ManifestChecksum
		profile.Active = true
	}

	checkerReleases := make(map[string]CheckerRelease, len(manifest.Checkers))
	seenCheckerDigests := make(map[string]struct{}, len(manifest.Checkers))
	for _, release := range manifest.Checkers {
		identity := release.CheckerKey + "\x00" + release.Version
		if _, duplicate := checkerReleases[identity]; duplicate ||
			!releaseIdentifierPattern.MatchString(release.CheckerKey) ||
			!releaseIdentifierPattern.MatchString(release.Version) ||
			!validReleaseDigest(release.ImplementationChecksum) {
			return nil, nil, ErrInvalidReleaseManifest
		}
		if _, duplicate := seenCheckerDigests[release.ImplementationChecksum]; duplicate {
			return nil, nil, ErrInvalidReleaseManifest
		}
		seenCheckerDigests[release.ImplementationChecksum] = struct{}{}
		checkerReleases[identity] = release
	}
	for index := range checkers {
		profile := &checkers[index]
		release, exists := checkerReleases[profile.Key+"\x00"+profile.Version]
		if !exists {
			return nil, nil, ErrInvalidReleaseManifest
		}
		profile.ImplementationChecksum = release.ImplementationChecksum
		profile.Active = true
	}
	if !validCapabilityReleases(manifest.Capabilities, runtimeReleases, checkerReleases) {
		return nil, nil, ErrInvalidReleaseManifest
	}
	return runtimes, checkers, nil
}

const maximumReleaseCapabilities = 64

func validCapabilityReleases(
	capabilities []CapabilityRelease,
	runtimes map[string]RuntimeRelease,
	checkers map[string]CheckerRelease,
) bool {
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if !validCapabilityRelease(capability) {
			return false
		}
		if _, duplicate := seen[capability.CapabilityProfileHash]; duplicate {
			return false
		}
		seen[capability.CapabilityProfileHash] = struct{}{}

		runtime, exists := runtimes[capability.RuntimeKey+"\x00"+capability.RuntimeProfileVersion]
		if !exists || !capabilityMatchesRuntimeRelease(capability, runtime) {
			return false
		}
		checker, exists := checkers[capability.CheckerKey+"\x00"+capability.CheckerVersion]
		if !exists ||
			checker.ImplementationChecksum != capability.CheckerImplementationChecksum {
			return false
		}
	}
	return true
}

func validCapabilityRelease(capability CapabilityRelease) bool {
	for _, value := range []string{
		capability.JudgeSpecProtocol,
		capability.RuntimeKey,
		capability.RuntimeProfileVersion,
		capability.CompilerVersion,
		capability.RuntimeBootstrapVersion,
		capability.CompileCommandID,
		capability.RunCommandID,
		capability.ManifestVersion,
		capability.SupervisorProtocol,
		capability.CheckerKey,
		capability.CheckerVersion,
	} {
		if !releaseIdentifierPattern.MatchString(value) {
			return false
		}
	}
	return validReleaseDigest(capability.ImageDigest) &&
		validReleaseDigest(capability.ManifestChecksum) &&
		validReleaseDigest(capability.CheckerImplementationChecksum) &&
		validReleaseDigest(capability.CapabilityProfileHash)
}

func capabilityMatchesRuntimeRelease(capability CapabilityRelease, runtime RuntimeRelease) bool {
	return capability.RuntimeKey == runtime.RuntimeKey &&
		capability.RuntimeProfileVersion == runtime.ProfileVersion &&
		capability.CompilerVersion == runtime.CompilerVersion &&
		capability.RuntimeBootstrapVersion == runtime.RuntimeBootstrapVersion &&
		capability.CompileCommandID == runtime.CompileCommandID &&
		capability.RunCommandID == runtime.RunCommandID &&
		capability.ImageDigest == runtime.ImageDigest &&
		capability.ManifestVersion == runtime.ManifestVersion &&
		capability.ManifestChecksum == runtime.ManifestChecksum
}

func compareCapabilityReleases(left, right CapabilityRelease) int {
	for index, leftValue := range capabilityReleaseParts(left) {
		rightValue := capabilityReleaseParts(right)[index]
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}

func capabilityReleaseParts(capability CapabilityRelease) []string {
	return []string{
		capability.JudgeSpecProtocol,
		capability.RuntimeKey,
		capability.RuntimeProfileVersion,
		capability.CompilerVersion,
		capability.RuntimeBootstrapVersion,
		capability.CompileCommandID,
		capability.RunCommandID,
		capability.ImageDigest,
		capability.ManifestVersion,
		capability.ManifestChecksum,
		capability.SupervisorProtocol,
		capability.CheckerKey,
		capability.CheckerVersion,
		capability.CheckerImplementationChecksum,
		capability.CapabilityProfileHash,
	}
}

func validReleaseDigest(value string) bool {
	if !releaseDigestPattern.MatchString(value) {
		return false
	}
	body := value[len("sha256:"):]
	for index := 1; index < len(body); index++ {
		if body[index] != body[0] {
			return true
		}
	}
	return false
}

func writeReleasePart(digest hash.Hash, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write([]byte(value))
}
