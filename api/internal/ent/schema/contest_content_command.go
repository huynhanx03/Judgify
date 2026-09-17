package schema

import (
	"fmt"

	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

const maximumContestContentResultProjectionBytes = 1 << 20

// ContestContentCommand is the durable exact-replay fence for side-effecting
// content commands. The request is retained only as a digest. Its bounded
// result projection is access-controlled with the owning aggregate and exists
// solely to return the original response after a lost network acknowledgement.
type ContestContentCommand struct{ ent.Schema }

func (ContestContentCommand) Mixin() []ent.Mixin { return []ent.Mixin{commonent.UUIDMixin{}} }
func (ContestContentCommand) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.Enum("command_kind").Values("announcement_create", "announcement_edit", "announcement_publish", "announcement_withdraw", "clarification_submit", "clarification_answer", "clarification_close", "clarification_withdraw").Immutable(),
		field.UUID("idempotency_key", uuid.UUID{}).Immutable(),
		field.Bytes("request_hash").NotEmpty().Immutable(),
		field.UUID("actor_user_id", uuid.UUID{}).Immutable(),
		field.UUID("actor_session_id", uuid.UUID{}).Immutable(),
		field.Enum("result_owner_type").Values("announcement", "clarification").Optional().Nillable(),
		field.UUID("result_content_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("result_revision_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("result_version").Optional().Nillable(),
		field.Bytes("result_projection").Optional().Nillable(),
		field.Time("created_at").Immutable().Annotations(entsql.Default("CURRENT_TIMESTAMP")),
	}
}
func (ContestContentCommand) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("content_commands").Field("contest_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("actor", User.Type).Ref("contest_content_commands").Field("actor_user_id").Unique().Required().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}
func (ContestContentCommand) Indexes() []ent.Index {
	return []ent.Index{index.Fields("command_kind", "idempotency_key").Unique(), index.Fields("contest_id", "created_at")}
}
func (ContestContentCommand) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Checks(map[string]string{
		"contest_content_command_identity": "id <> " + contentRenderNilUUIDSQL + " AND contest_id <> " + contentRenderNilUUIDSQL + " AND idempotency_key <> " + contentRenderNilUUIDSQL + " AND actor_user_id <> " + contentRenderNilUUIDSQL + " AND actor_session_id <> " + contentRenderNilUUIDSQL,
		"contest_content_command_hash":     "octet_length(request_hash) = 32 AND request_hash <> " + contentRenderZeroDigestSQL,
		"contest_content_command_result":   `(result_owner_type IS NULL AND result_content_id IS NULL AND result_revision_id IS NULL AND result_version IS NULL AND result_projection IS NULL) OR (result_owner_type IS NOT NULL AND result_content_id IS NOT NULL AND result_content_id <> ` + contentRenderNilUUIDSQL + ` AND result_version IS NOT NULL AND result_version >= 1 AND result_projection IS NOT NULL AND octet_length(result_projection) BETWEEN 2 AND ` + fmt.Sprint(maximumContestContentResultProjectionBytes) + `)`,
	})}
}
