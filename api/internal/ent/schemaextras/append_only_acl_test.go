package schemaextras

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJudgifyAppendOnlyACLDDLIsDeterministicAndComplete(t *testing.T) {
	t.Parallel()

	first, err := JudgifyAppendOnlyACLDDL("public")
	require.NoError(t, err)
	second, err := JudgifyAppendOnlyACLDDL("public")
	require.NoError(t, err)
	require.Equal(t, first, second)

	for _, role := range []string{judgifyAPIRole, judgifyWorkerRole} {
		require.Contains(t, first, "FROM pg_roles WHERE rolname = '"+role+"'")
		for _, table := range AppendOnlyTables() {
			statement := "REVOKE UPDATE, DELETE, TRUNCATE ON TABLE \"public\".\"" + table + "\" FROM " + role + ";"
			require.Equal(t, 1, strings.Count(first, statement), statement)
		}
	}
	for _, table := range TaxonomyAppendOnlyTables() {
		statement := "REVOKE INSERT ON TABLE \"public\".\"" + table + "\" FROM " + judgifyWorkerRole + ";"
		require.Equal(t, 1, strings.Count(first, statement), statement)
	}
	for _, table := range AppendOnlyTables() {
		if contains(TaxonomyAppendOnlyTables(), table) {
			continue
		}
		require.NotContains(t, first, "REVOKE INSERT ON TABLE \"public\".\""+table+"\" FROM "+judgifyWorkerRole+";")
	}
}

func TestJudgifyAppendOnlyACLDDLRejectsInvalidSchema(t *testing.T) {
	t.Parallel()

	_, err := JudgifyAppendOnlyACLDDL("public; DROP SCHEMA public")
	require.Error(t, err)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
