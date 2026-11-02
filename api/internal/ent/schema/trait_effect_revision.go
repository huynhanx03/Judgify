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

const maximumTraitEffectDocumentBytes = 8 << 10

// TraitEffectRevision is the immutable reward meaning of one trait revision.
// The typed JSON is paired with resolved element UUIDs so replay never asks a
// mutable element catalog what a stable code used to mean.
type TraitEffectRevision struct{ ent.Schema }

func (TraitEffectRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (TraitEffectRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("trait_id", uuid.UUID{}).Immutable(),
		field.Int64("revision").Min(1).Immutable(),
		field.Int("schema_version").Min(1).Immutable(),
		field.JSON("effect", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable(),
		field.JSON("target_element_ids", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable(),
		field.Bytes("checksum").NotEmpty().Immutable(),
		field.Time("effective_at").Immutable(),
		field.Time("created_at").
			Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (TraitEffectRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("trait", Trait.Type).
			Ref("effect_revisions").Field("trait_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("profile_traits", UserRewardProfileTrait.Type),
	}
}

func (TraitEffectRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("trait_id", "revision").Unique(),
		index.Fields("trait_id", "effective_at", "revision", "id").Unique(),
		index.Fields("checksum").Unique(),
	}
}

func (TraitEffectRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"trait_effect_revision_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
			AND trait_id <> ` + problemNilUUIDSQL,
		"trait_effect_revision_checksum_shape": `octet_length(checksum) = 32
			AND checksum <> ` + problemZeroDigestSQL,
		"trait_effect_revision_document_shape": fmt.Sprintf(`schema_version = 1
			AND jsonb_typeof(effect) = 'object'
			AND jsonb_typeof(target_element_ids) = 'array'
			AND octet_length(effect::text) <= %d
			AND octet_length(target_element_ids::text) <= %d`,
			maximumTraitEffectDocumentBytes,
			maximumTraitEffectDocumentBytes,
		),
	})}
}
