package schema

import (
	"testing"

	"entgo.io/ent"
	"github.com/stretchr/testify/require"
)

func TestCultivationRewardHistorySchemasAreImmutableAndEffectiveDated(t *testing.T) {
	tests := []struct {
		name   string
		fields []ent.Field
	}{
		{name: "trait effect revision", fields: TraitEffectRevision{}.Fields()},
		{name: "user reward profile revision", fields: UserRewardProfileRevision{}.Fields()},
		{name: "user reward profile trait", fields: UserRewardProfileTrait{}.Fields()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			byName := make(map[string]struct{}, len(test.fields))
			for _, schemaField := range test.fields {
				descriptor := schemaField.Descriptor()
				byName[descriptor.Name] = struct{}{}
				require.True(t, descriptor.Immutable, descriptor.Name)
			}
			require.Contains(t, byName, "id")
		})
	}
	require.Contains(t, fieldNames(TraitEffectRevision{}.Fields()), "effective_at")
	require.Contains(t, fieldNames(UserRewardProfileRevision{}.Fields()), "effective_at")
}

func fieldNames(fields []ent.Field) []string {
	result := make([]string, len(fields))
	for index, current := range fields {
		result[index] = current.Descriptor().Name
	}
	return result
}
