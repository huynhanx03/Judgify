package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

// ContentRenderArtifact stores one immutable sanitized result for an exact
// source and pinned renderer/sanitizer tuple.
type ContentRenderArtifact struct{ ent.Schema }

func (ContentRenderArtifact) Mixin() []ent.Mixin {
	return []ent.Mixin{
		commonent.AppendOnlyMixin{},
	}
}

func (ContentRenderArtifact) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(commonent.NewUUID).
			Immutable(),
		field.UUID("render_source_id", uuid.UUID{}).
			Immutable(),
		field.Enum("render_source_kind").
			Values(contentRenderSourceKinds...).
			Immutable(),
		field.Bytes("source_checksum").
			NotEmpty().
			Immutable(),
		field.String("renderer_version").
			NotEmpty().
			MaxLen(rendercontract.MaximumPipelineVersionBytes).
			Immutable(),
		field.String("sanitizer_policy_version").
			NotEmpty().
			MaxLen(rendercontract.MaximumPipelineVersionBytes).
			Immutable(),
		field.Text("sanitized_html").
			Optional().
			Nillable().
			Immutable().
			Sensitive(),
		field.Bytes("artifact_checksum").
			Optional().
			Nillable().
			Immutable(),
		field.Enum("status").
			Values(rendercontract.Statuses()...).
			Immutable(),
		field.JSON("diagnostics", json.RawMessage{}).
			SchemaType(map[string]string{
				dialect.Postgres: "jsonb",
				dialect.SQLite:   "json",
			}).
			Immutable().
			Sensitive(),
		field.UUID("created_by", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().
			Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (ContentRenderArtifact) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("render_source", ContentRenderSource.Type).
			Ref("artifacts").
			Field("render_source_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (ContentRenderArtifact) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id", "render_source_id", "render_source_kind", "source_checksum").Unique(),
		index.Fields("render_source_id", "renderer_version", "sanitizer_policy_version").Unique(),
		index.Fields("render_source_id", "render_source_kind", "source_checksum"),
	}
}

func (ContentRenderArtifact) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"content_render_artifact_renderer_version_bound": fmt.Sprintf(
				"octet_length(renderer_version) BETWEEN 1 AND %d AND renderer_version ~ '^[a-z0-9._-]+$'",
				rendercontract.MaximumPipelineVersionBytes,
			),
			"content_render_artifact_sanitizer_policy_version_bound": fmt.Sprintf(
				"octet_length(sanitizer_policy_version) BETWEEN 1 AND %d AND sanitizer_policy_version ~ '^[a-z0-9._-]+$'",
				rendercontract.MaximumPipelineVersionBytes,
			),
			"content_render_artifact_source_checksum_length": "octet_length(source_checksum) = 32",
			"content_render_artifact_nonzero_identity": "id <> " + contentRenderNilUUIDSQL +
				" AND render_source_id <> " + contentRenderNilUUIDSQL +
				" AND " + contentRenderNonzeroActorSQL,
			"content_render_artifact_nonzero_checksums": "source_checksum <> " + contentRenderZeroDigestSQL +
				" AND (artifact_checksum IS NULL OR artifact_checksum <> " + contentRenderZeroDigestSQL + ")",
			"content_render_artifact_source_kind_closed": contentRenderClosedDomainCheck("render_source_kind", contentRenderSourceKinds),
			"content_render_artifact_status_closed":      contentRenderClosedDomainCheck("status", rendercontract.Statuses()),
			"content_render_artifact_ready_shape": fmt.Sprintf(`status <> '%s' OR (
				sanitized_html IS NOT NULL AND
				artifact_checksum IS NOT NULL AND
				octet_length(artifact_checksum) = 32 AND
				jsonb_array_length(diagnostics) = 0
			)`, rendercontract.StatusReady),
			"content_render_artifact_rejected_shape": fmt.Sprintf(`status <> '%s' OR (
				sanitized_html IS NULL AND
				artifact_checksum IS NULL AND
				jsonb_array_length(diagnostics) BETWEEN 1 AND %d
			)`, rendercontract.StatusRejected, rendercontract.MaximumDiagnostics),
			"content_render_artifact_html_bound": fmt.Sprintf(
				"sanitized_html IS NULL OR octet_length(sanitized_html) <= %d",
				rendercontract.MaximumRenderedHTMLBytes,
			),
			"content_render_artifact_diagnostics_bound": `jsonb_typeof(diagnostics) = 'array' AND
				jsonb_array_length(diagnostics) <= ` + fmt.Sprint(rendercontract.MaximumDiagnostics) + ` AND
				octet_length(diagnostics::text) <= ` + fmt.Sprint(rendercontract.MaximumDiagnosticsJSONBytes),
			"content_render_artifact_diagnostics_safe_codes": contentRenderDiagnosticsSafeCodesCheck(),
		}),
	}
}

func contentRenderDiagnosticsSafeCodesCheck() string {
	values := make([]struct {
		Code string `json:"code"`
	}, 0, rendercontract.MaximumDiagnostics)
	for _, code := range rendercontract.DiagnosticCodes() {
		values = append(values, struct {
			Code string `json:"code"`
		}{Code: code})
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf("marshal content-render diagnostic contract: %v", err))
	}
	allowed := strings.ReplaceAll(string(encoded), "'", "''")
	return "diagnostics <@ '" + allowed + "'::jsonb AND " +
		`NOT (diagnostics @? '$[*] ? (!(exists(@.code)))')`
}
