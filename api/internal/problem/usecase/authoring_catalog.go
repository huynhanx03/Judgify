package usecase

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type authoringCatalog struct {
	source ports.AuthoringCatalogSource
}

const maximumAuthoringCapabilities = 64

func NewAuthoringCatalog(
	source ports.AuthoringCatalogSource,
) (ports.AuthoringCatalogQuery, error) {
	if nilAuthoringCatalogDependency(source) {
		return nil, ports.ErrInvalidAuthoringCatalog
	}
	return &authoringCatalog{source: source}, nil
}

var _ ports.AuthoringCatalogQuery = (*authoringCatalog)(nil)

func (query *authoringCatalog) GetCatalog(
	ctx context.Context,
) (ports.AuthoringCatalog, error) {
	if query == nil || nilAuthoringCatalogDependency(query.source) || ctx == nil {
		return ports.AuthoringCatalog{}, ports.ErrInvalidAuthoringCatalog
	}
	if err := ctx.Err(); err != nil {
		return ports.AuthoringCatalog{}, err
	}

	releaseID := query.source.ReleaseID()
	catalogVersion := query.source.CatalogVersion()
	if !validCatalogIdentifier(releaseID, 128) ||
		!ports.IsAuthoringCatalogVersion(catalogVersion) {
		return ports.AuthoringCatalog{}, ports.ErrInvalidAuthoringCatalog
	}

	runtimes, err := projectAuthoringRuntimes(query.source.RuntimeProfiles())
	if err != nil {
		return ports.AuthoringCatalog{}, err
	}
	checkers, err := projectAuthoringCheckers(query.source.CheckerProfiles())
	if err != nil {
		return ports.AuthoringCatalog{}, err
	}
	capabilities, err := projectAuthoringCapabilities(
		query.source.AuthoringCapabilities(),
		runtimes,
		checkers,
	)
	if err != nil {
		return ports.AuthoringCatalog{}, err
	}

	logger.FromContext(ctx).Debug(
		"problem authoring catalog projected",
		zap.String("release_id", releaseID),
		zap.String("catalog_version", catalogVersion),
		zap.Int("runtime_count", len(runtimes)),
		zap.Int("checker_count", len(checkers)),
		zap.Int("capability_count", len(capabilities)),
	)
	return ports.AuthoringCatalog{
		ReleaseID:      releaseID,
		CatalogVersion: catalogVersion,
		Runtimes:       runtimes,
		Checkers:       checkers,
		Capabilities:   capabilities,
		Policy:         authoringPolicy(),
	}, nil
}

func authoringPolicy() ports.AuthoringPolicy {
	return ports.AuthoringPolicy{
		SlugMaximumBytes:      problemcontract.MaximumSlugBytes,
		TitleMaximumBytes:     problemcontract.MaximumTitleBytes,
		StatementMaximumBytes: problemcontract.MaximumStatementMarkdownBytes,
		ReasonMaximumBytes:    problemcontract.MaximumPublicationReasonBytes,
		TestAssetMaximumBytes: problemcontract.MaximumTestAssetBytes,
		MaximumTestcases:      problemcontract.MaximumTestCases,
		MaximumGroups:         problemcontract.MaximumTestcaseGroups,
		MaximumTags:           problemcontract.MaximumTags,
		MaximumRuntimes:       problemcontract.MaximumAllowedRuntimes,
		CPUTimeMilliseconds: ports.AuthoringNumericPolicy{
			Default: problemcontract.DefaultCPUTimeLimitMilliseconds,
			Minimum: problemcontract.MinimumCPUTimeLimitMilliseconds,
			Maximum: problemcontract.MaximumCPUTimeLimitMilliseconds,
		},
		WallTimeMilliseconds: ports.AuthoringNumericPolicy{
			Default: problemcontract.DefaultWallTimeLimitMilliseconds,
			Minimum: problemcontract.MinimumCPUTimeLimitMilliseconds,
			Maximum: problemcontract.MaximumWallTimeLimitMilliseconds,
		},
		MemoryKilobytes: ports.AuthoringNumericPolicy{
			Default: problemcontract.DefaultMemoryLimitKilobytes,
			Minimum: problemcontract.MinimumMemoryLimitKilobytes,
			Maximum: problemcontract.MaximumMemoryLimitKilobytes,
		},
		OutputBytes: ports.AuthoringNumericPolicy{
			Default: problemcontract.DefaultOutputLimitBytes,
			Minimum: problemcontract.MinimumOutputLimitBytes,
			Maximum: problemcontract.MaximumOutputLimitBytes,
		},
		ProcessCount: ports.AuthoringNumericPolicy{
			Default: problemcontract.DefaultProcessLimit,
			Minimum: problemcontract.MinimumProcessLimit,
			Maximum: problemcontract.MaximumProcessLimit,
		},
	}
}

func projectAuthoringRuntimes(
	profiles []entity.RuntimeProfile,
) ([]ports.AuthoringRuntimeOption, error) {
	values := make([]ports.AuthoringRuntimeOption, 0, len(profiles))
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if !profile.Active {
			continue
		}
		if profile.ID == [16]byte{} ||
			!validCatalogIdentifier(profile.Key, 128) ||
			!validCatalogText(profile.Language, 128) ||
			!validCatalogText(profile.DisplayName, 256) ||
			!validCatalogIdentifier(profile.ProfileVersion, 128) {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		if _, duplicate := seen[profile.Key]; duplicate {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		seen[profile.Key] = struct{}{}
		values = append(values, ports.AuthoringRuntimeOption{
			Key:            profile.Key,
			Language:       profile.Language,
			DisplayName:    profile.DisplayName,
			ProfileVersion: profile.ProfileVersion,
		})
	}
	if len(values) == 0 {
		return nil, ports.ErrInvalidAuthoringCatalog
	}
	slices.SortFunc(values, func(left, right ports.AuthoringRuntimeOption) int {
		return strings.Compare(left.Key, right.Key)
	})
	return values, nil
}

func projectAuthoringCheckers(
	profiles []entity.CheckerProfile,
) ([]ports.AuthoringCheckerOption, error) {
	values := make([]ports.AuthoringCheckerOption, 0, len(profiles))
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if !profile.Active {
			continue
		}
		if profile.ID == [16]byte{} ||
			!validCatalogIdentifier(profile.Key, 128) ||
			!validCatalogIdentifier(profile.Version, 128) ||
			(profile.ConfigKind != entity.CheckerConfigNone &&
				profile.ConfigKind != entity.CheckerConfigFloatTolerance) {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		identity := profile.Key + "\x00" + profile.Version
		if _, duplicate := seen[identity]; duplicate {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		seen[identity] = struct{}{}
		values = append(values, ports.AuthoringCheckerOption{
			Key:        profile.Key,
			Version:    profile.Version,
			ConfigKind: profile.ConfigKind,
		})
	}
	if len(values) == 0 {
		return nil, ports.ErrInvalidAuthoringCatalog
	}
	slices.SortFunc(values, func(left, right ports.AuthoringCheckerOption) int {
		if compared := strings.Compare(left.Key, right.Key); compared != 0 {
			return compared
		}
		return strings.Compare(left.Version, right.Version)
	})
	return values, nil
}

func projectAuthoringCapabilities(
	values []ports.AuthoringCapabilityOption,
	runtimes []ports.AuthoringRuntimeOption,
	checkers []ports.AuthoringCheckerOption,
) ([]ports.AuthoringCapabilityOption, error) {
	if len(values) == 0 || len(values) > maximumAuthoringCapabilities ||
		len(runtimes) == 0 || len(checkers) == 0 {
		return nil, ports.ErrInvalidAuthoringCatalog
	}
	runtimeKeys := make(map[string]struct{}, len(runtimes))
	for _, runtime := range runtimes {
		runtimeKeys[runtime.Key] = struct{}{}
	}
	checkerIDs := make(map[string]struct{}, len(checkers))
	for _, checker := range checkers {
		checkerIDs[profileIdentity(checker.Key, checker.Version)] = struct{}{}
	}
	result := make([]ports.AuthoringCapabilityOption, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	coveredRuntimes := make(map[string]struct{}, len(runtimes))
	coveredCheckers := make(map[string]struct{}, len(checkers))
	for _, value := range values {
		checkerID := profileIdentity(value.CheckerKey, value.CheckerVersion)
		identity := value.RuntimeKey + "\x00" + checkerID
		_, runtimeExists := runtimeKeys[value.RuntimeKey]
		_, checkerExists := checkerIDs[checkerID]
		if !validCatalogIdentifier(value.RuntimeKey, 128) ||
			!validCatalogIdentifier(value.CheckerKey, 128) ||
			!validCatalogIdentifier(value.CheckerVersion, 128) ||
			!runtimeExists || !checkerExists {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		if _, duplicate := seen[identity]; duplicate {
			return nil, ports.ErrInvalidAuthoringCatalog
		}
		seen[identity] = struct{}{}
		coveredRuntimes[value.RuntimeKey] = struct{}{}
		coveredCheckers[checkerID] = struct{}{}
		result = append(result, value)
	}
	if len(coveredRuntimes) != len(runtimeKeys) ||
		len(coveredCheckers) != len(checkerIDs) {
		return nil, ports.ErrInvalidAuthoringCatalog
	}
	slices.SortFunc(result, func(left, right ports.AuthoringCapabilityOption) int {
		if compared := strings.Compare(left.RuntimeKey, right.RuntimeKey); compared != 0 {
			return compared
		}
		if compared := strings.Compare(left.CheckerKey, right.CheckerKey); compared != 0 {
			return compared
		}
		return strings.Compare(left.CheckerVersion, right.CheckerVersion)
	})
	return result, nil
}

func validCatalogIdentifier(value string, maximumBytes int) bool {
	if len(value) == 0 || len(value) > maximumBytes || !utf8.ValidString(value) {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validCatalogText(value string, maximumBytes int) bool {
	return len(value) <= maximumBytes &&
		utf8.ValidString(value) &&
		strings.TrimSpace(value) != "" &&
		strings.IndexByte(value, 0) < 0
}

func nilAuthoringCatalogDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
