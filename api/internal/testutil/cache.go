package testutil

import (
	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/go-common/pkg/common/cache/ristretto"
)

// NewCache returns a small process-local cache for tests, mirroring the
// production wiring in infrastructure.SetupCache. It panics on the (practically
// impossible) config error so callers stay one-liners.
func NewCache() cache.LocalCache[string, any] {
	c, err := ristretto.New[string, any](
		ristretto.WithMaxCost(10_000),
		ristretto.WithNumCounters(100_000),
	)
	if err != nil {
		panic(err)
	}
	return c
}
