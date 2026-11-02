package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

func TestRuntimeRegistryAcceptsOnlyActiveRegisteredRuntimeKeys(t *testing.T) {
	registry, err := NewRuntimeRegistry(runtimeProfilesForTest(t), checkerProfilesForTest(t))
	if err != nil {
		t.Fatalf("construct runtime registry: %v", err)
	}

	if err := registry.ValidateRuntimeKeys(
		context.Background(),
		[]string{"cpp17", "python3", "java17", "go"},
	); err != nil {
		t.Fatalf("validate initial runtime keys: %v", err)
	}
	if err := registry.ValidateRuntimeKeys(context.Background(), []string{"ruby3"}); err == nil {
		t.Fatal("unregistered runtime key validated")
	}

	profiles := runtimeProfilesForTest(t)
	profiles[0].Active = false
	inactiveRegistry, err := NewRuntimeRegistry(profiles, checkerProfilesForTest(t))
	if err != nil {
		t.Fatalf("construct registry with inactive profile: %v", err)
	}
	if err := inactiveRegistry.ValidateRuntimeKeys(context.Background(), []string{profiles[0].Key}); err == nil {
		t.Fatal("inactive runtime profile validated")
	}
}

func TestRuntimeRegistryValidatesOnlyRegisteredExecutionComponents(t *testing.T) {
	registry, err := NewRuntimeRegistry(runtimeProfilesForTest(t), checkerProfilesForTest(t))
	if err != nil {
		t.Fatalf("construct runtime registry: %v", err)
	}
	if err := registry.ValidateExecutionPairings(
		context.Background(),
		[]string{"cpp17", "python3"},
		"exact",
		"v1",
	); err != nil {
		t.Fatalf("ValidateExecutionPairings() error = %v", err)
	}
	if err := registry.ValidateExecutionPairings(
		context.Background(),
		[]string{"cpp17"},
		"custom",
		"v1",
	); !errors.Is(err, ports.ErrExecutionCapabilityUnavailable) {
		t.Fatalf("unknown checker pairing error = %v", err)
	}
}

func TestRuntimeRegistryAcceptsOnlyActiveRegisteredCheckerConfiguration(t *testing.T) {
	registry, err := NewRuntimeRegistry(runtimeProfilesForTest(t), checkerProfilesForTest(t))
	if err != nil {
		t.Fatalf("construct runtime registry: %v", err)
	}

	valid := []struct {
		key     string
		version string
		config  json.RawMessage
	}{
		{key: "exact", version: "v1", config: json.RawMessage(`{}`)},
		{key: "token", version: "v1", config: json.RawMessage(`{}`)},
		{key: "float", version: "v1", config: json.RawMessage(`{"abs_tol":0.001,"rel_tol":0.001}`)},
	}
	for _, checker := range valid {
		if err := registry.ValidateChecker(context.Background(), checker.key, checker.version, checker.config); err != nil {
			t.Fatalf("validate %s.%s: %v", checker.key, checker.version, err)
		}
	}

	invalid := []struct {
		name    string
		key     string
		version string
		config  json.RawMessage
	}{
		{name: "missing component", key: "custom", version: "v1", config: json.RawMessage(`{}`)},
		{name: "null exact config", key: "exact", version: "v1", config: json.RawMessage(`null`)},
		{name: "wrong exact config", key: "exact", version: "v1", config: json.RawMessage(`{"trim":true}`)},
		{name: "out of bounds float tolerance", key: "float", version: "v1", config: json.RawMessage(`{"abs_tol":2,"rel_tol":0}`)},
		{name: "unknown float config", key: "float", version: "v1", config: json.RawMessage(`{"abs_tol":0,"rel_tol":0,"mode":"loose"}`)},
	}
	for _, checker := range invalid {
		t.Run(checker.name, func(t *testing.T) {
			if err := registry.ValidateChecker(context.Background(), checker.key, checker.version, checker.config); err == nil {
				t.Fatal("invalid checker component validated")
			}
		})
	}

	checkers := checkerProfilesForTest(t)
	checkers[0].Active = false
	inactiveRegistry, err := NewRuntimeRegistry(runtimeProfilesForTest(t), checkers)
	if err != nil {
		t.Fatalf("construct registry with inactive checker: %v", err)
	}
	if err := inactiveRegistry.ValidateChecker(context.Background(), checkers[0].Key, checkers[0].Version, json.RawMessage(`{}`)); err == nil {
		t.Fatal("inactive checker component validated")
	}
}

func TestRuntimeRegistryRejectsUntrustedRuntimeProfiles(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*entity.RuntimeProfile)
	}{
		{
			name: "floating image tag",
			mutate: func(profile *entity.RuntimeProfile) {
				profile.ImageDigest = "judgify/cpp17:latest"
			},
		},
		{
			name: "shell executable",
			mutate: func(profile *entity.RuntimeProfile) {
				profile.Compile.Executable = "/bin/sh"
			},
		},
		{
			name: "shell interpolation",
			mutate: func(profile *entity.RuntimeProfile) {
				profile.Run.Arguments = append(profile.Run.Arguments, entity.CommandArgument{
					Kind:  entity.ArgumentLiteral,
					Value: "$(contestant-source)",
				})
			},
		},
		{
			name: "missing exact profile version",
			mutate: func(profile *entity.RuntimeProfile) {
				profile.ProfileVersion = ""
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profiles := runtimeProfilesForTest(t)
			test.mutate(&profiles[0])

			_, err := NewRuntimeRegistry(profiles, checkerProfilesForTest(t))
			if !errors.Is(err, ErrInvalidRuntimeProfile) {
				t.Fatalf("NewRuntimeRegistry() error = %v, want %v", err, ErrInvalidRuntimeProfile)
			}
		})
	}
}
