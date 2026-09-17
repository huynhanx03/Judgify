package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CasbinRule mirrors the persistence contract required by casbin/ent-adapter.
// Its integer key is adapter-internal and is not exposed as a Judgify domain ID.
type CasbinRule struct {
	ent.Schema
}

// Fields keeps the column names and defaults compatible with ent-adapter.
func (CasbinRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("ptype").Default(""),
		field.String("v0").Default(""),
		field.String("v1").Default(""),
		field.String("v2").Default(""),
		field.String("v3").Default(""),
		field.String("v4").Default(""),
		field.String("v5").Default(""),
	}
}

// Indexes prevents duplicate policy rows, including under concurrent writes.
func (CasbinRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ptype", "v0", "v1", "v2", "v3", "v4", "v5").Unique(),
	}
}
