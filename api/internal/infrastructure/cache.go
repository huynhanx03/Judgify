package infrastructure

import (
	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
)

// SetupCache initializes the local in-memory cache
func SetupCache() {
	global.Ember = ember.New[string, any](
		ember.WithMaxEntries(10000),
		ember.WithNumShards(256),
	)
}
