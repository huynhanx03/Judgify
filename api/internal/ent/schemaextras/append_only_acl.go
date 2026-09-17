package schemaextras

import (
	"fmt"
	"strings"
)

const (
	judgifyAPIRole    = "judgify_api"
	judgifyWorkerRole = "judgify_worker"
)

var taxonomyAppendOnlyTables = []string{
	"difficulty_revisions",
	"element_revisions",
	"tag_revision_elements",
	"tag_revisions",
}

// JudgifyAppendOnlyACLDDL returns the least-privilege grants that complement
// the append-only trigger guard. The anonymous block deliberately tests role
// existence before executing each REVOKE so an Atlas authoring database that
// has no production runtime roles can still replay the checked baseline.
func JudgifyAppendOnlyACLDDL(schema string) (string, error) {
	if schema != "" && !postgresIdentifierPattern.MatchString(schema) {
		return "", fmt.Errorf("schema extras: invalid PostgreSQL schema identifier %q", schema)
	}

	qualifiedTable := func(table string) string {
		if schema == "" {
			return quoteIdentifier(table)
		}
		return quoteIdentifier(schema) + "." + quoteIdentifier(table)
	}

	var ddl strings.Builder
	ddl.WriteString("-- Keep runtime roles from mutating append-only records.\n")
	ddl.WriteString("DO $$\nBEGIN\n")
	for _, role := range []string{judgifyAPIRole, judgifyWorkerRole} {
		fmt.Fprintf(&ddl, "  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN\n", role)
		for _, table := range judgifyAppendOnlyTables {
			fmt.Fprintf(
				&ddl,
				"    REVOKE UPDATE, DELETE, TRUNCATE ON TABLE %s FROM %s;\n",
				qualifiedTable(table),
				role,
			)
		}
		if role == judgifyWorkerRole {
			for _, table := range taxonomyAppendOnlyTables {
				fmt.Fprintf(&ddl, "    REVOKE INSERT ON TABLE %s FROM %s;\n", qualifiedTable(table), role)
			}
		}
		ddl.WriteString("  END IF;\n")
	}
	ddl.WriteString("END;\n$$;")
	return ddl.String(), nil
}

// TaxonomyAppendOnlyTables returns the worker-write-protected history tables.
func TaxonomyAppendOnlyTables() []string {
	return append([]string(nil), taxonomyAppendOnlyTables...)
}
