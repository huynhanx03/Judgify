package schema

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestProblemLifecycleOwnsLegacyPublicVisibility(t *testing.T) {
	t.Parallel()

	checks := sqlChecks(t, Problem{}.Annotations())
	const want = "is_published = (lifecycle = 'published')"
	if got := checks["problem_public_visibility_matches_lifecycle"]; got != want {
		t.Fatalf(
			"problem public visibility check = %q, want %q",
			got,
			want,
		)
	}
}

func TestProblemSlugIsRequiredForEveryStableAggregate(t *testing.T) {
	t.Parallel()

	for _, schemaField := range (Problem{}).Fields() {
		descriptor := schemaField.Descriptor()
		if descriptor.Name != "slug" {
			continue
		}
		if descriptor.Optional || descriptor.Nillable || !descriptor.Immutable {
			t.Fatalf(
				"problem slug descriptor = optional:%t nillable:%t immutable:%t, want required and immutable",
				descriptor.Optional,
				descriptor.Nillable,
				descriptor.Immutable,
			)
		}
		return
	}
	t.Fatal("problem slug field is missing")
}

func TestProblemStableMappingIdentitiesAreRequiredImmutableAndUniqueWithOwner(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"tag_mapping_id", "element_mapping_id"} {
		descriptor := requiredFieldDescriptor(t, (Problem{}).Fields(), name)
		if descriptor.Optional || descriptor.Nillable || !descriptor.Immutable {
			t.Errorf(
				"problem %s descriptor = optional:%t nillable:%t immutable:%t, want required and immutable",
				name,
				descriptor.Optional,
				descriptor.Nillable,
				descriptor.Immutable,
			)
		}
		generate, ok := descriptor.Default.(func() uuid.UUID)
		if !ok {
			t.Fatalf("problem %s default = %T, want server UUID generator", name, descriptor.Default)
		}
		if generated := generate(); generated == uuid.Nil {
			t.Fatalf("problem %s generated zero UUID", name)
		}
	}

	for _, fields := range [][]string{
		{"id", "tag_mapping_id"},
		{"id", "element_mapping_id"},
	} {
		requireExactUniqueEntIndex(t, (Problem{}).Indexes(), fields)
	}

	identityCheck := sqlChecks(t, Problem{}.Annotations())["problem_nonzero_mapping_identities"]
	for _, field := range []string{"tag_mapping_id", "element_mapping_id"} {
		if !strings.Contains(identityCheck, field+" <> "+problemNilUUIDSQL) {
			t.Errorf("problem mapping identity check %q does not reject zero %s", identityCheck, field)
		}
	}
	if !strings.Contains(identityCheck, "tag_mapping_id <> element_mapping_id") {
		t.Errorf("problem mapping identity check %q does not require distinct mapping IDs", identityCheck)
	}
}
