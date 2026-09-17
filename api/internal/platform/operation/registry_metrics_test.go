package operation

import (
	"context"
	"testing"
)

func TestRegistryKindsReturnsSortedCodeOwnedCatalog(t *testing.T) {
	registry := NewRegistry()
	for _, kind := range []string{"zeta.v1", "alpha.v1"} {
		if err := RegisterJSON[struct{}](registry, kind, func(context.Context, struct{}, BatchInput) (BatchResult, error) {
			return BatchResult{}, nil
		}); err != nil {
			t.Fatalf("RegisterJSON(%q): %v", kind, err)
		}
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	kinds := registry.Kinds()
	if len(kinds) != 2 || kinds[0] != "alpha.v1" || kinds[1] != "zeta.v1" {
		t.Fatalf("Kinds = %v", kinds)
	}
	kinds[0] = "mutated"
	if registry.Kinds()[0] != "alpha.v1" {
		t.Fatal("Kinds exposed mutable registry storage")
	}
}
