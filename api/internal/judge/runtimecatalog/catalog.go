// Package runtimecatalog binds a worker to the reviewed, code-owned runtime
// release. It deliberately depends on Judgify's Problem runtime contracts and
// capability activation records; go-common remains unaware of judge images,
// checker versions, or sandbox protocols.
package runtimecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/judge/capability"
	problemports "github.com/huynhanx03/judgify/internal/ports"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	problemservice "github.com/huynhanx03/judgify/internal/problem/usecase"
)

var (
	ErrRuntimeCatalogDenied    = errors.New("judge runtime catalog: denied")
	ErrCapabilityBindingDenied = errors.New("judge runtime catalog: activated capability denied")
)

var (
	releaseIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	digestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// Catalog is a release-scoped view of executable runtime and checker
// contracts. Its profiles originate only from BuildLaunchProfiles, which
// overlays a reviewed approval manifest on code-owned templates.
type Catalog struct {
	releaseID        string
	approvalChecksum string
	catalogVersion   string
	profiles         *problemservice.RuntimeRegistry
	runtimeProfiles  []problementity.RuntimeProfile
	checkerProfiles  []problementity.CheckerProfile
	approvedTuples   []capability.CapabilityTuple
	approvedByHash   map[string]capability.CapabilityTuple
}

// LoadApprovedRelease loads the complete code-owned catalog for exactly one
// approved release. It never reads executable commands from a database.
func LoadApprovedRelease(
	manifest problemservice.ApprovedReleaseManifest,
) (*Catalog, error) {
	runtimes, checkers, err := problemservice.BuildLaunchProfiles(manifest)
	if err != nil {
		return nil, errors.Join(ErrRuntimeCatalogDenied, err)
	}
	profiles, err := problemservice.NewRuntimeRegistry(runtimes, checkers)
	if err != nil || !releaseIDPattern.MatchString(manifest.ReleaseID) {
		return nil, errors.Join(ErrRuntimeCatalogDenied, err)
	}
	approvedTuples, err := catalogTuples(manifest.Capabilities, profiles)
	if err != nil {
		return nil, errors.Join(ErrRuntimeCatalogDenied, err)
	}
	restrictProfilesToApprovedTuples(runtimes, checkers, approvedTuples)
	profiles, err = problemservice.NewRuntimeRegistry(runtimes, checkers)
	if err != nil {
		return nil, errors.Join(ErrRuntimeCatalogDenied, err)
	}
	approvedByHash := make(map[string]capability.CapabilityTuple, len(approvedTuples))
	for _, tuple := range approvedTuples {
		approvedByHash[tuple.CapabilityProfileHash] = tuple
	}
	catalogVersion, err := problemports.NewAuthoringCatalogVersion(
		manifest.ReleaseID,
		manifest.ApprovalChecksum,
	)
	if err != nil {
		return nil, errors.Join(ErrRuntimeCatalogDenied, err)
	}
	return &Catalog{
		releaseID: manifest.ReleaseID, approvalChecksum: manifest.ApprovalChecksum,
		catalogVersion:  catalogVersion,
		profiles:        profiles,
		runtimeProfiles: cloneRuntimeProfiles(runtimes),
		checkerProfiles: cloneCheckerProfiles(checkers),
		approvedTuples:  approvedTuples, approvedByHash: approvedByHash,
	}, nil
}

func restrictProfilesToApprovedTuples(
	runtimes []problementity.RuntimeProfile,
	checkers []problementity.CheckerProfile,
	tuples []capability.CapabilityTuple,
) {
	approvedRuntimes := make(map[string]struct{}, len(tuples))
	approvedCheckers := make(map[string]struct{}, len(tuples))
	for _, tuple := range tuples {
		approvedRuntimes[tuple.RuntimeKey+"\x00"+tuple.RuntimeProfileVersion] = struct{}{}
		approvedCheckers[tuple.CheckerKey+"\x00"+tuple.CheckerVersion] = struct{}{}
	}
	for index := range runtimes {
		_, runtimes[index].Active = approvedRuntimes[runtimes[index].Key+"\x00"+runtimes[index].ProfileVersion]
	}
	for index := range checkers {
		_, checkers[index].Active = approvedCheckers[checkers[index].Key+"\x00"+checkers[index].Version]
	}
}

func (catalog *Catalog) ReleaseID() string {
	if catalog == nil {
		return ""
	}
	return catalog.releaseID
}

func (catalog *Catalog) ApprovalChecksum() string {
	if catalog == nil {
		return ""
	}
	return catalog.approvalChecksum
}

func (catalog *Catalog) CatalogVersion() string {
	if catalog == nil {
		return ""
	}
	return catalog.catalogVersion
}

func (catalog *Catalog) ValidateAuthoringCatalogVersion(
	ctx context.Context,
	version string,
) error {
	if catalog == nil || ctx == nil {
		return problemports.ErrAuthoringCatalogChanged
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !problemports.MatchesAuthoringCatalogVersion(
		catalog.catalogVersion,
		version,
	) {
		return problemports.ErrAuthoringCatalogChanged
	}
	return nil
}

// ApprovedTuples returns the explicit release allow-list used during worker
// bootstrap/probing. It never reads platform activation: activation is
// necessarily downstream of a worker proving one of these tuples.
func (catalog *Catalog) ApprovedTuples() []capability.CapabilityTuple {
	if catalog == nil {
		return nil
	}
	return append([]capability.CapabilityTuple(nil), catalog.approvedTuples...)
}

// ApprovedTuple returns one exact reviewed tuple by its content-addressed
// profile hash. Callers cannot assemble a runtime/checker combination from
// independent fields.
func (catalog *Catalog) ApprovedTuple(
	profileHash string,
) (capability.CapabilityTuple, bool) {
	if catalog == nil || !digestPattern.MatchString(profileHash) {
		return capability.CapabilityTuple{}, false
	}
	tuple, exists := catalog.approvedByHash[profileHash]
	return tuple, exists
}

// RuntimeProfiles returns the reviewed, code-owned records which may be
// append-verified in the database for browser/admin projection. Workers never
// resolve executable commands from those database rows.
func (catalog *Catalog) RuntimeProfiles() []problementity.RuntimeProfile {
	if catalog == nil {
		return nil
	}
	return cloneRuntimeProfiles(catalog.runtimeProfiles)
}

// CheckerProfiles returns the reviewed, code-owned checker records for the
// same append-verify projection boundary as RuntimeProfiles.
func (catalog *Catalog) CheckerProfiles() []problementity.CheckerProfile {
	if catalog == nil {
		return nil
	}
	return cloneCheckerProfiles(catalog.checkerProfiles)
}

func (catalog *Catalog) AuthoringCapabilities() []problemports.AuthoringCapabilityOption {
	if catalog == nil {
		return nil
	}
	capabilities := make(
		[]problemports.AuthoringCapabilityOption,
		0,
		len(catalog.approvedTuples),
	)
	for _, tuple := range catalog.approvedTuples {
		capabilities = append(capabilities, problemports.AuthoringCapabilityOption{
			RuntimeKey:     tuple.RuntimeKey,
			CheckerKey:     tuple.CheckerKey,
			CheckerVersion: tuple.CheckerVersion,
		})
	}
	return capabilities
}

// RuntimeProfile is intentionally compatible with the worker executor's
// narrow catalog contract. It returns a defensive profile clone.
func (catalog *Catalog) RuntimeProfile(
	key string,
	version string,
) (problementity.RuntimeProfile, bool) {
	if catalog == nil || catalog.profiles == nil {
		return problementity.RuntimeProfile{}, false
	}
	return catalog.profiles.RuntimeProfile(key, version)
}

func (catalog *Catalog) CheckerProfile(
	key string,
	version string,
) (problementity.CheckerProfile, bool) {
	if catalog == nil || catalog.profiles == nil {
		return problementity.CheckerProfile{}, false
	}
	return catalog.profiles.CheckerProfile(key, version)
}

// ValidateRuntimeKeys satisfies Problem's publication boundary using the same
// reviewed in-memory registry as worker execution. Database profile rows are
// projections only and can never make a runtime publishable by themselves.
func (catalog *Catalog) ValidateRuntimeKeys(ctx context.Context, keys []string) error {
	if catalog == nil || catalog.profiles == nil || ctx == nil || ctx.Err() != nil {
		return ErrRuntimeCatalogDenied
	}
	return catalog.profiles.ValidateRuntimeKeys(ctx, keys)
}

// ValidateExecutionPairings proves each authored runtime/checker combination
// against one exact tuple from the reviewed release allow-list. Independent
// runtime and checker membership is intentionally insufficient.
func (catalog *Catalog) ValidateExecutionPairings(
	ctx context.Context,
	runtimeKeys []string,
	checkerKey string,
	checkerVersion string,
) error {
	if catalog == nil || catalog.profiles == nil || ctx == nil {
		return problemports.ErrExecutionCapabilityUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := catalog.ValidateRuntimeKeys(ctx, runtimeKeys); err != nil {
		return errors.Join(problemports.ErrExecutionCapabilityUnavailable, err)
	}
	checker, exists := catalog.CheckerProfile(checkerKey, checkerVersion)
	if !exists || !checker.Active {
		return problemports.ErrExecutionCapabilityUnavailable
	}
	seen := make(map[string]struct{}, len(runtimeKeys))
	for _, runtimeKey := range runtimeKeys {
		if _, duplicate := seen[runtimeKey]; duplicate {
			return problemports.ErrExecutionCapabilityUnavailable
		}
		seen[runtimeKey] = struct{}{}
		approved := false
		for _, tuple := range catalog.approvedTuples {
			if tuple.RuntimeKey == runtimeKey && tuple.CheckerKey == checkerKey &&
				tuple.CheckerVersion == checkerVersion {
				runtime, exists := catalog.profiles.RuntimeProfile(
					runtimeKey,
					tuple.RuntimeProfileVersion,
				)
				if exists && runtime.Active && runtimeMatchesTuple(runtime, tuple) {
					approved = true
					break
				}
			}
		}
		if !approved {
			return problemports.ErrExecutionCapabilityUnavailable
		}
	}
	return nil
}

// ValidateChecker applies the code-owned checker config contract for Problem
// drafts/publications. It deliberately has no permissive or DB-backed path.
func (catalog *Catalog) ValidateChecker(
	ctx context.Context,
	key string,
	version string,
	config json.RawMessage,
) error {
	if catalog == nil || catalog.profiles == nil || ctx == nil || ctx.Err() != nil {
		return ErrRuntimeCatalogDenied
	}
	return catalog.profiles.ValidateChecker(ctx, key, version, config)
}

// BoundCapability contains the only executable runtime contract a worker may
// use for one plan. Activation is retained for provenance/logging, but callers
// must not use it as an alternative source of command text.
type BoundCapability struct {
	ReleaseID  string
	Tuple      capability.CapabilityTuple
	Runtime    problementity.RuntimeProfile
	Checker    problementity.CheckerProfile
	Activation capability.ActivatedCapability
}

// ActivationBinder verifies a complete, content-addressed tuple against both
// the code-owned release catalog and the exact durable activation for that
// same release. It cannot synthesize runtime/checker combinations.
type ActivationBinder struct {
	catalog     *Catalog
	activations capability.ActivationReader
}

func NewActivationBinder(
	catalog *Catalog,
	activations capability.ActivationReader,
) (*ActivationBinder, error) {
	if catalog == nil || catalog.profiles == nil || len(catalog.approvedTuples) == 0 ||
		len(catalog.approvedByHash) != len(catalog.approvedTuples) ||
		!releaseIDPattern.MatchString(catalog.releaseID) ||
		!digestPattern.MatchString(catalog.approvalChecksum) || activations == nil {
		return nil, ErrRuntimeCatalogDenied
	}
	return &ActivationBinder{catalog: catalog, activations: activations}, nil
}

func (binder *ActivationBinder) Bind(
	ctx context.Context,
	tuple capability.CapabilityTuple,
) (BoundCapability, error) {
	if binder == nil || binder.catalog == nil || binder.activations == nil ||
		ctx == nil || ctx.Err() != nil || !exactTupleHash(tuple) {
		return BoundCapability{}, ErrCapabilityBindingDenied
	}
	approved, exists := binder.catalog.approvedByHash[tuple.CapabilityProfileHash]
	if !exists || approved != tuple {
		return BoundCapability{}, ErrCapabilityBindingDenied
	}
	runtime, exists := binder.catalog.RuntimeProfile(tuple.RuntimeKey, tuple.RuntimeProfileVersion)
	if !exists || !runtime.Active || !runtimeMatchesTuple(runtime, tuple) {
		return BoundCapability{}, ErrCapabilityBindingDenied
	}
	checker, exists := binder.catalog.CheckerProfile(tuple.CheckerKey, tuple.CheckerVersion)
	if !exists || !checker.Active ||
		checker.ImplementationChecksum != tuple.CheckerImplementationChecksum {
		return BoundCapability{}, ErrCapabilityBindingDenied
	}
	activation, found, err := binder.activations.Active(ctx, binder.catalog.releaseID, tuple)
	if err != nil || !found || !exactActiveActivation(
		activation,
		binder.catalog.releaseID,
		binder.catalog.approvalChecksum,
		tuple,
		checker,
	) {
		return BoundCapability{}, errors.Join(ErrCapabilityBindingDenied, err)
	}
	return BoundCapability{
		ReleaseID: binder.catalog.releaseID, Tuple: tuple, Runtime: runtime,
		Checker: checker, Activation: activation,
	}, nil
}

func catalogTuples(
	releases []problemservice.CapabilityRelease,
	profiles *problemservice.RuntimeRegistry,
) ([]capability.CapabilityTuple, error) {
	if profiles == nil || len(releases) == 0 {
		return nil, ErrRuntimeCatalogDenied
	}
	tuples := make([]capability.CapabilityTuple, 0, len(releases))
	seen := make(map[string]struct{}, len(releases))
	for _, release := range releases {
		tuple := capability.CapabilityTuple{
			JudgeSpecProtocol: release.JudgeSpecProtocol,
			RuntimeKey:        release.RuntimeKey, RuntimeProfileVersion: release.RuntimeProfileVersion,
			CompilerVersion: release.CompilerVersion, RuntimeBootstrapVersion: release.RuntimeBootstrapVersion,
			CompileCommandID: release.CompileCommandID, RunCommandID: release.RunCommandID,
			ImageDigest: release.ImageDigest, ManifestVersion: release.ManifestVersion,
			ManifestChecksum: release.ManifestChecksum, SupervisorProtocol: release.SupervisorProtocol,
			CheckerKey: release.CheckerKey, CheckerVersion: release.CheckerVersion,
			CheckerImplementationChecksum: release.CheckerImplementationChecksum,
			CapabilityProfileHash:         release.CapabilityProfileHash,
		}
		if !exactTupleHash(tuple) {
			return nil, ErrRuntimeCatalogDenied
		}
		if _, exists := seen[tuple.CapabilityProfileHash]; exists {
			return nil, ErrRuntimeCatalogDenied
		}
		seen[tuple.CapabilityProfileHash] = struct{}{}
		runtime, found := profiles.RuntimeProfile(tuple.RuntimeKey, tuple.RuntimeProfileVersion)
		if !found || !runtime.Active || !runtimeMatchesTuple(runtime, tuple) {
			return nil, ErrRuntimeCatalogDenied
		}
		checker, found := profiles.CheckerProfile(tuple.CheckerKey, tuple.CheckerVersion)
		if !found || !checker.Active ||
			checker.ImplementationChecksum != tuple.CheckerImplementationChecksum {
			return nil, ErrRuntimeCatalogDenied
		}
		tuples = append(tuples, tuple)
	}
	slices.SortFunc(tuples, func(left, right capability.CapabilityTuple) int {
		if left.CapabilityProfileHash < right.CapabilityProfileHash {
			return -1
		}
		if left.CapabilityProfileHash > right.CapabilityProfileHash {
			return 1
		}
		return 0
	})
	return tuples, nil
}

func exactTupleHash(tuple capability.CapabilityTuple) bool {
	want, err := capability.ComputeCapabilityProfileHash(tuple)
	return err == nil && want == tuple.CapabilityProfileHash
}

func cloneRuntimeProfiles(values []problementity.RuntimeProfile) []problementity.RuntimeProfile {
	clones := make([]problementity.RuntimeProfile, len(values))
	for index, value := range values {
		clones[index] = value
		clones[index].Compile.Arguments = append([]problementity.CommandArgument(nil), value.Compile.Arguments...)
		clones[index].Compile.Environment = append([]problementity.EnvironmentVariable(nil), value.Compile.Environment...)
		clones[index].Run.Arguments = append([]problementity.CommandArgument(nil), value.Run.Arguments...)
		clones[index].Run.Environment = append([]problementity.EnvironmentVariable(nil), value.Run.Environment...)
		clones[index].Artifacts = append([]problementity.RuntimeArtifact(nil), value.Artifacts...)
		clones[index].Workspace.Values = append([]problementity.RuntimeWorkspaceValue(nil), value.Workspace.Values...)
		clones[index].Workspace.TrustedFiles = append([]problementity.RuntimeWorkspaceFile(nil), value.Workspace.TrustedFiles...)
	}
	return clones
}

func cloneCheckerProfiles(values []problementity.CheckerProfile) []problementity.CheckerProfile {
	return append([]problementity.CheckerProfile(nil), values...)
}

func runtimeMatchesTuple(
	runtime problementity.RuntimeProfile,
	tuple capability.CapabilityTuple,
) bool {
	return runtime.Key == tuple.RuntimeKey &&
		runtime.ProfileVersion == tuple.RuntimeProfileVersion &&
		runtime.CompilerVersion == tuple.CompilerVersion &&
		runtime.RuntimeBootstrapVersion == tuple.RuntimeBootstrapVersion &&
		runtime.CompileCommandID == tuple.CompileCommandID &&
		runtime.RunCommandID == tuple.RunCommandID &&
		runtime.ImageDigest == tuple.ImageDigest &&
		runtime.ManifestVersion == tuple.ManifestVersion &&
		runtime.ManifestChecksum == tuple.ManifestChecksum
}

func exactActiveActivation(
	activation capability.ActivatedCapability,
	releaseID string,
	approvalChecksum string,
	tuple capability.CapabilityTuple,
	checker problementity.CheckerProfile,
) bool {
	return activation.ID != uuid.Nil && activation.ReleaseID == releaseID &&
		activation.ApprovedReleaseManifestChecksum == approvalChecksum &&
		digestPattern.MatchString(approvalChecksum) &&
		activation.State == capability.ActivationStateActive &&
		activation.RetiredAt == nil && activation.Tuple == tuple &&
		activation.ActorID != uuid.Nil && activation.AuditID != uuid.Nil &&
		activation.CheckerImplementationChecksum == checker.ImplementationChecksum &&
		activation.CheckerImplementationChecksum == tuple.CheckerImplementationChecksum &&
		digestPattern.MatchString(activation.SmokeChecksum) &&
		digestPattern.MatchString(activation.GoldenChecksum) &&
		activation.ProofWorkerInstanceID != uuid.Nil && activation.ProofBootID != uuid.Nil &&
		activation.ProofEvidenceID != uuid.Nil && !activation.ActivatedAt.IsZero()
}

var _ capability.ProfileCatalog = (*Catalog)(nil)
var _ problemports.AuthoringCatalogSource = (*Catalog)(nil)
var _ problemports.AuthoringCatalogFence = (*Catalog)(nil)
var _ problemports.ExecutionCatalog = (*Catalog)(nil)
var _ problemports.CheckerCatalog = (*Catalog)(nil)
