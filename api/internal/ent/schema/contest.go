package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/judgify/pkg/database/ent"
	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Contest holds the schema definition for the Contest entity.
type Contest struct {
	ent.Schema
}

func (Contest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

func (Contest) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").NotEmpty().MaxLen(300),
		field.Text("description").Optional(),
		field.Time("start_time"),
		field.Time("end_time"),
		field.Enum("status").
			Values("draft", "upcoming", "running", "ended").
			Default("draft"),
		field.Int("author_id"),
		field.Int("max_participants").Default(0).Comment("0 = unlimited"),
	}
}

func (Contest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("start_time"),
	}
}

func (Contest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).Ref("contests").Field("author_id").Unique().Required(),
		edge.To("problems", Problem.Type),
		edge.To("registrations", ContestRegistration.Type),
		edge.To("standings", ContestStanding.Type),
		edge.To("submissions", Submission.Type),
		edge.To("rating_histories", RatingHistory.Type),
	}
}
