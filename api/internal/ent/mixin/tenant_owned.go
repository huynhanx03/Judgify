package mixin

import (
	"github.com/google/uuid"

	"entgo.io/ent"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TenantOwnedMixin adds the required tenant owner key to tenant data.
// Database migration constraints and the Ent tenant interceptor enforce its
// value and scope at runtime.
type TenantOwnedMixin struct{}

func (TenantOwnedMixin) Fields() []ent.Field {
	return []ent.Field{field.UUID("tenant_id", uuid.UUID{}).Immutable()}
}

func (TenantOwnedMixin) Annotations() []entschema.Annotation { return nil }
func (TenantOwnedMixin) Edges() []ent.Edge                   { return nil }
func (TenantOwnedMixin) Indexes() []ent.Index                { return []ent.Index{index.Fields("tenant_id")} }
func (TenantOwnedMixin) Hooks() []ent.Hook                   { return nil }
func (TenantOwnedMixin) Interceptors() []ent.Interceptor     { return nil }
func (TenantOwnedMixin) Policy() ent.Policy                  { return nil }
