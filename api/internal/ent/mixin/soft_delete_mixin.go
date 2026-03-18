package mixin

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"

	gen "github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/intercept"
	e "github.com/huynhanx03/judgify/pkg/database/ent"
)

// SoftDeleteMixin embeds the base mixin from pkg/database/ent and overrides
// Hooks/Interceptors to use the generated client for proper delete-to-update conversion.
type SoftDeleteMixin struct {
	e.SoftDeleteMixin
}

// Hooks converts delete operations into soft-delete updates.
func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		e.On(
			func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					// Skip soft-delete, means delete the entity permanently.
					if e.IsSkipSoftDelete(ctx) {
						return next.Mutate(ctx, m)
					}
					mx, ok := m.(interface {
						SetOp(ent.Op)
						Client() *gen.Client
						SetDeletedAt(time.Time)
						WhereP(...func(*sql.Selector))
					})
					if !ok {
						return nil, fmt.Errorf("unexpected mutation type %T", m)
					}
					d.P(mx)
					mx.SetOp(ent.OpUpdate)
					mx.SetDeletedAt(time.Now().UTC())
					return mx.Client().Mutate(ctx, m)
				})
			},
			ent.OpDeleteOne|ent.OpDelete,
		),
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
