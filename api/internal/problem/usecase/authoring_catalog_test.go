package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestAuthoringCatalogReturnsOnlyActiveSafeMetadata(t *testing.T) {
	t.Parallel()

	source := authoringCatalogSourceStub{
		releaseID:      "judge-release.2026.07.29.1",
		catalogVersion: testAuthoringCatalogVersion,
		runtimes: []entity.RuntimeProfile{
			{
				ID:             uuid.New(),
				Key:            "python3",
				Language:       "python",
				DisplayName:    "Python 3",
				ProfileVersion: "3.13-v1",
				ImageDigest:    "must-not-leak",
				Active:         true,
			},
			{
				ID:             uuid.New(),
				Key:            "cpp17",
				Language:       "cpp",
				DisplayName:    "GNU C++17",
				ProfileVersion: "gcc-14-v1",
				Active:         true,
			},
			{
				ID:             uuid.New(),
				Key:            "disabled",
				Language:       "disabled",
				DisplayName:    "Disabled",
				ProfileVersion: "v1",
				Active:         false,
			},
		},
		checkers: []entity.CheckerProfile{
			{
				ID:                     uuid.New(),
				Key:                    entity.CheckerKeyFloat,
				Version:                "v1",
				ImplementationChecksum: "must-not-leak",
				ConfigKind:             entity.CheckerConfigFloatTolerance,
				Active:                 true,
			},
			{
				ID:         uuid.New(),
				Key:        entity.CheckerKeyExact,
				Version:    "v1",
				ConfigKind: entity.CheckerConfigNone,
				Active:     true,
			},
			{
				ID:         uuid.New(),
				Key:        "disabled",
				Version:    "v1",
				ConfigKind: entity.CheckerConfigNone,
				Active:     false,
			},
		},
	}
	query, err := NewAuthoringCatalog(source)
	if err != nil {
		t.Fatalf("NewAuthoringCatalog() error = %v", err)
	}

	result, err := query.GetCatalog(context.Background())
	if err != nil {
		t.Fatalf("GetCatalog() error = %v", err)
	}
	if result.ReleaseID != source.releaseID {
		t.Fatalf("release id = %q, want %q", result.ReleaseID, source.releaseID)
	}
	if result.CatalogVersion != source.catalogVersion {
		t.Fatalf(
			"catalog version = %q, want %q",
			result.CatalogVersion,
			source.catalogVersion,
		)
	}
	if len(result.Runtimes) != 2 ||
		result.Runtimes[0].Key != "cpp17" ||
		result.Runtimes[1].Key != "python3" {
		t.Fatalf("runtimes = %#v, want active runtimes sorted by key", result.Runtimes)
	}
	if len(result.Checkers) != 2 ||
		result.Checkers[0].Key != entity.CheckerKeyExact ||
		result.Checkers[1].Key != entity.CheckerKeyFloat {
		t.Fatalf("checkers = %#v, want active checkers sorted by identity", result.Checkers)
	}
	if len(result.Capabilities) != 4 ||
		result.Capabilities[0].RuntimeKey != "cpp17" ||
		result.Capabilities[0].CheckerKey != entity.CheckerKeyExact {
		t.Fatalf("capabilities = %#v, want sorted safe runtime/checker pairs", result.Capabilities)
	}
	if result.Policy.CPUTimeMilliseconds.Default < result.Policy.CPUTimeMilliseconds.Minimum ||
		result.Policy.CPUTimeMilliseconds.Default > result.Policy.CPUTimeMilliseconds.Maximum ||
		result.Policy.MaximumTestcases < 1 ||
		result.Policy.TestAssetMaximumBytes < 1 {
		t.Fatalf("authoring policy = %#v, want bounded server policy", result.Policy)
	}
}

func TestAuthoringCatalogRejectsInvalidSourceAndProfiles(t *testing.T) {
	t.Parallel()

	if _, err := NewAuthoringCatalog(nil); !errors.Is(err, ports.ErrInvalidAuthoringCatalog) {
		t.Fatalf("NewAuthoringCatalog(nil) error = %v", err)
	}

	tests := []struct {
		name   string
		source authoringCatalogSourceStub
	}{
		{
			name: "missing release",
			source: authoringCatalogSourceStub{
				catalogVersion: testAuthoringCatalogVersion,
				runtimes:       validAuthoringCatalogRuntimes(),
				checkers:       validAuthoringCatalogCheckers(),
			},
		},
		{
			name: "missing catalog version",
			source: authoringCatalogSourceStub{
				releaseID: "release-1",
				runtimes:  validAuthoringCatalogRuntimes(),
				checkers:  validAuthoringCatalogCheckers(),
			},
		},
		{
			name: "malformed catalog version",
			source: authoringCatalogSourceStub{
				releaseID:      "release-1",
				catalogVersion: "v1.not-a-digest",
				runtimes:       validAuthoringCatalogRuntimes(),
				checkers:       validAuthoringCatalogCheckers(),
			},
		},
		{
			name: "duplicate runtime key",
			source: authoringCatalogSourceStub{
				releaseID:      "release-1",
				catalogVersion: testAuthoringCatalogVersion,
				runtimes: append(
					validAuthoringCatalogRuntimes(),
					validAuthoringCatalogRuntimes()[0],
				),
				checkers: validAuthoringCatalogCheckers(),
			},
		},
		{
			name: "unknown checker config",
			source: authoringCatalogSourceStub{
				releaseID:      "release-1",
				catalogVersion: testAuthoringCatalogVersion,
				runtimes:       validAuthoringCatalogRuntimes(),
				checkers: []entity.CheckerProfile{{
					ID: uuid.New(), Key: "custom", Version: "v1",
					ConfigKind: "unknown", Active: true,
				}},
			},
		},
		{
			name: "empty active catalog",
			source: authoringCatalogSourceStub{
				releaseID:      "release-1",
				catalogVersion: testAuthoringCatalogVersion,
			},
		},
		{
			name: "capability references unknown runtime",
			source: authoringCatalogSourceStub{
				releaseID:      "release-1",
				catalogVersion: testAuthoringCatalogVersion,
				runtimes:       validAuthoringCatalogRuntimes(),
				checkers:       validAuthoringCatalogCheckers(),
				capabilities: []ports.AuthoringCapabilityOption{{
					RuntimeKey:     "python3",
					CheckerKey:     entity.CheckerKeyExact,
					CheckerVersion: "v1",
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			query, err := NewAuthoringCatalog(test.source)
			if err != nil {
				t.Fatalf("NewAuthoringCatalog() error = %v", err)
			}
			if _, err := query.GetCatalog(context.Background()); !errors.Is(err, ports.ErrInvalidAuthoringCatalog) {
				t.Fatalf("GetCatalog() error = %v, want ErrInvalidAuthoringCatalog", err)
			}
		})
	}
}

func TestAuthoringCatalogRejectsNilOrCancelledContext(t *testing.T) {
	t.Parallel()

	query, err := NewAuthoringCatalog(authoringCatalogSourceStub{
		releaseID:      "release-1",
		catalogVersion: testAuthoringCatalogVersion,
		runtimes:       validAuthoringCatalogRuntimes(),
		checkers:       validAuthoringCatalogCheckers(),
	})
	if err != nil {
		t.Fatalf("NewAuthoringCatalog() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := query.GetCatalog(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetCatalog(cancelled) error = %v", err)
	}
}

type authoringCatalogSourceStub struct {
	releaseID      string
	catalogVersion string
	runtimes       []entity.RuntimeProfile
	checkers       []entity.CheckerProfile
	capabilities   []ports.AuthoringCapabilityOption
}

func (stub authoringCatalogSourceStub) ReleaseID() string {
	return stub.releaseID
}

func (stub authoringCatalogSourceStub) CatalogVersion() string {
	return stub.catalogVersion
}

func (stub authoringCatalogSourceStub) RuntimeProfiles() []entity.RuntimeProfile {
	return stub.runtimes
}

func (stub authoringCatalogSourceStub) CheckerProfiles() []entity.CheckerProfile {
	return stub.checkers
}

func (stub authoringCatalogSourceStub) AuthoringCapabilities() []ports.AuthoringCapabilityOption {
	if stub.capabilities != nil {
		return stub.capabilities
	}
	capabilities := make([]ports.AuthoringCapabilityOption, 0)
	for _, runtime := range stub.runtimes {
		if !runtime.Active {
			continue
		}
		for _, checker := range stub.checkers {
			if !checker.Active {
				continue
			}
			capabilities = append(capabilities, ports.AuthoringCapabilityOption{
				RuntimeKey:     runtime.Key,
				CheckerKey:     checker.Key,
				CheckerVersion: checker.Version,
			})
		}
	}
	return capabilities
}

func validAuthoringCatalogRuntimes() []entity.RuntimeProfile {
	return []entity.RuntimeProfile{{
		ID: uuid.New(), Key: "go", Language: "go",
		DisplayName: "Go", ProfileVersion: "1.25-v1", Active: true,
	}}
}

func validAuthoringCatalogCheckers() []entity.CheckerProfile {
	return []entity.CheckerProfile{{
		ID: uuid.New(), Key: entity.CheckerKeyExact, Version: "v1",
		ConfigKind: entity.CheckerConfigNone, Active: true,
	}}
}

const testAuthoringCatalogVersion = "v1.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
