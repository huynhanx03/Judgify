package schema

import (
	"time"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// MaterialViewDaily is a privacy-preserving daily idempotency record. It
// stores only a keyed digest, never a raw IP, cookie, user, or session value.
type MaterialViewDaily struct{ ent.Schema }

func (MaterialViewDaily) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("material_id", uuid.UUID{}).Immutable(),
		field.Time("view_day").Immutable().SchemaType(map[string]string{"postgres": "date"}),
		field.Bytes("viewer_fingerprint").NotEmpty().Immutable().Sensitive(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Time("projected_at").Optional().Nillable(),
	}
}

func (MaterialViewDaily) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("material", Material.Type).
			Ref("daily_views").
			Field("material_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (MaterialViewDaily) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("material_id", "view_day", "viewer_fingerprint").Unique(),
		index.Fields("view_day", "id"),
		index.Fields("projected_at", "view_day", "id"),
	}
}

func (MaterialViewDaily) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"material_view_daily_fingerprint_shape": "octet_length(viewer_fingerprint) = 32 AND viewer_fingerprint <> decode(repeat('00', 32), 'hex')",
			"material_view_daily_projection_time":   "projected_at IS NULL OR projected_at >= created_at",
		}),
	}
}
