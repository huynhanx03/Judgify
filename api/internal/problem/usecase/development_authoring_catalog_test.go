package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestDevelopmentAuthoringCatalogIsMetadataOnlyAndDefensive(t *testing.T) {
	catalog, err := NewDevelopmentAuthoringCatalog("judge-release.development")
	if err != nil {
		t.Fatalf("NewDevelopmentAuthoringCatalog() error = %v", err)
	}
	if catalog.ReleaseID() != "judge-release.development" {
		t.Fatalf("ReleaseID() = %q", catalog.ReleaseID())
	}
	if !strings.HasPrefix(catalog.CatalogVersion(), "v1.") ||
		len(catalog.CatalogVersion()) != 67 {
		t.Fatalf("CatalogVersion() = %q, want opaque v1 digest", catalog.CatalogVersion())
	}
	runtimes := catalog.RuntimeProfiles()
	checkers := catalog.CheckerProfiles()
	if len(runtimes) != 4 || len(checkers) != 3 {
		t.Fatalf("profile counts = %d/%d, want 4/3", len(runtimes), len(checkers))
	}
	if runtimes[0].ImageDigest != "" || runtimes[0].ManifestChecksum != "" ||
		checkers[0].ImplementationChecksum != "" {
		t.Fatal("development catalog must not claim executable artifact identities")
	}
	runtimes[0].Key = "mutated"
	checkers[0].Key = "mutated"
	if catalog.RuntimeProfiles()[0].Key == "mutated" ||
		catalog.CheckerProfiles()[0].Key == "mutated" {
		t.Fatal("catalog returned mutable internal profile storage")
	}
}

func TestDevelopmentAuthoringCatalogFencesMutationsToExactCatalog(t *testing.T) {
	t.Parallel()

	first, err := NewDevelopmentAuthoringCatalog("judge-release.development")
	if err != nil {
		t.Fatalf("NewDevelopmentAuthoringCatalog(first) error = %v", err)
	}
	second, err := NewDevelopmentAuthoringCatalog("judge-release.development")
	if err != nil {
		t.Fatalf("NewDevelopmentAuthoringCatalog(second) error = %v", err)
	}
	if first.CatalogVersion() != second.CatalogVersion() {
		t.Fatalf(
			"equivalent development catalogs produced different versions: %q / %q",
			first.CatalogVersion(),
			second.CatalogVersion(),
		)
	}
	if err := first.ValidateAuthoringCatalogVersion(
		context.Background(),
		first.CatalogVersion(),
	); err != nil {
		t.Fatalf("ValidateAuthoringCatalogVersion(current) error = %v", err)
	}
	if err := first.ValidateAuthoringCatalogVersion(
		context.Background(),
		"v1.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	); !errors.Is(err, ports.ErrAuthoringCatalogChanged) {
		t.Fatalf("ValidateAuthoringCatalogVersion(stale) error = %v", err)
	}
}

func TestDevelopmentAuthoringCatalogValidatesAuthoringChoices(t *testing.T) {
	catalog, err := NewDevelopmentAuthoringCatalog("judge-release.development")
	if err != nil {
		t.Fatalf("NewDevelopmentAuthoringCatalog() error = %v", err)
	}
	ctx := context.Background()
	if err := catalog.ValidateRuntimeKeys(ctx, []string{
		entity.RuntimeKeyCPP17,
		entity.RuntimeKeyPython3,
	}); err != nil {
		t.Fatalf("ValidateRuntimeKeys() error = %v", err)
	}
	if err := catalog.ValidateChecker(
		ctx,
		entity.CheckerKeyExact,
		"v1",
		json.RawMessage(`{}`),
	); err != nil {
		t.Fatalf("ValidateChecker(exact) error = %v", err)
	}
	if err := catalog.ValidateChecker(
		ctx,
		entity.CheckerKeyFloat,
		"v1",
		json.RawMessage(`{"abs_tol":0.000001,"rel_tol":0.000001}`),
	); err != nil {
		t.Fatalf("ValidateChecker(float) error = %v", err)
	}
	if err := catalog.ValidateExecutionPairings(
		ctx,
		[]string{entity.RuntimeKeyCPP17, entity.RuntimeKeyPython3},
		entity.CheckerKeyFloat,
		"v1",
	); err != nil {
		t.Fatalf("ValidateExecutionPairings() error = %v", err)
	}

	for name, err := range map[string]error{
		"duplicate runtime": catalog.ValidateRuntimeKeys(ctx, []string{
			entity.RuntimeKeyCPP17,
			entity.RuntimeKeyCPP17,
		}),
		"unknown runtime": catalog.ValidateRuntimeKeys(ctx, []string{"ruby"}),
		"unknown checker": catalog.ValidateChecker(ctx, "custom", "v1", json.RawMessage(`{}`)),
		"unknown pairing": catalog.ValidateExecutionPairings(
			ctx,
			[]string{entity.RuntimeKeyGo},
			"custom",
			"v1",
		),
	} {
		if err == nil {
			t.Fatalf("%s unexpectedly succeeded", name)
		}
	}
}

func TestDevelopmentAuthoringCatalogRejectsInvalidIdentityAndContext(t *testing.T) {
	if _, err := NewDevelopmentAuthoringCatalog("Development Release"); !errors.Is(err, ports.ErrInvalidAuthoringCatalog) {
		t.Fatalf("invalid release error = %v", err)
	}
	catalog, err := NewDevelopmentAuthoringCatalog("judge-release.development")
	if err != nil {
		t.Fatalf("NewDevelopmentAuthoringCatalog() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := catalog.ValidateRuntimeKeys(ctx, []string{entity.RuntimeKeyGo}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled runtime validation error = %v", err)
	}
	if err := catalog.ValidateChecker(
		ctx,
		entity.CheckerKeyExact,
		"v1",
		json.RawMessage(`{}`),
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled checker validation error = %v", err)
	}
}
