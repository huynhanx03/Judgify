package testutil

import (
	"github.com/huynhanx03/go-common/pkg/common/cache"
	"github.com/huynhanx03/go-common/pkg/common/cache/ristretto"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
)

// NewCache returns a small process-local cache for tests, mirroring the
// production wiring in infrastructure.SetupCache. It panics on the (practically
// impossible) config error so callers stay one-liners.
func NewCache() *cache.Cache {
	c, err := ristretto.New[string, any](
		ristretto.WithMaxCost(10_000),
		ristretto.WithNumCounters(100_000),
	)
	if err != nil {
		panic(err)
	}
	return NewCacheWithL1(c)
}

// NewCacheWithL1 creates a coordinator around a test L1 engine.
func NewCacheWithL1(l1 cache.L1Engine) *cache.Cache {
	shared, err := cache.New(cache.Config{L1: l1, Logger: logger.Wrap(zap.NewNop())})
	if err != nil {
		l1.Close()
		panic(err)
	}
	return shared
}
