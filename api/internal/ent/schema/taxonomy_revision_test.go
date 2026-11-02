package schema

import (
	"reflect"
	"strings"
	"testing"

	"entgo.io/ent"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

type taxonomySchemaContract interface {
	Fields() []ent.Field
	Indexes() []ent.Index
	Annotations() []entschema.Annotation
	Mixin() []ent.Mixin
}

func TestTaxonomyStableVersionsAreRequiredPositiveAndDefaultOne(t *testing.T) {
	t.Parallel()

	for name, currentSchema := range map[string]interface {
		Fields() []ent.Field
		Annotations() []entschema.Annotation
	}{
		"difficulty": Difficulty{},
		"tag":        Tag{},
		"element":    Element{},
	} {
		name, currentSchema := name, currentSchema
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			descriptor := requiredFieldDescriptor(t, currentSchema.Fields(), "version")
			if descriptor.Optional || descriptor.Nillable {
				t.Fatalf("version descriptor = optional:%t nillable:%t, want required", descriptor.Optional, descriptor.Nillable)
			}
			if got, ok := descriptor.Default.(int64); !ok || got != 1 {
				t.Fatalf("version default = %#v, want int64(1)", descriptor.Default)
			}
			requireInt64PositiveValidator(t, descriptor)

			checks := sqlChecks(t, currentSchema.Annotations())
			if got, want := checks[name+"_positive_version"], "version >= 1"; got != want {
				t.Fatalf("%s positive version check = %q, want %q", name, got, want)
			}
		})
	}
}

func TestTaxonomyDifficultyRewardUsesClosedServerOwnedBounds(t *testing.T) {
	t.Parallel()

	for name, fields := range map[string][]ent.Field{
		"stable":   (Difficulty{}).Fields(),
		"revision": (DifficultyRevision{}).Fields(),
	} {
		descriptor := requiredFieldDescriptor(t, fields, "exp_reward")
		if validateInt64Field(descriptor, -1) == nil {
			t.Errorf("%s difficulty exp reward accepts -1", name)
		}
		if err := validateInt64Field(descriptor, problemcontract.MaximumBaseExperience); err != nil {
			t.Errorf("%s difficulty exp reward rejects maximum: %v", name, err)
		}
		if validateInt64Field(descriptor, problemcontract.MaximumBaseExperience+1) == nil {
			t.Errorf("%s difficulty exp reward accepts maximum + 1", name)
		}
	}
}

func TestTaxonomyRevisionSchemasAreAppendOnlyImmutableAndUniquelyVersioned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		currentSchema taxonomySchemaContract
		uniqueFields  []string
	}{
		{name: "difficulty", currentSchema: DifficultyRevision{}, uniqueFields: []string{"difficulty_id", "revision"}},
		{name: "tag", currentSchema: TagRevision{}, uniqueFields: []string{"tag_id", "revision"}},
		{name: "element", currentSchema: ElementRevision{}, uniqueFields: []string{"element_id", "revision"}},
		{name: "tag revision element", currentSchema: TagRevisionElement{}, uniqueFields: []string{"tag_revision_id", "element_id"}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requireAppendOnlyMixin(t, test.currentSchema.Mixin())
			for _, schemaField := range test.currentSchema.Fields() {
				descriptor := schemaField.Descriptor()
				if !descriptor.Immutable {
					t.Errorf("field %q is mutable", descriptor.Name)
				}
			}
			requireExactUniqueEntIndex(t, test.currentSchema.Indexes(), test.uniqueFields)
		})
	}
}

func TestTaxonomyRevisionIdentitiesOwnersAndChecksumsFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		currentSchema taxonomySchemaContract
		ownerFields   []string
		checkName     string
	}{
		{
			name: "difficulty", currentSchema: DifficultyRevision{},
			ownerFields: []string{"difficulty_id"}, checkName: "difficulty_revision",
		},
		{
			name: "tag", currentSchema: TagRevision{},
			ownerFields: []string{"tag_id"}, checkName: "tag_revision",
		},
		{
			name: "element", currentSchema: ElementRevision{},
			ownerFields: []string{"element_id"}, checkName: "element_revision",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			checks := sqlChecks(t, test.currentSchema.Annotations())
			identityCheck := checks[test.checkName+"_nonzero_identity"]
			for _, name := range append([]string{"id"}, test.ownerFields...) {
				if !strings.Contains(identityCheck, name+" <> "+problemNilUUIDSQL) {
					t.Errorf("identity check %q does not reject zero %s", identityCheck, name)
				}
			}
			if got, want := checks[test.checkName+"_checksum_shape"],
				"octet_length(checksum) = 32 AND checksum <> "+problemZeroDigestSQL; got != want {
				t.Errorf("checksum check = %q, want %q", got, want)
			}
		})
	}

	linkChecks := sqlChecks(t, TagRevisionElement{}.Annotations())
	linkIdentity := linkChecks["tag_revision_element_nonzero_identity"]
	for _, name := range []string{"id", "tag_revision_id", "element_id"} {
		if !strings.Contains(linkIdentity, name+" <> "+problemNilUUIDSQL) {
			t.Errorf("tag revision link identity check %q does not reject zero %s", linkIdentity, name)
		}
	}
}

func TestTaxonomyTagRevisionElementPinsExactElementRevision(t *testing.T) {
	t.Parallel()

	descriptor := requiredFieldDescriptor(t, (TagRevisionElement{}).Fields(), "element_revision")
	if descriptor.Optional || descriptor.Nillable || !descriptor.Immutable {
		t.Fatalf(
			"element revision descriptor = optional:%t nillable:%t immutable:%t, want required and immutable",
			descriptor.Optional,
			descriptor.Nillable,
			descriptor.Immutable,
		)
	}
	requireInt64PositiveValidator(t, descriptor)
}

func requiredFieldDescriptor(t *testing.T, fields []ent.Field, name string) *field.Descriptor {
	t.Helper()
	for _, schemaField := range fields {
		descriptor := schemaField.Descriptor()
		if descriptor.Name == name {
			return descriptor
		}
	}
	t.Fatalf("field %q is missing", name)
	return nil
}

func requireInt64PositiveValidator(t *testing.T, descriptor *field.Descriptor) {
	t.Helper()
	for _, validator := range descriptor.Validators {
		validate, ok := validator.(func(int64) error)
		if !ok {
			continue
		}
		if validate(0) != nil && validate(1) == nil {
			return
		}
	}
	t.Fatalf("field %q does not reject 0 and accept 1", descriptor.Name)
}

func validateInt64Field(descriptor *field.Descriptor, value int64) error {
	for _, validator := range descriptor.Validators {
		validate, ok := validator.(func(int64) error)
		if ok {
			if err := validate(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func requireAppendOnlyMixin(t *testing.T, mixins []ent.Mixin) {
	t.Helper()
	want := reflect.TypeOf(commonent.AppendOnlyMixin{})
	var matches int
	for _, currentMixin := range mixins {
		if reflect.TypeOf(currentMixin) == want {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("AppendOnlyMixin count = %d, want 1", matches)
	}
}

func requireExactUniqueEntIndex(t *testing.T, indexes []ent.Index, fields []string) {
	t.Helper()
	var matches int
	for _, currentIndex := range indexes {
		descriptor := currentIndex.Descriptor()
		if descriptor.Unique && reflect.DeepEqual(descriptor.Fields, fields) {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("unique index on %v count = %d, want 1", fields, matches)
	}
}
