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

const maximumRewardRuleConfigurationBytes = 16 << 10

// RewardRuleRevision is an immutable, effective-dated formula snapshot. A
// ledger grant points at the exact row used for calculation, so a deployment
// can introduce a new formula without reinterpreting historical balances.
type RewardRuleRevision struct{ ent.Schema }

func (RewardRuleRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (RewardRuleRevision) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(commonent.NewUUID).Immutable(),
		field.String("rule_key").NotEmpty().MaxLen(64).Immutable(),
		field.Int64("revision").Min(1).Immutable(),
		field.String("algorithm_version").NotEmpty().MaxLen(64).Immutable(),
		field.JSON("configuration", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb", dialect.SQLite: "json"}).
			Immutable(),
		field.Bytes("checksum").Immutable(),
		field.Bytes("request_checksum").Immutable(),
		field.Time("effective_from").Immutable(),
		field.Time("created_at").Default(func() time.Time { return time.Now().UTC() }).
			Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}

func (RewardRuleRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("ledger_entries", ProgressionLedgerEntry.Type),
	}
}

func (RewardRuleRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("rule_key", "revision").Unique(),
		index.Fields("rule_key", "effective_from"),
		index.Fields("checksum").Unique(),
		index.Fields("request_checksum").Unique(),
	}
}

func (RewardRuleRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"reward_rule_revision_identity": `id <> ` + problemNilUUIDSQL,
			"reward_rule_revision_checksum": `octet_length(checksum) = 32
				AND checksum <> ` + problemZeroDigestSQL + `
				AND octet_length(request_checksum) = 32
				AND request_checksum <> ` + problemZeroDigestSQL,
			"reward_rule_revision_configuration": fmt.Sprintf(`jsonb_typeof(configuration) = 'object'
				AND octet_length(configuration::text) <= %d`, maximumRewardRuleConfigurationBytes),
		}),
	}
}
