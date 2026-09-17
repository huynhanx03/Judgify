package schema

import (
	"encoding/json"
	"fmt"
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
)

// CultivationCatalogRevision is the immutable, exact trait pool used by an
// onboarding offer. The highest revision is the active launch catalog.
type CultivationCatalogRevision struct{ ent.Schema }

func (CultivationCatalogRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (CultivationCatalogRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.Int64("revision").Min(1).Immutable(),
		field.String("algorithm_version").NotEmpty().MaxLen(64).Immutable(),
		field.Bytes("catalog_checksum").NotEmpty().Immutable(),
		field.JSON("root_pool", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable().Sensitive(),
		field.JSON("talent_pool", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable().Sensitive(),
		field.Int("maximum_offers").Min(1).Max(10).Immutable(),
		field.Int64("offer_ttl_seconds").Min(60).Max(24 * 60 * 60).Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (CultivationCatalogRevision) Edges() []ent.Edge {
	return []ent.Edge{edge.To("offers", OnboardingTraitOffer.Type)}
}

func (CultivationCatalogRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("revision").Unique(),
		index.Fields("catalog_checksum").Unique(),
	}
}

func (CultivationCatalogRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"cultivation_catalog_checksum_shape":  "octet_length(catalog_checksum) = 32 AND catalog_checksum <> decode(repeat('00', 32), 'hex')",
		"cultivation_catalog_algorithm_bound": fmt.Sprintf("octet_length(algorithm_version) BETWEEN 1 AND %d", 64),
		"cultivation_catalog_pool_shape":      "jsonb_typeof(root_pool) = 'array' AND jsonb_array_length(root_pool) >= 1 AND jsonb_typeof(talent_pool) = 'array' AND jsonb_array_length(talent_pool) >= 6",
	})}
}
