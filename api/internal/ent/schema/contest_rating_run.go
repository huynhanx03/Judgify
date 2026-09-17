package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

// ContestRatingRun is an immutable decision attempt for one contest sequence.
// Retrying an applied run returns the same row; historical changes create a
// new run_version and are selected through a new run chain.
type ContestRatingRun struct{ ent.Schema }

func (ContestRatingRun) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.UUIDMixin{}, commonent.TimeMixin{}}
}

func (ContestRatingRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("contest_id", uuid.UUID{}).Immutable(),
		field.UUID("rating_sequence_slot_id", uuid.UUID{}).Immutable(),
		field.Int64("rating_sequence").Min(1).Immutable(),
		field.Int64("run_version").Min(1).Immutable(),
		field.Int64("standings_projection_version").Min(1).Immutable(),
		field.String("algorithm_version").NotEmpty().MaxLen(64).Immutable(),
		field.Enum("status").
			Values("waiting", "running", "applied", "skipped", "superseded", "failed").
			Default("waiting"),
		field.String("skip_code").MaxLen(64).Optional().Nillable().Immutable(),
		field.UUID("actor_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(1024).Optional().Nillable().Immutable(),
		field.Bytes("input_checksum").NotEmpty().MaxLen(32).Immutable(),
		field.Int("participant_count").NonNegative().Immutable(),
		field.String("failure_code").MaxLen(64).Optional().Nillable(),
		field.Time("applied_at").Optional().Nillable(),
	}
}

func (ContestRatingRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).
			Ref("rating_runs").
			Field("contest_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.From("sequence_slot", RatingSequenceSlot.Type).
			Ref("runs").
			Field("rating_sequence_slot_id").
			Unique().
			Required().
			Immutable().
			Annotations(entsql.OnDelete(entsql.NoAction)),
		edge.To("results", ContestRatingResult.Type),
		edge.To("chain_entries", RatingRunChainEntry.Type),
		edge.To("compatibility_histories", RatingHistory.Type),
	}
}

func (ContestRatingRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "run_version").Unique(),
		index.Fields("rating_sequence", "run_version").Unique(),
		index.Fields("rating_sequence_slot_id", "run_version").Unique(),
		index.Fields("status", "created_at", "id"),
		index.Fields("contest_id", "status", "run_version"),
	}
}

func (ContestRatingRun) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"contest_rating_run_nonzero_contest": "contest_id <> '00000000-0000-0000-0000-000000000000'::uuid AND rating_sequence_slot_id <> '00000000-0000-0000-0000-000000000000'::uuid",
			"contest_rating_run_checksum":        "octet_length(input_checksum) = 32",
			"contest_rating_run_skip_shape":      "(status = 'skipped' AND skip_code IS NOT NULL AND reason IS NOT NULL AND applied_at IS NOT NULL) OR (status <> 'skipped' AND skip_code IS NULL)",
			"contest_rating_run_applied_shape":   "(status IN ('applied', 'skipped') AND applied_at IS NOT NULL AND failure_code IS NULL) OR (status NOT IN ('applied', 'skipped') AND applied_at IS NULL)",
			"contest_rating_run_failed_shape":    "(status = 'failed' AND failure_code IS NOT NULL) OR (status <> 'failed' AND failure_code IS NULL)",
		}),
	}
}
