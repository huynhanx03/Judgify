package judging

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRuntimeCatalogIsClosedAndReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	values := LanguageValues()
	if want := []string{"cpp", "python", "java", "go"}; !reflect.DeepEqual(values, want) {
		t.Fatalf("LanguageValues() = %#v, want %#v", values, want)
	}
	for _, value := range values {
		parsed, ok := ParseLanguage(value)
		if !ok || string(parsed) != value {
			t.Fatalf("ParseLanguage(%q) = (%q, %t)", value, parsed, ok)
		}
	}
	if _, ok := ParseLanguage(" CPP "); ok {
		t.Fatal("ParseLanguage silently changed a protocol identifier")
	}

	identities := []RuntimeIdentity{
		{Key: "python3", Language: "python", DisplayName: "Python 3"},
		{Key: "cpp17", Language: "cpp", DisplayName: "C++17"},
	}
	first, err := NewRuntimeCatalog(identities)
	if err != nil {
		t.Fatalf("NewRuntimeCatalog() error = %v", err)
	}
	second, err := NewRuntimeCatalog(identities)
	if err != nil {
		t.Fatalf("NewRuntimeCatalog() error = %v", err)
	}
	if first.Version != RuntimeCatalogVersion || first.MaximumSourceCodeBytes != MaximumSourceCodeBytes {
		t.Fatalf("unexpected catalog metadata: %+v", first)
	}
	if len(first.Runtimes) != len(identities) ||
		first.Runtimes[0].Key != "cpp17" ||
		len(first.Runtimes[0].FileExtensions) == 0 {
		t.Fatal("runtime catalog is empty")
	}
	first.Runtimes[0].FileExtensions[0] = ".mutated"
	if second.Runtimes[0].FileExtensions[0] == ".mutated" {
		t.Fatal("runtime catalog leaked mutable package state")
	}
}

func TestRuntimeCatalogRejectsConflictingAndUntrustedIdentities(t *testing.T) {
	t.Parallel()

	for _, identities := range [][]RuntimeIdentity{
		{
			{Key: "go", Language: "go", DisplayName: "Go"},
			{Key: "go", Language: "cpp", DisplayName: "C++"},
		},
		{{Key: "Go 1.25", Language: "go", DisplayName: "Go"}},
		{{Key: "go", Language: "ruby", DisplayName: "Ruby"}},
		{{Key: "go", Language: "go", DisplayName: " Go "}},
	} {
		if _, err := NewRuntimeCatalog(identities); err == nil {
			t.Fatalf("NewRuntimeCatalog(%+v) unexpectedly succeeded", identities)
		}
	}
}

func TestRuntimeCatalogRejectsAnUnboundedActivationProjection(t *testing.T) {
	t.Parallel()

	identities := make([]RuntimeIdentity, MaximumCatalogRuntimes+1)
	for index := range identities {
		identities[index] = RuntimeIdentity{
			Key:         fmt.Sprintf("go.%02d", index),
			Language:    "go",
			DisplayName: fmt.Sprintf("Go %02d", index),
		}
	}
	if _, err := NewRuntimeCatalog(identities); err == nil {
		t.Fatal("unbounded runtime activation projection unexpectedly succeeded")
	}
}
