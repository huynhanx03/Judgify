package cache_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
)

const benchKeys = 10000

// -- Get Benchmarks --

func BenchmarkCompare_Get_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value")
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(fmt.Sprintf("key%d", i%benchKeys))
			i++
		}
	})
}

// -- Get Single Key (Hot Key) --

func BenchmarkCompare_GetHit_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	c.Set("hotkey", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("hotkey")
	}
}

// -- Set Benchmarks --

func BenchmarkCompare_Set_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(fmt.Sprintf("key%d", i%benchKeys), "value")
			i++
		}
	})
}

// -- Mixed 90% Read / 10% Write --

func BenchmarkCompare_Mixed90R10W_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value")
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%benchKeys)
			if i%10 == 0 {
				c.Set(key, "value")
			} else {
				c.Get(key)
			}
			i++
		}
	})
}

// -- Concurrent Mixed (8 goroutines, 70R/20W/10D) --

func BenchmarkCompare_ConcurrentMixed_Ember(b *testing.B) {
	c := ember.New[string, int](ember.WithMaxEntries(50000))
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
	var wg sync.WaitGroup
	b.ResetTimer()
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g)))
			for i := 0; i < b.N/8; i++ {
				key := fmt.Sprintf("key%d", rng.Intn(20000))
				switch rng.Intn(10) {
				case 0, 1:
					c.Set(key, i)
				case 2:
					c.Delete(key)
				default:
					c.Get(key)
				}
			}
		}(g)
	}
	wg.Wait()
}
