package mixin

import (
	"context"
	"fmt"

	"entgo.io/ent"

	e "github.com/huynhanx03/go-common/pkg/database/ent"

	gen "github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/intercept"
)

// SoftDeleteMixin embeds the base mixin from pkg/database/ent and overrides
// Hooks/Interceptors to use the generated client for proper delete-to-update conversion.
type SoftDeleteMixin struct {
	e.SoftDeleteMixin
}

// Hooks converts delete operations into soft-delete updates.
func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		e.SoftDeleteHook(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			mx, ok := m.(interface{ Client() *gen.Client })
			if !ok {
				return nil, fmt.Errorf("unexpected mutation type %T", m)
			}
			return mx.Client().Mutate(ctx, m)
		}),
	}
}

// Interceptors auto-applies WHERE deleted_at IS NULL to all queries.
func (d SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
			// Skip soft-delete, means include soft-deleted entities.
			if e.IsSkipSoftDelete(ctx) {
				return nil
			}
			d.P(q)
			return nil
		}),
	}
}
