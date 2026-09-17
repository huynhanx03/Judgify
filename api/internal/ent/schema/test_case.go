package schema

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/mixin"
)

// TestCase holds the schema definition for the TestCase entity.
type TestCase struct {
	ent.Schema
}

// Mixin of the TestCase.
func (TestCase) Mixin() []ent.Mixin {
	return []ent.Mixin{
		e.UUIDMixin{},
		e.TimeMixin{},
		mixin.SoftDeleteMixin{},
	}
}

// Fields of the TestCase.
func (TestCase) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("problem_id", uuid.UUID{}).Immutable(),
	}
}

// Edges of the TestCase.
func (TestCase) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("problem", Problem.Type).
			Ref("test_cases").
			Field("problem_id").
			Unique().
			Required().
			Immutable(),
		edge.To("revisions", TestcaseRevision.Type),
	}
}

func (TestCase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id", "problem_id").Unique(),
	}
}

func (TestCase) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"test_case_nonzero_identity": "id <> " + problemNilUUIDSQL +
				" AND problem_id <> " + problemNilUUIDSQL,
		}),
	}
}
