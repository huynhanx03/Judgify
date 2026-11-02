package schema

import (
	"entgo.io/ent"
	commonent "github.com/huynhanx03/go-common/pkg/security/authorization/casbin/ent"
)

// CasbinRule mirrors the persistence contract required by casbin/ent-adapter.
// Its integer key is adapter-internal and is not exposed as a Judgify domain ID.
type CasbinRule struct {
	ent.Schema
}

// Mixins keeps the persisted row shape identical across go-common consumers.
func (CasbinRule) Mixin() []ent.Mixin {
	return []ent.Mixin{commonent.Mixin{}}
}
