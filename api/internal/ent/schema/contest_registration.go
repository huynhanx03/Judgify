package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ContestRegistration holds the schema definition for the ContestRegistration entity.
type ContestRegistration struct {
	ent.Schema
}

func (ContestRegistration) Fields() []ent.Field {
	return []ent.Field{
		field.Int("contest_id"),
		field.Int("user_id"),
	}
}

func (ContestRegistration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("contest_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}

func (ContestRegistration) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("contest", Contest.Type).Ref("registrations").Field("contest_id").Unique().Required(),
		edge.From("user", User.Type).Ref("contest_registrations").Field("user_id").Unique().Required(),
	}
}
