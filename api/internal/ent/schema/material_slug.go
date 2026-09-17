package schema

import (
	"fmt"
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

const materialMaximumSlugBytes = 160

// MaterialSlug retains every canonical slug permanently. A retired slug
// resolves through its material's one current pointer; no redirect chain exists.
type MaterialSlug struct{ ent.Schema }

func (MaterialSlug) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("material_id", uuid.UUID{}).Immutable(),
		field.String("slug").NotEmpty().MaxLen(materialMaximumSlugBytes).Immutable(),
		field.Enum("state").Values("current", "retired").Default("current"),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
		field.Time("retired_at").Optional().Nillable(),
	}
}

func (MaterialSlug) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("material", Material.Type).
			Ref("slugs").
			Field("material_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (MaterialSlug) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").Unique(),
		index.Fields("id", "material_id").Unique(),
		index.Fields("id", "material_id", "state").Unique(),
		index.Fields("material_id", "state"),
		index.Fields("material_id").Unique().Annotations(entsql.IndexWhere("state = 'current'")),
	}
}

func (MaterialSlug) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"material_slug_state_shape": "(state = 'current' AND retired_at IS NULL) OR (state = 'retired' AND retired_at IS NOT NULL)",
			"material_slug_normalized": fmt.Sprintf(
				"octet_length(slug) BETWEEN 1 AND %d AND slug = lower(slug) AND slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'",
				materialMaximumSlugBytes,
			),
		}),
	}
}
