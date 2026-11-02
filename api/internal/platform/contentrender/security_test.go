package contentrender

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	dbschema "github.com/huynhanx03/judgify/internal/ent/schema"
)

func TestRendererHostileCorpusFailsClosed(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	tests := []struct {
		name     string
		markdown string
		forbid   []string
		preserve []string
	}{
		{
			name:     "raw HTML scripts and event attributes",
			markdown: `<div onclick="alert(1)"><script>alert(1)</script><b onmouseover="x">unsafe</b></div>`,
			forbid:   []string{"<script", "onclick", "onmouseover", "<div", "<b"},
		},
		{
			name:     "javascript URL",
			markdown: `[click](javascript:alert(document.domain))`,
			forbid:   []string{`href="javascript:`, `src="javascript:`},
		},
		{
			name:     "entity encoded javascript URL",
			markdown: `[click](javascript&colon;alert(document.domain))`,
			forbid:   []string{`href="javascript:`, `href="javascript&colon;`, `src="javascript:`},
		},
		{
			name:     "scheme relative external URL",
			markdown: `[click](//evil.example/path)`,
			forbid:   []string{`href="//evil.example/path"`, `href="\\evil.example`},
		},
		{
			name:     "data URL",
			markdown: `[click](data:text/html;base64,PHNjcmlwdD4=)`,
			forbid:   []string{`href="data:`, `src="data:`},
		},
		{
			name:     "hostile SVG image",
			markdown: `![x](data:image/svg+xml,<svg onload=alert(1)></svg>)`,
			forbid:   []string{"<img", "<svg", `src="data:`, `onload="`},
		},
		{
			name:     "raw SVG",
			markdown: `<svg><foreignObject><script>alert(1)</script></foreignObject></svg>`,
			forbid:   []string{"<svg", "foreignobject", "<script"},
		},
		{
			name:     "malicious fenced code is escaped",
			markdown: "```html\n<img src=x onerror=alert(1)><script>alert(1)</script>\n```",
			forbid:   []string{"<img", "<script"},
			preserve: []string{"&lt;img src=x onerror=alert(1)&gt;", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, requestErr := NewRenderRequest(uuid.New(), ProblemStatement, []byte(test.markdown))
			require.NoError(t, requestErr)
			artifact, renderErr := renderer.Render(context.Background(), request)
			require.NoError(t, renderErr)
			require.Equal(t, StatusReady, artifact.Status)
			lowerHTML := strings.ToLower(artifact.SanitizedHTML)
			for _, forbidden := range test.forbid {
				require.NotContains(t, lowerHTML, strings.ToLower(forbidden))
			}
			for _, preserved := range test.preserve {
				require.Contains(t, artifact.SanitizedHTML, preserved)
			}
		})
	}
}

func TestRendererAddsSafeExternalLinkAttributes(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	request, err := NewRenderRequest(uuid.New(), ContestAnnouncement, []byte(`[safe](https://example.com/path)`))
	require.NoError(t, err)
	artifact, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	require.Contains(t, artifact.SanitizedHTML, `href="https://example.com/path"`)
	require.Contains(t, artifact.SanitizedHTML, `rel="nofollow noreferrer noopener"`)
	require.Contains(t, artifact.SanitizedHTML, `target="_blank"`)
}

func TestArtifactValidationRejectsUnsafeOrUnboundedDiagnostics(t *testing.T) {
	t.Parallel()

	artifact := rejectedArtifact(
		CurrentRendererVersion,
		CurrentSanitizerPolicyVersion,
		DefaultMaxDiagnostics,
		DiagnosticInvalidUTF8,
	)
	require.NoError(t, artifact.Validate())

	artifact.Diagnostics[0].Code = "secret-user-content"
	require.Error(t, artifact.Validate())
	artifact.Diagnostics = make([]Diagnostic, DefaultMaxDiagnostics+1)
	for index := range artifact.Diagnostics {
		artifact.Diagnostics[index] = Diagnostic{Code: DiagnosticRenderRejected}
	}
	require.Error(t, artifact.Validate())
}

func TestRendererRejectsMalformedUTF8AndExcessiveInputWithSafeDiagnostics(t *testing.T) {
	t.Parallel()

	renderer, err := NewRenderer()
	require.NoError(t, err)
	sourceID := uuid.New()
	tests := []struct {
		name     string
		input    []byte
		wantCode string
	}{
		{name: "malformed UTF-8", input: []byte{0xff, 0xfe, 0xfd}, wantCode: DiagnosticInvalidUTF8},
		{name: "NUL byte", input: []byte("safe\x00hidden"), wantCode: DiagnosticAmbiguousEncoding},
		{name: "source too large", input: []byte(strings.Repeat("x", DefaultMaxSourceBytes+1)), wantCode: DiagnosticSourceTooLarge},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			artifact, renderErr := renderer.Render(context.Background(), RenderRequest{
				SourceID:           sourceID,
				SourceKind:         ProblemStatement,
				SourceChecksum:     sha256.Sum256(test.input),
				NormalizedMarkdown: test.input,
			})
			require.NoError(t, renderErr)
			require.Equal(t, StatusRejected, artifact.Status)
			require.Empty(t, artifact.SanitizedHTML)
			require.Equal(t, [32]byte{}, artifact.ArtifactChecksum)
			require.Equal(t, []Diagnostic{{Code: test.wantCode}}, artifact.Diagnostics)
			diagnosticJSON, marshalErr := json.Marshal(artifact.Diagnostics)
			require.NoError(t, marshalErr)
			require.NotContains(t, string(diagnosticJSON), string(test.input))
		})
	}
}

func TestRendererRejectsExcessiveNestingAndOutput(t *testing.T) {
	t.Parallel()

	t.Run("nesting", func(t *testing.T) {
		renderer, err := NewRenderer(WithLimits(Limits{
			MaxSourceBytes:       64 * 1024,
			MaxRenderedHTMLBytes: 128 * 1024,
			MaxNestingDepth:      8,
			MaxDiagnostics:       4,
		}))
		require.NoError(t, err)
		request, err := NewRenderRequest(uuid.New(), MaterialBody, []byte(strings.Repeat("> ", 32)+"deep"))
		require.NoError(t, err)
		artifact, err := renderer.Render(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, StatusRejected, artifact.Status)
		require.Equal(t, []Diagnostic{{Code: DiagnosticNestingTooDeep}}, artifact.Diagnostics)
	})

	t.Run("output", func(t *testing.T) {
		renderer, err := NewRenderer(WithLimits(Limits{
			MaxSourceBytes:       16 * 1024,
			MaxRenderedHTMLBytes: 64,
			MaxNestingDepth:      16,
			MaxDiagnostics:       4,
		}))
		require.NoError(t, err)
		request, err := NewRenderRequest(uuid.New(), NotificationCampaignBody, []byte(strings.Repeat("**x** ", 32)))
		require.NoError(t, err)
		artifact, err := renderer.Render(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, StatusRejected, artifact.Status)
		require.Equal(t, []Diagnostic{{Code: DiagnosticOutputTooLarge}}, artifact.Diagnostics)
	})
}

func TestContentRenderSchemasKeepIdentityAndArtifactFactsImmutable(t *testing.T) {
	t.Parallel()

	source := dbschema.ContentRenderSource{}
	requireAppendOnlyMixin(t, source.Mixin())
	requireImmutableFields(t, source.Fields(), "id", "owner_revision_id", "source_kind", "source_checksum", "created_by", "created_at")
	requireImmutableEdge(t, source.Edges(), "artifacts")
	requireUniqueIndex(t, source.Indexes(), "source_kind", "owner_revision_id")
	requireUniqueIndex(t, source.Indexes(), "id", "source_kind", "source_checksum")
	requireUniqueIndex(t, source.Indexes(), "id", "owner_revision_id", "source_kind", "source_checksum")
	require.Equal(t, map[string]string{
		"content_render_source_checksum_length": "octet_length(source_checksum) = 32",
		"content_render_source_nonzero_identity": "id <> '00000000-0000-0000-0000-000000000000'::uuid AND " +
			"owner_revision_id <> '00000000-0000-0000-0000-000000000000'::uuid AND " +
			"(created_by IS NULL OR created_by <> '00000000-0000-0000-0000-000000000000'::uuid)",
		"content_render_source_nonzero_checksum": "source_checksum <> decode(repeat('00', 32), 'hex')",
		"content_render_source_kind_closed": "source_kind IN (" +
			"'problem_statement', 'contest_description', 'contest_announcement', " +
			"'contest_clarification', 'material_body', 'notification_campaign_body')",
	}, normalizedSQLChecks(source.Annotations()))

	artifact := dbschema.ContentRenderArtifact{}
	requireAppendOnlyMixin(t, artifact.Mixin())
	requireImmutableFields(t, artifact.Fields(),
		"id", "render_source_id", "render_source_kind", "source_checksum",
		"renderer_version", "sanitizer_policy_version", "sanitized_html",
		"artifact_checksum", "status", "diagnostics", "created_by", "created_at",
	)
	requireImmutableEdge(t, artifact.Edges(), "render_source")
	requireUniqueIndex(t, artifact.Indexes(), "id", "render_source_id", "render_source_kind", "source_checksum")
	requireUniqueIndex(t, artifact.Indexes(), "render_source_id", "renderer_version", "sanitizer_policy_version")

	require.Equal(t, map[string]string{
		"content_render_artifact_renderer_version_bound": "octet_length(renderer_version) BETWEEN 1 AND 64 AND " +
			"renderer_version ~ '^[a-z0-9._-]+$'",
		"content_render_artifact_sanitizer_policy_version_bound": "octet_length(sanitizer_policy_version) BETWEEN 1 AND 64 AND " +
			"sanitizer_policy_version ~ '^[a-z0-9._-]+$'",
		"content_render_artifact_source_checksum_length": "octet_length(source_checksum) = 32",
		"content_render_artifact_nonzero_identity": "id <> '00000000-0000-0000-0000-000000000000'::uuid AND " +
			"render_source_id <> '00000000-0000-0000-0000-000000000000'::uuid AND " +
			"(created_by IS NULL OR created_by <> '00000000-0000-0000-0000-000000000000'::uuid)",
		"content_render_artifact_nonzero_checksums": "source_checksum <> decode(repeat('00', 32), 'hex') AND " +
			"(artifact_checksum IS NULL OR artifact_checksum <> decode(repeat('00', 32), 'hex'))",
		"content_render_artifact_source_kind_closed": "render_source_kind IN (" +
			"'problem_statement', 'contest_description', 'contest_announcement', " +
			"'contest_clarification', 'material_body', 'notification_campaign_body')",
		"content_render_artifact_status_closed": "status IN ('ready', 'rejected')",
		"content_render_artifact_ready_shape": "status <> 'ready' OR ( sanitized_html IS NOT NULL AND " +
			"artifact_checksum IS NOT NULL AND octet_length(artifact_checksum) = 32 AND " +
			"jsonb_array_length(diagnostics) = 0 )",
		"content_render_artifact_rejected_shape": "status <> 'rejected' OR ( sanitized_html IS NULL AND " +
			"artifact_checksum IS NULL AND jsonb_array_length(diagnostics) BETWEEN 1 AND 8 )",
		"content_render_artifact_html_bound": "sanitized_html IS NULL OR octet_length(sanitized_html) <= 2097152",
		"content_render_artifact_diagnostics_bound": "jsonb_typeof(diagnostics) = 'array' AND " +
			"jsonb_array_length(diagnostics) <= 8 AND octet_length(diagnostics::text) <= 2048",
		"content_render_artifact_diagnostics_safe_codes": "diagnostics <@ " +
			`'[{"code":"invalid_utf8"},{"code":"ambiguous_encoding"},{"code":"source_too_large"},` +
			`{"code":"output_too_large"},{"code":"nesting_too_deep"},{"code":"render_rejected"}]'::jsonb AND ` +
			`NOT (diagnostics @? '$[*] ? (!(exists(@.code)))')`,
	}, normalizedSQLChecks(artifact.Annotations()))
}

func requireAppendOnlyMixin(t *testing.T, mixins []ent.Mixin) {
	t.Helper()
	for _, schemaMixin := range mixins {
		if _, ok := schemaMixin.(commonent.AppendOnlyMixin); ok {
			return
		}
	}
	t.Fatal("schema is missing go-common AppendOnlyMixin")
}

func requireImmutableEdge(t *testing.T, edges []ent.Edge, name string) {
	t.Helper()
	for _, schemaEdge := range edges {
		descriptor := schemaEdge.Descriptor()
		if descriptor.Name == name {
			require.Truef(t, descriptor.Immutable, "edge %q is mutable", name)
			return
		}
	}
	t.Fatalf("edge %q is missing", name)
}

func requireImmutableFields(t *testing.T, fields []ent.Field, names ...string) {
	t.Helper()
	descriptors := make(map[string]bool, len(fields))
	for _, schemaField := range fields {
		descriptor := schemaField.Descriptor()
		require.NoError(t, descriptor.Err)
		descriptors[descriptor.Name] = descriptor.Immutable
	}
	for _, name := range names {
		immutable, ok := descriptors[name]
		require.Truef(t, ok, "field %q is missing", name)
		require.Truef(t, immutable, "field %q is mutable", name)
	}
}

func requireUniqueIndex(t *testing.T, indexes []ent.Index, fields ...string) {
	t.Helper()
	for _, schemaIndex := range indexes {
		descriptor := schemaIndex.Descriptor()
		if descriptor.Unique && strings.Join(descriptor.Fields, "\x00") == strings.Join(fields, "\x00") {
			return
		}
	}
	t.Fatalf("missing unique index on %v", fields)
}

func entSQLChecks(annotations []entschema.Annotation) map[string]string {
	checks := make(map[string]string)
	for _, annotation := range annotations {
		switch value := annotation.(type) {
		case entsql.Annotation:
			for name, expression := range value.Checks {
				checks[name] = expression
			}
		case *entsql.Annotation:
			if value != nil {
				for name, expression := range value.Checks {
					checks[name] = expression
				}
			}
		}
	}
	return checks
}

func normalizedSQLChecks(annotations []entschema.Annotation) map[string]string {
	checks := entSQLChecks(annotations)
	for name, expression := range checks {
		checks[name] = strings.Join(strings.Fields(expression), " ")
	}
	return checks
}
