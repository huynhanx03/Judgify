package permissions

import (
	"slices"
	"testing"
)

func TestCatalogIsClosedSortedAndUnique(t *testing.T) {
	catalog := Catalog()
	if len(catalog) == 0 {
		t.Fatal("Catalog() is empty")
	}
	for index, resource := range catalog {
		if resource.Key == "" || len(resource.Actions) == 0 {
			t.Fatalf("Catalog()[%d] = %#v, want non-empty resource/actions", index, resource)
		}
		if index > 0 && catalog[index-1].Key >= resource.Key {
			t.Fatalf("resources are not strictly sorted: %q then %q", catalog[index-1].Key, resource.Key)
		}
		if !slices.IsSorted(resource.Actions) {
			t.Fatalf("actions for %q are not sorted: %v", resource.Key, resource.Actions)
		}
		for actionIndex := 1; actionIndex < len(resource.Actions); actionIndex++ {
			if resource.Actions[actionIndex-1] == resource.Actions[actionIndex] {
				t.Fatalf("duplicate action %q for %q", resource.Actions[actionIndex], resource.Key)
			}
		}
	}
}

func TestCatalogReturnsAnImmutableCopy(t *testing.T) {
	first := Catalog()
	originalResource := first[0].Key
	originalAction := first[0].Actions[0]
	first[0].Key = "mutated"
	first[0].Actions[0] = "mutated"

	second := Catalog()
	if second[0].Key != originalResource || second[0].Actions[0] != originalAction {
		t.Fatalf("Catalog() leaked mutable storage: %#v", second[0])
	}
}

func TestContainsRejectsUnknownAndWildcardCapabilities(t *testing.T) {
	if !Contains(ResourceKeyRole, PermissionScopeRead) {
		t.Fatal("role:read is missing")
	}
	for _, capability := range [][2]string{
		{"unknown", PermissionScopeRead},
		{ResourceKeyRole, "unknown"},
		{"*", PermissionScopeRead},
		{ResourceKeyRole, "*"},
		{"", ""},
	} {
		if Contains(capability[0], capability[1]) {
			t.Fatalf("Contains(%q, %q) = true", capability[0], capability[1])
		}
	}
}

func TestCatalogContainsEveryAdministrativeReadCapability(t *testing.T) {
	for _, resource := range []string{
		ResourceKeyAudit,
		ResourceKeyContest,
		ResourceKeyDifficulty,
		ResourceKeyMaterial,
		ResourceKeyMaterialCategory,
		ResourceKeyTag,
	} {
		if !Contains(resource, PermissionScopeRead) {
			t.Errorf("%s:read is missing from the code-owned catalog", resource)
		}
	}
}

func TestSubmissionCatalogSeparatesSafeReadFromSensitiveInspection(t *testing.T) {
	if !Contains(ResourceKeySubmission, PermissionScopeRead) {
		t.Fatal("submission:read is missing")
	}
	if !Contains(ResourceKeySubmission, PermissionScopeInspect) {
		t.Fatal("submission:inspect is missing")
	}

	for _, resource := range Catalog() {
		if resource.Key != ResourceKeySubmission {
			continue
		}
		if !slices.Equal(resource.Actions, []string{PermissionScopeInspect, PermissionScopeRead, PermissionScopeUpdate}) {
			t.Fatalf("submission actions = %v", resource.Actions)
		}
		return
	}
	t.Fatal("submission resource is missing")
}

func TestUserTraitCatalogIsReadOnlyProjection(t *testing.T) {
	for _, resource := range Catalog() {
		if resource.Key != ResourceKeyUserTrait {
			continue
		}
		if !slices.Equal(resource.Actions, []string{PermissionScopeRead}) {
			t.Fatalf("user trait projection actions = %v, want read only", resource.Actions)
		}
		return
	}
	t.Fatal("user trait projection resource is missing")
}

func TestDetailedCatalogIsCompleteAndReturnsAnImmutableCopy(t *testing.T) {
	t.Parallel()

	plain := Catalog()
	detailed := DetailedCatalog()
	if len(detailed) != len(plain) {
		t.Fatalf("DetailedCatalog() length = %d, want %d", len(detailed), len(plain))
	}
	for resourceIndex, resource := range detailed {
		if resource.Key != plain[resourceIndex].Key || resource.Label == "" || resource.Description == "" {
			t.Fatalf("DetailedCatalog()[%d] = %#v", resourceIndex, resource)
		}
		if len(resource.Actions) != len(plain[resourceIndex].Actions) {
			t.Fatalf("actions for %q = %d, want %d", resource.Key, len(resource.Actions), len(plain[resourceIndex].Actions))
		}
		for actionIndex, action := range resource.Actions {
			if action.Key != plain[resourceIndex].Actions[actionIndex] || action.Label == "" || action.Description == "" {
				t.Fatalf("DetailedCatalog()[%d].Actions[%d] = %#v", resourceIndex, actionIndex, action)
			}
		}
	}

	detailed[0].Key = "mutated"
	detailed[0].Actions[0].Key = "mutated"
	second := DetailedCatalog()
	if second[0].Key == "mutated" || second[0].Actions[0].Key == "mutated" {
		t.Fatal("DetailedCatalog() leaked mutable storage")
	}
}
