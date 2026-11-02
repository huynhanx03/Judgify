package cache

import (
	"context"
	"sort"
	"strings"

	"github.com/huynhanx03/judgify/internal/tenantctx"
)

// tenantKey prevents a process-local tenant projection from being reused in a
// different tenant or in an All-memberships view. Sorting makes All-scope keys
// stable regardless of membership query order.
func TenantScopedKey(ctx context.Context, key string) string {
	scope, ok := tenantctx.FromContext(ctx)
	if !ok {
		return key + ":scope:none"
	}
	ids := scope.TenantIDs()
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = id.String()
	}
	sort.Strings(values)
	if scope.IsTenant() {
		return key + ":tenant:" + values[0]
	}
	return key + ":tenants:" + strings.Join(values, ",")
}

func tenantKey(ctx context.Context, key string) string { return TenantScopedKey(ctx, key) }
