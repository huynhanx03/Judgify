package schema

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
	rendercontract "github.com/huynhanx03/judgify/internal/platform/contentrender/contract"
)

var contentRenderSourceKinds = rendercontract.SourceKinds()

const (
	contentRenderNilUUIDSQL      = "'00000000-0000-0000-0000-000000000000'::uuid"
	contentRenderZeroDigestSQL   = "decode(repeat('00', 32), 'hex')"
	contentRenderNonzeroActorSQL = "(created_by IS NULL OR created_by <> " + contentRenderNilUUIDSQL + ")"
)

// ContentRenderSource is the shared immutable parent registry that relationally
// binds one typed authored source to its exact domain revision and checksum.
type ContentRenderSource struct{ ent.Schema }

func (ContentRenderSource) Mixin() []ent.Mixin {
	return []ent.Mixin{
		commonent.AppendOnlyMixin{},
	}
}

func (ContentRenderSource) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(commonent.NewUUID).
			Immutable(),
		field.UUID("owner_revision_id", uuid.UUID{}).
			Immutable(),
		field.Enum("source_kind").
			Values(contentRenderSourceKinds...).
			Immutable(),
		field.Bytes("source_checksum").
			NotEmpty().
			Immutable(),
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

func (ContentRenderSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("artifacts", ContentRenderArtifact.Type).
			Immutable(),
	}
}

func (ContentRenderSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_kind", "owner_revision_id").Unique(),
		index.Fields("id", "source_kind", "source_checksum").Unique(),
		index.Fields("id", "owner_revision_id", "source_kind", "source_checksum").Unique(),
	}
}

func (ContentRenderSource) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"content_render_source_checksum_length": "octet_length(source_checksum) = 32",
			"content_render_source_nonzero_identity": "id <> " + contentRenderNilUUIDSQL +
				" AND owner_revision_id <> " + contentRenderNilUUIDSQL +
				" AND " + contentRenderNonzeroActorSQL,
			"content_render_source_nonzero_checksum": "source_checksum <> " + contentRenderZeroDigestSQL,
			"content_render_source_kind_closed":      contentRenderClosedDomainCheck("source_kind", contentRenderSourceKinds),
		}),
	}
}

func contentRenderClosedDomainCheck(column string, values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+strings.ReplaceAll(value, "'", "''")+"'")
	}
	return column + " IN (" + strings.Join(quoted, ", ") + ")"
}
