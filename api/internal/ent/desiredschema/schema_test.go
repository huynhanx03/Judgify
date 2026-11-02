package desiredschema

import (
	"context"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entschema "entgo.io/ent/dialect/sql/schema"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
)

func TestRuntimeCapabilityStorageBoundsAreDatabaseEnforced(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)

	wantTypes := map[string]map[string]string{
		"checker_profiles": {
			"checker_key":             "character varying(64)",
			"checker_version":         "character varying(64)",
			"implementation_checksum": "character varying(71)",
		},
		"judge_worker_instances": {
			"worker_key": "character varying(128)",
		},
		"judge_worker_capabilities": {
			"judge_spec_protocol":       "character varying(128)",
			"runtime_key":               "character varying(64)",
			"runtime_profile_version":   "character varying(128)",
			"compiler_version":          "character varying(128)",
			"runtime_bootstrap_version": "character varying(128)",
			"compile_command_id":        "character varying(128)",
			"run_command_id":            "character varying(128)",
			"image_digest":              "character varying(71)",
			"manifest_version":          "character varying(128)",
			"manifest_checksum":         "character varying(71)",
			"supervisor_protocol":       "character varying(128)",
			"checker_key":               "character varying(64)",
			"checker_version":           "character varying(64)",
			"capability_profile_hash":   "character varying(71)",
			"smoke_checksum":            "character varying(71)",
		},
		"activated_judge_capabilities": {
			"compiler_version":          "character varying(128)",
			"runtime_bootstrap_version": "character varying(128)",
			"compile_command_id":        "character varying(128)",
			"run_command_id":            "character varying(128)",
		},
		"runtime_profiles": {
			"runtime_key":                "character varying(64)",
			"language":                   "character varying(64)",
			"display_name":               "character varying(128)",
			"profile_version":            "character varying(128)",
			"compiler_version":           "character varying(128)",
			"runtime_bootstrap_version":  "character varying(128)",
			"sandbox_image_digest":       "character varying(71)",
			"compile_command_id":         "character varying(128)",
			"run_command_id":             "character varying(128)",
			"default_compile_timeout_ms": "integer",
			"manifest_version":           "character varying(128)",
			"manifest_checksum":          "character varying(71)",
		},
	}
	for tableName, columns := range wantTypes {
		table := requireTable(t, state.tables, tableName)
		for columnName, wantType := range columns {
			column, ok := table.Column(columnName)
			require.Truef(t, ok, "table %q is missing column %q", tableName, columnName)
			require.Equalf(
				t, wantType, column.SchemaType[dialect.Postgres],
				"%s.%s must declare its bounded PostgreSQL storage type",
				tableName, columnName,
			)
		}
	}

	runtimeProfiles := requireTable(t, state.tables, "runtime_profiles")
	require.Equal(
		t,
		"default_compile_timeout_ms BETWEEN 1 AND 60000",
		runtimeProfiles.Annotation.Checks["runtime_profiles_default_compile_timeout_ms"],
		"compile timeout bounds must be enforced by PostgreSQL, not only by Ent validators",
	)
}

func TestWorkerCapabilityUsesOnlyExactBootCompositeForeignKey(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)
	worker := requireTable(t, state.tables, "judge_worker_instances")
	evidence := requireTable(t, state.tables, "judge_worker_capabilities")
	requireExactUniqueIndex(t, worker, []string{"id", "boot_id"})

	var workerForeignKeys []*entschema.ForeignKey
	for _, foreignKey := range evidence.ForeignKeys {
		if foreignKey != nil && foreignKey.RefTable == worker {
			workerForeignKeys = append(workerForeignKeys, foreignKey)
		}
	}
	require.Len(t, workerForeignKeys, 1, "a weaker single-column worker FK must not remain")
	require.Equal(t, "judge_worker_capabilities_worker_boot_fkey", workerForeignKeys[0].Symbol)
	require.Equal(t, []string{"worker_instance_id", "boot_id"}, columnNames(workerForeignKeys[0].Columns))
	require.Equal(t, []string{"id", "boot_id"}, columnNames(workerForeignKeys[0].RefColumns))
	require.Equal(t, entschema.NoAction, workerForeignKeys[0].OnUpdate)
	require.Equal(t, entschema.NoAction, workerForeignKeys[0].OnDelete)

	ddl, err := state.RelationalDDL(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(ddl, `CONSTRAINT "judge_worker_capabilities_worker_boot_fkey" FOREIGN KEY ("worker_instance_id", "boot_id") REFERENCES "judge_worker_instances" ("id", "boot_id") ON UPDATE NO ACTION ON DELETE NO ACTION`))
	require.NotContains(t, ddl, `FOREIGN KEY ("worker_instance_id") REFERENCES "judge_worker_instances" ("id")`)
}

func TestActivationProofUsesOnlyExactEvidenceCompositeForeignKey(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)
	evidence := requireTable(t, state.tables, "judge_worker_capabilities")
	activation := requireTable(t, state.tables, "activated_judge_capabilities")
	localNames := []string{
		"proof_evidence_id", "proof_worker_instance_id", "proof_boot_id",
		"judge_spec_protocol", "runtime_key", "runtime_profile_version",
		"compiler_version", "runtime_bootstrap_version", "compile_command_id", "run_command_id",
		"image_digest", "manifest_version", "manifest_checksum", "supervisor_protocol",
		"checker_key", "checker_version", "capability_profile_hash",
		"checker_implementation_checksum", "smoke_checksum", "golden_checksum",
	}
	targetNames := append([]string{"id", "worker_instance_id", "boot_id"}, localNames[3:]...)
	requireExactUniqueIndex(t, evidence, targetNames)

	var evidenceForeignKeys []*entschema.ForeignKey
	for _, foreignKey := range activation.ForeignKeys {
		if foreignKey != nil && foreignKey.RefTable == evidence {
			evidenceForeignKeys = append(evidenceForeignKeys, foreignKey)
		}
	}
	require.Len(t, evidenceForeignKeys, 1, "a weaker evidence-ID-only FK must not remain")
	require.Equal(t, "activated_judge_capabilities_worker_evidence_fkey", evidenceForeignKeys[0].Symbol)
	require.Equal(t, localNames, columnNames(evidenceForeignKeys[0].Columns))
	require.Equal(t, targetNames, columnNames(evidenceForeignKeys[0].RefColumns))
	require.Equal(t, entschema.NoAction, evidenceForeignKeys[0].OnUpdate)
	require.Equal(t, entschema.NoAction, evidenceForeignKeys[0].OnDelete)

	ddl, err := state.RelationalDDL(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(ddl, `CONSTRAINT "activated_judge_capabilities_worker_evidence_fkey" FOREIGN KEY (`))
	require.NotContains(t, ddl, `FOREIGN KEY ("proof_evidence_id") REFERENCES "judge_worker_capabilities" ("id")`)
}

func TestContestProblemUsesExactProblemOwnedSnapshots(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)
	contestProblem := requireTable(t, state.tables, contestProblemsTable)

	want := map[string][]string{
		"contest_problems_problem_revision_identity_fkey": {"problem_revision_id", "problem_id", "testset_revision_id"},
		"contest_problems_testset_owner_fkey":             {"testset_revision_id", "problem_id"},
	}
	for symbol, columns := range want {
		var matches []*entschema.ForeignKey
		for _, foreignKey := range contestProblem.ForeignKeys {
			if foreignKey != nil && foreignKey.Symbol == symbol {
				matches = append(matches, foreignKey)
			}
		}
		require.Len(t, matches, 1, symbol)
		require.Equal(t, columns, columnNames(matches[0].Columns))
		require.Equal(t, entschema.NoAction, matches[0].OnUpdate)
		require.Equal(t, entschema.NoAction, matches[0].OnDelete)
	}
}

func TestContestRegistrationTransitionUsesExactRegistrationIdentity(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)
	table := requireTable(t, state.tables, contestRegistrationTransitionsTable)
	var matches []*entschema.ForeignKey
	for _, foreignKey := range table.ForeignKeys {
		if foreignKey != nil && foreignKey.Symbol == "contest_registration_transitions_registration_identity_fkey" {
			matches = append(matches, foreignKey)
		}
	}
	require.Len(t, matches, 1)
	require.Equal(t, []string{"registration_id", "contest_id", "user_id"}, columnNames(matches[0].Columns))
	require.Equal(t, []string{"id", "contest_id", "user_id"}, columnNames(matches[0].RefColumns))
	require.Equal(t, entschema.NoAction, matches[0].OnUpdate)
	require.Equal(t, entschema.NoAction, matches[0].OnDelete)
}

func TestExtendJudgeWorkerCapabilityIdentityFailsClosedAndIsIdempotent(t *testing.T) {
	t.Run("idempotent", func(t *testing.T) {
		tables := validWorkerCapabilityIdentityTables(t)
		require.NoError(t, extendJudgeWorkerCapabilityIdentity(tables))
		require.NoError(t, extendJudgeWorkerCapabilityIdentity(tables))
	})

	tests := []struct {
		name    string
		mutate  func([]*entschema.Table)
		wantErr string
	}{
		{
			name: "missing worker table",
			mutate: func(tables []*entschema.Table) {
				tables[0].Name = "other_workers"
			},
			wantErr: `expected exactly one table "judge_worker_instances", found 0`,
		},
		{
			name: "missing local boot column",
			mutate: func(tables []*entschema.Table) {
				tables[1].Columns = removeColumn(tables[1].Columns, "boot_id")
			},
			wantErr: `expected exactly one column "boot_id" on table "judge_worker_capabilities", found 0`,
		},
		{
			name: "missing exact target unique index",
			mutate: func(tables []*entschema.Table) {
				tables[0].Indexes = nil
			},
			wantErr: `expected exactly one unique index on table "judge_worker_instances" for columns (id, boot_id), found 0`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tables := validWorkerCapabilityIdentityTables(t)
			test.mutate(tables)
			require.ErrorContains(t, extendJudgeWorkerCapabilityIdentity(tables), test.wantErr)
		})
	}
}

func validWorkerCapabilityIdentityTables(t *testing.T) []*entschema.Table {
	t.Helper()
	worker := entschema.NewTable(judgeWorkerInstancesTable)
	worker.AddColumn(&entschema.Column{Name: "id"})
	worker.AddColumn(&entschema.Column{Name: "boot_id"})
	worker.AddIndex("judgeworkerinstance_id_boot_id", true, []string{"id", "boot_id"})
	evidence := entschema.NewTable(judgeWorkerEvidenceTable)
	evidence.AddColumn(&entschema.Column{Name: "worker_instance_id"})
	evidence.AddColumn(&entschema.Column{Name: "boot_id"})
	evidence.AddForeignKey(&entschema.ForeignKey{
		Symbol:   entWorkerEvidenceForeignKey,
		Columns:  requireColumns(t, evidence, "worker_instance_id"),
		RefTable: worker, RefColumns: requireColumns(t, worker, "id"),
		OnDelete: entschema.NoAction,
	})
	return []*entschema.Table{worker, evidence}
}

func TestLoadExtendsActualEntSchemaWithRenderSourceIdentity(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)

	source := requireTable(t, state.tables, contentRenderSourcesTable)
	artifact := requireTable(t, state.tables, contentRenderArtifactsTable)
	requireExactUniqueIndex(t, source, []string{"id", "source_kind", "source_checksum"})

	var single, composite []*entschema.ForeignKey
	for _, fk := range artifact.ForeignKeys {
		switch {
		case fk.Symbol == renderSourceIdentityForeignKey:
			composite = append(composite, fk)
		case fk.RefTable == source && strings.Join(columnNames(fk.Columns), ",") == "render_source_id":
			single = append(single, fk)
		}
	}
	require.Len(t, single, 1, "the Ent edge's single-column foreign key must remain present")
	require.Equal(t, []string{"id"}, columnNames(single[0].RefColumns))
	require.Len(t, composite, 1)
	require.Equal(t, []string{"render_source_id", "render_source_kind", "source_checksum"}, columnNames(composite[0].Columns))
	require.Same(t, source, composite[0].RefTable)
	require.Equal(t, []string{"id", "source_kind", "source_checksum"}, columnNames(composite[0].RefColumns))
	require.Equal(t, entschema.NoAction, composite[0].OnUpdate)
	require.Equal(t, entschema.NoAction, composite[0].OnDelete)

	ddl, err := state.DDL(ctx)
	require.NoError(t, err)
	require.NotContains(t, ddl, "-- atlas:pos ", "DDL must not leak checkout-specific Ent source paths")
	wantConstraint := `CONSTRAINT "content_render_artifacts_render_source_identity_fkey" FOREIGN KEY ("render_source_id", "render_source_kind", "source_checksum") REFERENCES "content_render_sources" ("id", "source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION`
	require.Equal(t, 1, strings.Count(ddl, wantConstraint))
	for _, constraint := range []string{
		`CONSTRAINT "content_render_source_nonzero_identity" CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid AND owner_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid AND (created_by IS NULL OR created_by <> '00000000-0000-0000-0000-000000000000'::uuid))`,
		`CONSTRAINT "content_render_source_nonzero_checksum" CHECK (source_checksum <> decode(repeat('00', 32), 'hex'))`,
		`CONSTRAINT "content_render_artifact_nonzero_identity" CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid AND render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid AND (created_by IS NULL OR created_by <> '00000000-0000-0000-0000-000000000000'::uuid))`,
		`CONSTRAINT "content_render_artifact_nonzero_checksums" CHECK (source_checksum <> decode(repeat('00', 32), 'hex') AND (artifact_checksum IS NULL OR artifact_checksum <> decode(repeat('00', 32), 'hex')))`,
		`CONSTRAINT "content_render_artifact_renderer_version_bound" CHECK (octet_length(renderer_version) BETWEEN 1 AND 64 AND renderer_version ~ '^[a-z0-9._-]+$')`,
		`CONSTRAINT "content_render_artifact_sanitizer_policy_version_bound" CHECK (octet_length(sanitizer_policy_version) BETWEEN 1 AND 64 AND sanitizer_policy_version ~ '^[a-z0-9._-]+$')`,
		`CONSTRAINT "content_render_artifact_diagnostics_safe_codes" CHECK (diagnostics <@ '[{"code":"invalid_utf8"},{"code":"ambiguous_encoding"},{"code":"source_too_large"},{"code":"output_too_large"},{"code":"nesting_too_deep"},{"code":"render_rejected"}]'::jsonb AND NOT (diagnostics @? '$[*] ? (!(exists(@.code)))'))`,
	} {
		require.Equal(t, 1, strings.Count(ddl, constraint), "desired DDL must contain %s exactly once", constraint)
	}
	appendOnlyDDL, err := schemaextras.JudgifyAppendOnlyDDL("")
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(ddl, appendOnlyDDL), "desired DDL must contain the exact append-only block")

	relationalDDL, err := state.RelationalDDL(ctx)
	require.NoError(t, err)
	require.NotContains(t, relationalDDL, appendOnlyDDL)
	require.NotContains(t, relationalDDL, "judgify_reject_append_only_mutation")
	require.Contains(t, relationalDDL, wantConstraint)

	again, err := state.DDL(ctx)
	require.NoError(t, err)
	require.Equal(t, ddl, again, "DDL emission must be deterministic for one desired state")
}

func TestExtendContentRenderIdentityFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func([]*entschema.Table)
		wantErr string
	}{
		{
			name: "missing artifact table",
			mutate: func(tables []*entschema.Table) {
				tables[1].Name = "other_artifacts"
			},
			wantErr: `expected exactly one table "content_render_artifacts", found 0`,
		},
		{
			name: "ambiguous source table",
			mutate: func(tables []*entschema.Table) {
				tables[1] = tables[0]
			},
			wantErr: `expected exactly one table "content_render_sources", found 2`,
		},
		{
			name: "missing artifact column",
			mutate: func(tables []*entschema.Table) {
				artifact := tables[1]
				artifact.Columns = removeColumn(artifact.Columns, "render_source_kind")
			},
			wantErr: `expected exactly one column "render_source_kind" on table "content_render_artifacts", found 0`,
		},
		{
			name: "ambiguous artifact column",
			mutate: func(tables []*entschema.Table) {
				tables[1].Columns = append(tables[1].Columns, &entschema.Column{Name: "source_checksum"})
			},
			wantErr: `expected exactly one column "source_checksum" on table "content_render_artifacts", found 2`,
		},
		{
			name: "missing exact referenced unique index",
			mutate: func(tables []*entschema.Table) {
				source := tables[0]
				source.Indexes[0].Columns = append(source.Indexes[0].Columns, &entschema.Column{Name: "owner_revision_id"})
			},
			wantErr: `expected exactly one unique index on table "content_render_sources" for columns (id, source_kind, source_checksum), found 0`,
		},
		{
			name: "ambiguous referenced unique index",
			mutate: func(tables []*entschema.Table) {
				source := tables[0]
				duplicate := *source.Indexes[0]
				source.Indexes = append(source.Indexes, &duplicate)
			},
			wantErr: `expected exactly one unique index on table "content_render_sources" for columns (id, source_kind, source_checksum), found 2`,
		},
		{
			name: "duplicate symbol",
			mutate: func(tables []*entschema.Table) {
				tables[1].ForeignKeys = append(tables[1].ForeignKeys, &entschema.ForeignKey{Symbol: renderSourceIdentityForeignKey})
			},
			wantErr: `foreign key symbol "content_render_artifacts_render_source_identity_fkey" already exists`,
		},
		{
			name: "duplicate tuple under another symbol",
			mutate: func(tables []*entschema.Table) {
				source, artifact := tables[0], tables[1]
				artifact.ForeignKeys = append(artifact.ForeignKeys, &entschema.ForeignKey{
					Symbol:     "same_tuple_other_name",
					Columns:    requireColumns(t, artifact, "render_source_id", "render_source_kind", "source_checksum"),
					RefTable:   source,
					RefColumns: requireColumns(t, source, "id", "source_kind", "source_checksum"),
				})
			},
			wantErr: `composite foreign key tuple already exists as "same_tuple_other_name"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tables := validContentRenderTables(t)
			test.mutate(tables)

			err := extendContentRenderIdentity(tables)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestLoadExtendsActualEntSchemaWithEveryProblemIdentity(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)
	ddl, err := state.DDL(ctx)
	require.NoError(t, err)

	foreignKeys := problemIdentityForeignKeysForTest()
	require.Len(t, foreignKeys, 15, "the reviewed relational identity catalog must remain closed")
	for _, expected := range foreignKeys {
		expected := expected
		t.Run(expected.symbol, func(t *testing.T) {
			table := requireTable(t, state.tables, expected.table)
			refTable := requireTable(t, state.tables, expected.refTable)
			requireExactUniqueIndex(t, refTable, expected.refColumns)

			var matches []*entschema.ForeignKey
			for _, foreignKey := range table.ForeignKeys {
				if foreignKey != nil && foreignKey.Symbol == expected.symbol {
					matches = append(matches, foreignKey)
				}
			}
			require.Len(t, matches, 1)
			require.Equal(t, expected.columns, columnNames(matches[0].Columns))
			require.Same(t, refTable, matches[0].RefTable)
			require.Equal(t, expected.refColumns, columnNames(matches[0].RefColumns))
			require.Equal(t, entschema.NoAction, matches[0].OnUpdate)
			require.Equal(t, entschema.NoAction, matches[0].OnDelete)

			constraint := `CONSTRAINT "` + expected.symbol + `" FOREIGN KEY ("` +
				strings.Join(expected.columns, `", "`) + `") REFERENCES "` +
				expected.refTable + `" ("` + strings.Join(expected.refColumns, `", "`) +
				`") ON UPDATE NO ACTION ON DELETE NO ACTION`
			require.Equal(t, 1, strings.Count(ddl, constraint))
		})
	}
}

func TestLoadExtendsActualEntSchemaWithEveryMaterialIdentity(t *testing.T) {
	t.Chdir("../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	state, err := Load(ctx)
	require.NoError(t, err)
	ddl, err := state.DDL(ctx)
	require.NoError(t, err)

	foreignKeys := materialIdentityForeignKeys()
	require.Len(t, foreignKeys, 7, "the reviewed material identity catalog must remain closed")
	for _, expected := range foreignKeys {
		table := requireTable(t, state.tables, expected.table)
		refTable := requireTable(t, state.tables, expected.refTable)
		requireExactUniqueIndex(t, refTable, expected.refColumns)

		var matches []*entschema.ForeignKey
		for _, foreignKey := range table.ForeignKeys {
			if foreignKey != nil && foreignKey.Symbol == expected.symbol {
				matches = append(matches, foreignKey)
			}
		}
		require.Len(t, matches, 1, expected.symbol)
		require.Equal(t, expected.columns, columnNames(matches[0].Columns))
		require.Same(t, refTable, matches[0].RefTable)
		require.Equal(t, expected.refColumns, columnNames(matches[0].RefColumns))
		require.Equal(t, entschema.NoAction, matches[0].OnUpdate)
		require.Equal(t, entschema.NoAction, matches[0].OnDelete)

		constraint := `CONSTRAINT "` + expected.symbol + `" FOREIGN KEY ("` +
			strings.Join(expected.columns, `", "`) + `") REFERENCES "` +
			expected.refTable + `" ("` + strings.Join(expected.refColumns, `", "`) +
			`") ON UPDATE NO ACTION ON DELETE NO ACTION`
		require.Equal(t, 1, strings.Count(ddl, constraint))
	}
}

func TestProblemIdentityForeignKeysFailClosedForEveryTarget(t *testing.T) {
	for _, foreignKey := range problemIdentityForeignKeysForTest() {
		foreignKey := foreignKey
		t.Run(foreignKey.symbol, func(t *testing.T) {
			localColumn := foreignKey.columns[0]
			targetColumn := foreignKey.refColumns[0]

			tests := []struct {
				name    string
				mutate  func([]*entschema.Table)
				wantErr string
			}{
				{
					name: "missing local column",
					mutate: func(tables []*entschema.Table) {
						table := requireTable(t, tables, foreignKey.table)
						table.Columns = removeColumn(table.Columns, localColumn)
					},
					wantErr: `expected exactly one column "` + localColumn +
						`" on table "` + foreignKey.table + `", found 0`,
				},
				{
					name: "ambiguous local column",
					mutate: func(tables []*entschema.Table) {
						table := requireTable(t, tables, foreignKey.table)
						table.Columns = append(table.Columns, &entschema.Column{Name: localColumn})
					},
					wantErr: `expected exactly one column "` + localColumn +
						`" on table "` + foreignKey.table + `", found 2`,
				},
				{
					name: "missing target column",
					mutate: func(tables []*entschema.Table) {
						table := requireTable(t, tables, foreignKey.refTable)
						table.Columns = removeColumn(table.Columns, targetColumn)
					},
					wantErr: `expected exactly one column "` + targetColumn +
						`" on table "` + foreignKey.refTable + `", found 0`,
				},
				{
					name: "ambiguous target column",
					mutate: func(tables []*entschema.Table) {
						table := requireTable(t, tables, foreignKey.refTable)
						table.Columns = append(table.Columns, &entschema.Column{Name: targetColumn})
					},
					wantErr: `expected exactly one column "` + targetColumn +
						`" on table "` + foreignKey.refTable + `", found 2`,
				},
				{
					name: "missing exact target unique index",
					mutate: func(tables []*entschema.Table) {
						table := requireTable(t, tables, foreignKey.refTable)
						table.Indexes = nil
					},
					wantErr: `expected exactly one unique index on table "` +
						foreignKey.refTable + `" for columns (` +
						strings.Join(foreignKey.refColumns, ", ") + `), found 0`,
				},
				{
					name: "ambiguous exact target unique index",
					mutate: func(tables []*entschema.Table) {
						table := requireTable(t, tables, foreignKey.refTable)
						duplicate := *table.Indexes[0]
						table.Indexes = append(table.Indexes, &duplicate)
					},
					wantErr: `expected exactly one unique index on table "` +
						foreignKey.refTable + `" for columns (` +
						strings.Join(foreignKey.refColumns, ", ") + `), found 2`,
				},
			}

			for _, test := range tests {
				test := test
				t.Run(test.name, func(t *testing.T) {
					tables := validCompositeForeignKeyTables(t, foreignKey)
					test.mutate(tables)

					err := addCompositeForeignKeys(tables, []compositeForeignKey{foreignKey})
					require.ErrorContains(t, err, test.wantErr)
				})
			}
		})
	}
}

func TestLoadHonorsAlreadyCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Load(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func validContentRenderTables(t *testing.T) []*entschema.Table {
	t.Helper()
	source := entschema.NewTable(contentRenderSourcesTable)
	for _, name := range []string{"id", "source_kind", "source_checksum", "owner_revision_id"} {
		source.AddColumn(&entschema.Column{Name: name})
	}
	source.AddIndex("source_identity", true, []string{"id", "source_kind", "source_checksum"})

	artifact := entschema.NewTable(contentRenderArtifactsTable)
	for _, name := range []string{"id", "render_source_id", "render_source_kind", "source_checksum"} {
		artifact.AddColumn(&entschema.Column{Name: name})
	}
	artifact.AddForeignKey(&entschema.ForeignKey{
		Symbol:     "ent_single_source_fk",
		Columns:    requireColumns(t, artifact, "render_source_id"),
		RefTable:   source,
		RefColumns: requireColumns(t, source, "id"),
		OnDelete:   entschema.NoAction,
	})
	return []*entschema.Table{source, artifact}
}

func validCompositeForeignKeyTables(
	t *testing.T,
	foreignKey compositeForeignKey,
) []*entschema.Table {
	t.Helper()

	table := entschema.NewTable(foreignKey.table)
	for _, name := range foreignKey.columns {
		table.AddColumn(&entschema.Column{Name: name})
	}
	refTable := entschema.NewTable(foreignKey.refTable)
	for _, name := range foreignKey.refColumns {
		refTable.AddColumn(&entschema.Column{Name: name})
	}
	refTable.AddIndex("target_identity", true, foreignKey.refColumns)
	return []*entschema.Table{table, refTable}
}

func problemIdentityForeignKeysForTest() []compositeForeignKey {
	sourceOwnerIdentity := []string{"id", "owner_revision_id", "source_kind", "source_checksum"}
	revisionSourceIdentity := []string{"id", "render_source_id", "render_source_kind", "render_source_checksum"}
	revisionOwnerIdentity := []string{"id", "problem_id"}
	testsetOwnerIdentity := []string{"id", "problem_id"}
	groupIdentity := []string{"id", "testset_revision_id", "group_index", "stop_on_failure"}

	return []compositeForeignKey{
		{
			symbol: "problem_revisions_render_source_identity_fkey",
			table:  problemRevisionsTable,
			columns: []string{
				"render_source_id", "id", "render_source_kind", "render_source_checksum",
			},
			refTable: contentRenderSourcesTable, refColumns: sourceOwnerIdentity,
		},
		{
			symbol: "problem_revisions_testset_owner_fkey",
			table:  problemRevisionsTable, columns: []string{"testset_revision_id", "problem_id"},
			refTable: testsetRevisionsTable, refColumns: testsetOwnerIdentity,
		},
		{
			symbol:   "tag_revision_elements_element_revision_fkey",
			table:    tagRevisionElementsTable,
			columns:  []string{"element_id", "element_revision"},
			refTable: elementRevisionsTable, refColumns: []string{"element_id", "revision"},
		},
		{
			symbol:   "problem_revisions_difficulty_revision_fkey",
			table:    problemRevisionsTable,
			columns:  []string{"difficulty_id", "difficulty_revision"},
			refTable: difficultyRevisionsTable, refColumns: []string{"difficulty_id", "revision"},
		},
		{
			symbol:   "problem_revisions_tag_mapping_owner_fkey",
			table:    problemRevisionsTable,
			columns:  []string{"problem_id", "tag_mapping_id"},
			refTable: problemsTable, refColumns: []string{"id", "tag_mapping_id"},
		},
		{
			symbol:   "problem_revisions_element_mapping_owner_fkey",
			table:    problemRevisionsTable,
			columns:  []string{"problem_id", "element_mapping_id"},
			refTable: problemsTable, refColumns: []string{"id", "element_mapping_id"},
		},
		{
			symbol: "problems_draft_revision_source_fkey",
			table:  problemsTable,
			columns: []string{
				"draft_revision_id", "draft_render_source_id",
				"draft_render_source_kind", "draft_render_source_checksum",
			},
			refTable: problemRevisionsTable, refColumns: revisionSourceIdentity,
		},
		{
			symbol: "problems_draft_artifact_source_fkey",
			table:  problemsTable,
			columns: []string{
				"draft_render_artifact_id", "draft_render_source_id",
				"draft_render_source_kind", "draft_render_source_checksum",
			},
			refTable:   contentRenderArtifactsTable,
			refColumns: []string{"id", "render_source_id", "render_source_kind", "source_checksum"},
		},
		{
			symbol: "problems_draft_revision_owner_fkey",
			table:  problemsTable, columns: []string{"draft_revision_id", "id"},
			refTable: problemRevisionsTable, refColumns: revisionOwnerIdentity,
		},
		{
			symbol: "problems_published_revision_source_fkey",
			table:  problemsTable,
			columns: []string{
				"published_revision_id", "published_render_source_id",
				"published_render_source_kind", "published_render_source_checksum",
			},
			refTable: problemRevisionsTable, refColumns: revisionSourceIdentity,
		},
		{
			symbol: "problems_published_artifact_source_fkey",
			table:  problemsTable,
			columns: []string{
				"published_render_artifact_id", "published_render_source_id",
				"published_render_source_kind", "published_render_source_checksum",
			},
			refTable:   contentRenderArtifactsTable,
			refColumns: []string{"id", "render_source_id", "render_source_kind", "source_checksum"},
		},
		{
			symbol: "problems_published_revision_owner_fkey",
			table:  problemsTable, columns: []string{"published_revision_id", "id"},
			refTable: problemRevisionsTable, refColumns: revisionOwnerIdentity,
		},
		{
			symbol: "testcase_revisions_test_case_owner_fkey",
			table:  testcaseRevisionsTable, columns: []string{"test_case_id", "problem_id"},
			refTable: testCasesTable, refColumns: []string{"id", "problem_id"},
		},
		{
			symbol: "testcase_revisions_testset_owner_fkey",
			table:  testcaseRevisionsTable, columns: []string{"testset_revision_id", "problem_id"},
			refTable: testsetRevisionsTable, refColumns: testsetOwnerIdentity,
		},
		{
			symbol: "testcase_revisions_group_identity_fkey",
			table:  testcaseRevisionsTable,
			columns: []string{
				"group_id", "testset_revision_id", "group_index", "stop_on_failure",
			},
			refTable: testcaseGroupsTable, refColumns: groupIdentity,
		},
	}
}

func requireTable(t *testing.T, tables []*entschema.Table, name string) *entschema.Table {
	t.Helper()
	var matches []*entschema.Table
	for _, table := range tables {
		if table != nil && table.Name == name {
			matches = append(matches, table)
		}
	}
	require.Len(t, matches, 1)
	return matches[0]
}

func requireColumns(t *testing.T, table *entschema.Table, names ...string) []*entschema.Column {
	t.Helper()
	columns := make([]*entschema.Column, 0, len(names))
	for _, name := range names {
		column, ok := table.Column(name)
		require.Truef(t, ok, "missing test fixture column %q", name)
		columns = append(columns, column)
	}
	return columns
}

func requireExactUniqueIndex(t *testing.T, table *entschema.Table, names []string) {
	t.Helper()
	var matches int
	for _, index := range table.Indexes {
		if index.Unique && strings.Join(columnNames(index.Columns), ",") == strings.Join(names, ",") {
			matches++
		}
	}
	require.Equal(t, 1, matches)
}

func removeColumn(columns []*entschema.Column, name string) []*entschema.Column {
	kept := columns[:0]
	for _, column := range columns {
		if column.Name != name {
			kept = append(kept, column)
		}
	}
	return kept
}

func columnNames(columns []*entschema.Column) []string {
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		if column == nil {
			names = append(names, "<nil>")
			continue
		}
		names = append(names, column.Name)
	}
	return names
}
