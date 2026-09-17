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

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

// DifficultyRevision stores one immutable, server-owned difficulty snapshot.
type DifficultyRevision struct{ ent.Schema }

func (DifficultyRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.AppendOnlyMixin{}}
}

func (DifficultyRevision) Fields() []ent.Field {
	return append(problemAppendOnlyRecordFields(),
		field.UUID("difficulty_id", uuid.UUID{}).Immutable(),
		field.Int64("revision").
			Min(1).
			Max(problemcontract.MaximumDifficultyRevision).
			Immutable(),
		field.String("name").
			NotEmpty().
			MaxLen(50).
			Immutable(),
		field.Int("level").
			Positive().
			Immutable(),
		field.Int64("exp_reward").
			Min(0).
			Max(problemcontract.MaximumBaseExperience).
			Immutable(),
		field.String("description").
			Optional().
			MaxLen(255).
			Immutable(),
		field.Bytes("checksum").
			NotEmpty().
			Immutable(),
	)
}

func (DifficultyRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("difficulty", Difficulty.Type).
			Ref("revisions").
			Field("difficulty_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}

func (DifficultyRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("difficulty_id", "revision").Unique(),
		index.Fields("difficulty_id", "created_at", "id"),
	}
}

func (DifficultyRevision) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"difficulty_revision_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND difficulty_id <> " + problemNilUUIDSQL,
			"difficulty_revision_checksum_shape": "octet_length(checksum) = 32 AND checksum <> " + problemZeroDigestSQL,
			"difficulty_revision_number_bound": fmt.Sprintf(
				"revision BETWEEN 1 AND %d",
				problemcontract.MaximumDifficultyRevision,
			),
			"difficulty_revision_exp_reward_bound": fmt.Sprintf(
				"exp_reward BETWEEN 0 AND %d",
				problemcontract.MaximumBaseExperience,
			),
		}),
	}
}
