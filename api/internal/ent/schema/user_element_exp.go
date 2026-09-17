package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// UserElementExp holds the schema definition for per-element EXP tracking.
type UserElementExp struct {
	ent.Schema
}

// Mixin of the UserElementExp.
func (UserElementExp) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the UserElementExp.
func (UserElementExp) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("element_id", uuid.UUID{}),
		field.Int64("exp").
			Default(0).
			NonNegative().
			Comment("Accumulated EXP for this element"),
	}
}

// Edges of the UserElementExp.
func (UserElementExp) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("user_element_exps").Field("user_id").Unique().Required(),
		edge.From("element", Element.Type).Ref("user_element_exps").Field("element_id").Unique().Required(),
	}
}

// Indexes of the UserElementExp.
func (UserElementExp) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "element_id").Unique(),
	}
}
