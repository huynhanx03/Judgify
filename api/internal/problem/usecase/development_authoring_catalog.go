package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// DevelopmentAuthoringCatalog exposes the code-owned problem authoring
// contract without claiming that a sandbox image has passed release review.
// It deliberately does not implement any worker runtime or capability
// interface. Composition may use it only for a non-production API process.
type DevelopmentAuthoringCatalog struct {
	releaseID      string
	catalogVersion string
	runtimes       []entity.RuntimeProfile
	checkers       []entity.CheckerProfile
	runtimeKeys    map[string]struct{}
	checkerByID    map[string]entity.CheckerProfile
}

func NewDevelopmentAuthoringCatalog(
	releaseID string,
) (*DevelopmentAuthoringCatalog, error) {
	if !releaseIdentifierPattern.MatchString(releaseID) {
		return nil, ports.ErrInvalidAuthoringCatalog
	}
	runtimes := initialRuntimeTemplates()
	checkers := initialCheckerTemplates()
	if len(runtimes) == 0 || len(checkers) == 0 {
		return nil, ports.ErrInvalidAuthoringCatalog
	}

	catalog := &DevelopmentAuthoringCatalog{
		releaseID:   releaseID,
		runtimes:    make([]entity.RuntimeProfile, 0, len(runtimes)),
		checkers:    make([]entity.CheckerProfile, 0, len(checkers)),
		runtimeKeys: make(map[string]struct{}, len(runtimes)),
		checkerByID: make(map[string]entity.CheckerProfile, len(checkers)),
	}
	for _, profile := range runtimes {
		if profile.ID == [16]byte{} || !profile.Active ||
			!validCatalogIdentifier(profile.Key, 128) ||
			!validCatalogIdentifier(profile.ProfileVersion, 128) ||
			!validCatalogText(profile.Language, 128) ||
			!validCatalogText(profile.DisplayName, 256) {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		if _, duplicate := catalog.runtimeKeys[profile.Key]; duplicate {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		catalog.runtimeKeys[profile.Key] = struct{}{}
		catalog.runtimes = append(catalog.runtimes, cloneRuntimeProfile(profile))
	}
	for _, profile := range checkers {
		if profile.ID == [16]byte{} || !profile.Active ||
			!validCatalogIdentifier(profile.Key, 128) ||
			!validCatalogIdentifier(profile.Version, 128) ||
			(profile.ConfigKind != entity.CheckerConfigNone &&
				profile.ConfigKind != entity.CheckerConfigFloatTolerance) {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		identity := profileIdentity(profile.Key, profile.Version)
		if _, duplicate := catalog.checkerByID[identity]; duplicate {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		catalog.checkerByID[identity] = profile
		catalog.checkers = append(catalog.checkers, profile)
	}
	binding, err := developmentAuthoringCatalogBinding(catalog)
	if err != nil {
		return nil, err
	}
	catalog.catalogVersion, err = ports.NewAuthoringCatalogVersion(
		catalog.releaseID,
		binding,
	)
	if err != nil {
		return nil, err
	}
	return catalog, nil
}

func (catalog *DevelopmentAuthoringCatalog) ReleaseID() string {
	if catalog == nil {
		return ""
	}
	return catalog.releaseID
}

func (catalog *DevelopmentAuthoringCatalog) CatalogVersion() string {
	if catalog == nil {
		return ""
	}
	return catalog.catalogVersion
}

func (catalog *DevelopmentAuthoringCatalog) ValidateAuthoringCatalogVersion(
	ctx context.Context,
	version string,
) error {
	if catalog == nil || ctx == nil {
		return ports.ErrAuthoringCatalogChanged
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ports.MatchesAuthoringCatalogVersion(catalog.catalogVersion, version) {
		return ports.ErrAuthoringCatalogChanged
	}
	return nil
}

func (catalog *DevelopmentAuthoringCatalog) RuntimeProfiles() []entity.RuntimeProfile {
	if catalog == nil {
		return nil
	}
	profiles := make([]entity.RuntimeProfile, 0, len(catalog.runtimes))
	for _, profile := range catalog.runtimes {
		profiles = append(profiles, cloneRuntimeProfile(profile))
	}
	return profiles
}

func (catalog *DevelopmentAuthoringCatalog) CheckerProfiles() []entity.CheckerProfile {
	if catalog == nil {
		return nil
	}
	return append([]entity.CheckerProfile(nil), catalog.checkers...)
}

func (catalog *DevelopmentAuthoringCatalog) AuthoringCapabilities() []ports.AuthoringCapabilityOption {
	if catalog == nil {
		return nil
	}
	capabilities := make(
		[]ports.AuthoringCapabilityOption,
		0,
		len(catalog.runtimes)*len(catalog.checkers),
	)
	for _, runtime := range catalog.runtimes {
		for _, checker := range catalog.checkers {
			capabilities = append(capabilities, ports.AuthoringCapabilityOption{
				RuntimeKey:     runtime.Key,
				CheckerKey:     checker.Key,
				CheckerVersion: checker.Version,
			})
		}
	}
	return capabilities
}

func (catalog *DevelopmentAuthoringCatalog) ValidateRuntimeKeys(
	ctx context.Context,
	runtimeKeys []string,
) error {
	if catalog == nil || ctx == nil || len(runtimeKeys) == 0 {
		return ErrRuntimeProfileUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(runtimeKeys))
	for _, key := range runtimeKeys {
		if _, duplicate := seen[key]; duplicate {
			return ErrRuntimeProfileUnavailable
		}
		seen[key] = struct{}{}
		if _, exists := catalog.runtimeKeys[key]; !exists {
			return ErrRuntimeProfileUnavailable
		}
	}
	return nil
}

func (catalog *DevelopmentAuthoringCatalog) ValidateChecker(
	ctx context.Context,
	key string,
	version string,
	config json.RawMessage,
) error {
	if catalog == nil || ctx == nil {
		return ErrCheckerProfileUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	profile, exists := catalog.checkerByID[profileIdentity(key, version)]
	if !exists {
		return ErrCheckerProfileUnavailable
	}
	return validateCheckerConfiguration(profile, config)
}

func (catalog *DevelopmentAuthoringCatalog) ValidateExecutionPairings(
	ctx context.Context,
	runtimeKeys []string,
	checkerKey string,
	checkerVersion string,
) error {
	if catalog == nil || ctx == nil {
		return ports.ErrExecutionCapabilityUnavailable
	}
	if err := catalog.ValidateRuntimeKeys(ctx, runtimeKeys); err != nil {
		return errors.Join(ports.ErrExecutionCapabilityUnavailable, err)
	}
	profile, exists := catalog.checkerByID[profileIdentity(checkerKey, checkerVersion)]
	if !exists || !profile.Active {
		return ports.ErrExecutionCapabilityUnavailable
	}
	return nil
}

func developmentAuthoringCatalogBinding(
	catalog *DevelopmentAuthoringCatalog,
) (string, error) {
	if catalog == nil {
		return "", ports.ErrInvalidAuthoringCatalog
	}
	runtimes, err := projectAuthoringRuntimes(catalog.runtimes)
	if err != nil {
		return "", err
	}
	checkers, err := projectAuthoringCheckers(catalog.checkers)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(struct {
		Runtimes     []ports.AuthoringRuntimeOption
		Checkers     []ports.AuthoringCheckerOption
		Capabilities []ports.AuthoringCapabilityOption
		Policy       ports.AuthoringPolicy
	}{
		Runtimes:     runtimes,
		Checkers:     checkers,
		Capabilities: catalog.AuthoringCapabilities(),
		Policy:       authoringPolicy(),
	})
	if err != nil {
		return "", errors.Join(ports.ErrInvalidAuthoringCatalog, err)
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

var (
	_ ports.AuthoringCatalogSource = (*DevelopmentAuthoringCatalog)(nil)
	_ ports.AuthoringCatalogFence  = (*DevelopmentAuthoringCatalog)(nil)
	_ ports.ExecutionCatalog       = (*DevelopmentAuthoringCatalog)(nil)
	_ ports.CheckerCatalog         = (*DevelopmentAuthoringCatalog)(nil)
)
