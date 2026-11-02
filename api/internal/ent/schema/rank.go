package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// Rank holds the schema definition for rating-based titles.
type Rank struct {
	ent.Schema
}

// Mixin of the Rank.
func (Rank) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the Rank.
func (Rank) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(100).
			Comment("Title: Tan Tu, Ngoai Mon De Tu, Noi Mon De Tu..."),
		field.Int("min_rating").
			Default(0).
			Comment("Minimum rating to earn this title"),
		field.String("description").
			Optional().
			MaxLen(500),
	}
}

func (Rank) Indexes() []ent.Index {
	return []ent.Index{index.Fields("name").Unique()}
}

// Edges of the Rank.
func (Rank) Edges() []ent.Edge {
	return nil
}
