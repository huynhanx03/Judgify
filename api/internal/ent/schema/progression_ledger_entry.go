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

const maximumProgressionLedgerJSONBytes = 64 << 10

// ProgressionLedgerEntry is the immutable accounting authority for cultivation
// progression. Source identity is deliberately generic: solve rewards are the
// first producer, while later cultivation adjustments can use the same ledger
// without importing Submission, Judge, or transport concepts into the schema.
type ProgressionLedgerEntry struct{ ent.Schema }

func (ProgressionLedgerEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (ProgressionLedgerEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("reversal", ProgressionLedgerEntry.Type).
			Unique().
			From("reversed_entry").
			Unique().
			Field("reversed_entry_id").
			Immutable(),
		edge.From("reward_rule_revision", RewardRuleRevision.Type).
			Ref("ledger_entries").
			Field("reward_rule_revision_id").
			Unique().
			Immutable(),
		edge.From("reward_profile_revision", UserRewardProfileRevision.Type).
			Ref("ledger_entries").
			Field("reward_profile_revision_id").
			Unique().
			Immutable(),
	}
}

func (ProgressionLedgerEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.Enum("kind").Values(
			"grant",
			"reversal",
			"manual_adjustment",
			"manual_reversal",
			"legacy_reconstructed",
		).Immutable(),
		field.String("source_type").NotEmpty().MaxLen(64).Immutable(),
		field.UUID("source_aggregate_id", uuid.UUID{}).Immutable(),
		field.Int64("source_aggregate_version").Min(1).Immutable(),
		field.UUID("source_transition_id", uuid.UUID{}).Immutable(),
		field.UUID("source_event_id", uuid.UUID{}).Immutable(),
		field.UUID("reversed_entry_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("reward_rule_revision_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("reward_profile_revision_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int64("total_exp_delta").Immutable(),
		field.Int64("balance_before").NonNegative().Immutable(),
		field.Int64("balance_after").NonNegative().Immutable(),
		field.Int64("progression_version").Min(1).Immutable(),
		field.JSON("element_deltas", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).Immutable(),
		field.JSON("difficulty_deltas", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).Immutable(),
		field.JSON("tag_deltas", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).Immutable(),
		field.Bytes("source_checksum").Immutable(),
		field.JSON("provenance", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).Immutable(),
		field.String("correlation_id").NotEmpty().MaxLen(128).
			SchemaType(map[string]string{dialect.Postgres: "character varying(128)"}).Immutable(),
		field.Time("occurred_at").Immutable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (ProgressionLedgerEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_type", "source_aggregate_id", "source_aggregate_version").Unique(),
		index.Fields("source_transition_id").Unique(),
		index.Fields("source_event_id").Unique(),
		index.Fields("reversed_entry_id").Unique(),
		index.Fields("id", "source_aggregate_id", "user_id").Unique(),
		index.Fields(
			"id", "source_transition_id", "source_event_id", "source_aggregate_id",
			"user_id", "source_aggregate_version", "kind",
		).Unique(),
		index.Fields("user_id", "occurred_at", "id"),
		index.Fields("user_id", "progression_version").Unique(),
		index.Fields("source_type", "source_aggregate_id", "occurred_at", "id"),
	}
}

func (ProgressionLedgerEntry) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"progression_ledger_nonzero_identity": `id <> ` + problemNilUUIDSQL + `
				AND user_id <> ` + problemNilUUIDSQL + `
				AND source_aggregate_id <> ` + problemNilUUIDSQL + `
				AND source_transition_id <> ` + problemNilUUIDSQL + `
				AND source_event_id <> ` + problemNilUUIDSQL + `
				AND (reversed_entry_id IS NULL OR (
					reversed_entry_id <> ` + problemNilUUIDSQL + `
					AND reversed_entry_id <> id
				))`,
			"progression_ledger_kind_shape": `(
				kind IN ('grant', 'legacy_reconstructed')
				AND reversed_entry_id IS NULL
				AND total_exp_delta > 0
			) OR (
				kind = 'reversal'
				AND reversed_entry_id IS NOT NULL
				AND total_exp_delta < 0
			) OR (
				kind = 'manual_adjustment'
				AND reversed_entry_id IS NULL
				AND total_exp_delta <> 0
			) OR (
				kind = 'manual_reversal'
				AND reversed_entry_id IS NOT NULL
				AND total_exp_delta <> 0
			)`,
			"progression_ledger_rule_shape": `(
				kind IN ('grant', 'reversal')
				AND reward_rule_revision_id IS NOT NULL
				AND reward_profile_revision_id IS NOT NULL
			) OR (
				kind NOT IN ('grant', 'reversal')
				AND reward_rule_revision_id IS NULL
				AND reward_profile_revision_id IS NULL
			)`,
			"progression_ledger_balance": `balance_before <= 9007199254740991
				AND balance_after <= 9007199254740991
				AND balance_after - balance_before = total_exp_delta`,
			"progression_ledger_source_checksum": `octet_length(source_checksum) = 32
				AND source_checksum <> ` + problemZeroDigestSQL,
			"progression_ledger_json_shape": fmt.Sprintf(`
				jsonb_typeof(element_deltas) = 'array'
				AND jsonb_typeof(difficulty_deltas) = 'array'
				AND jsonb_typeof(tag_deltas) = 'array'
				AND jsonb_typeof(provenance) = 'object'
				AND octet_length(element_deltas::text) <= %d
				AND octet_length(difficulty_deltas::text) <= %d
				AND octet_length(tag_deltas::text) <= %d
				AND octet_length(provenance::text) <= %d`,
				maximumProgressionLedgerJSONBytes,
				maximumProgressionLedgerJSONBytes,
				maximumProgressionLedgerJSONBytes,
				maximumProgressionLedgerJSONBytes,
			),
		}),
	}
}
