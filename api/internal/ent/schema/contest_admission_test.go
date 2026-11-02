package schema

import (
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/stretchr/testify/require"
)

func TestContestAdmissionSchemaCarriesDurablePolicyAndSequence(t *testing.T) {
	t.Parallel()

	fields := schemaFieldsByName(t, Contest{}.Fields())
	require.Equal(t, []string{"draft", "upcoming", "running", "ended", "cancelled"}, enumValues(t, fields["status"]))
	require.Contains(t, fields, "version")
	require.Contains(t, fields, "registration_mode")
	require.Contains(t, fields, "registration_opens_at")
	require.Contains(t, fields, "registration_closes_at")
	require.Contains(t, fields, "next_submission_sequence")
	require.True(t, fields["max_participants"].Optional)
	require.True(t, fields["max_participants"].Nillable)
}

func TestContestRegistrationSchemaRetainsBanAndVersionHistory(t *testing.T) {
	t.Parallel()

	fields := schemaFieldsByName(t, ContestRegistration{}.Fields())
	require.Equal(t, []string{"registered", "withdrawn", "banned"}, enumValues(t, fields["status"]))
	require.Equal(t, []string{"explicit", "implicit_open", "admin"}, enumValues(t, fields["source"]))
	for _, name := range []string{"version", "registered_at", "withdrawn_at", "banned_at", "reason"} {
		require.Contains(t, fields, name)
	}
	require.True(t, hasExactUniqueFields(ContestRegistration{}.Indexes(), "contest_id", "user_id"))
}

func TestContestProblemSnapshotIdentityIsUnique(t *testing.T) {
	t.Parallel()

	fields := schemaFieldsByName(t, ContestProblem{}.Fields())
	for _, name := range []string{
		"contest_id", "problem_id", "problem_revision_id", "testset_revision_id",
		"checker_key", "checker_version", "checker_config", "allowed_runtime_keys", "snapshot_checksum",
	} {
		require.Contains(t, fields, name)
	}
	require.True(t, hasExactUniqueFields(ContestProblem{}.Indexes(), "contest_id", "alias"))
	require.True(t, hasExactUniqueFields(ContestProblem{}.Indexes(), "contest_id", "display_order"))
}

func TestContestHistoriesUseImmutableUUIDRecordFields(t *testing.T) {
	t.Parallel()
	for _, fields := range [][]ent.Field{
		ContestProblem{}.Fields(),
		ContestTransition{}.Fields(),
		ContestRegistrationTransition{}.Fields(),
	} {
		descriptors := schemaFieldsByName(t, fields)
		for _, name := range []string{"id", "created_at"} {
			require.Contains(t, descriptors, name)
			require.True(t, descriptors[name].Immutable)
		}
	}
}

func schemaFieldsByName(t *testing.T, fields []ent.Field) map[string]*field.Descriptor {
	t.Helper()
	result := make(map[string]*field.Descriptor, len(fields))
	for _, item := range fields {
		descriptor := item.Descriptor()
		result[descriptor.Name] = descriptor
	}
	return result
}

func enumValues(t *testing.T, descriptor *field.Descriptor) []string {
	t.Helper()
	require.NotNil(t, descriptor)
	values := make([]string, 0, len(descriptor.Enums))
	for _, enum := range descriptor.Enums {
		values = append(values, enum.V)
	}
	return values
}

func hasExactUniqueFields(indexes []ent.Index, names ...string) bool {
	for _, item := range indexes {
		descriptor := item.Descriptor()
		if !descriptor.Unique || len(descriptor.Fields) != len(names) {
			continue
		}
		match := true
		for index, name := range names {
			if descriptor.Fields[index] != name {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
