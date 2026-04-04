package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserTagStats tracks how many problems a user solved per tag.
type UserTagStats struct {
	ent.Schema
}

func (UserTagStats) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("tag_id"),
		field.Int("solved_count").Default(0).NonNegative(),
	}
}

func (UserTagStats) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("tag_stats").Field("user_id").Unique().Required(),
		edge.From("tag", Tag.Type).Ref("user_tag_stats").Field("tag_id").Unique().Required(),
	}
}

func (UserTagStats) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "tag_id").Unique(),
		index.Fields("user_id"),
	}
}
