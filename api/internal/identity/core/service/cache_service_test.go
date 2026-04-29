package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
)

func TestGetPermissionConfigVersion_Default(t *testing.T) {
	c := ember.New[string, any]()
	svc := NewCacheService(c).(*cacheService)

	v, err := svc.GetPermissionConfigVersion(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), v)
}

func TestInvalidatePermissionConfig_Increments(t *testing.T) {
	c := ember.New[string, any]()
	svc := NewCacheService(c).(*cacheService)

	err := svc.InvalidatePermissionConfig(context.Background())
	assert.NoError(t, err)

	v, _ := svc.GetPermissionConfigVersion(context.Background())
	assert.Equal(t, int64(2), v)
}

func TestInvalidatePermissionConfig_Multiple(t *testing.T) {
	c := ember.New[string, any]()
	svc := NewCacheService(c).(*cacheService)

	for i := 0; i < 5; i++ {
		svc.InvalidatePermissionConfig(context.Background())
	}
	v, _ := svc.GetPermissionConfigVersion(context.Background())
	assert.Equal(t, int64(6), v)
}
