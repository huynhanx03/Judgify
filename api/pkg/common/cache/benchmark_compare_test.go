package cache_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
	"github.com/huynhanx03/judgify/pkg/common/cache/tinylfu"
)

const benchKeys = 10000

// -- Get Benchmarks --

func BenchmarkCompare_Get_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value", 1)
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

func BenchmarkCompare_Get_TinyLFU(b *testing.B) {
	c := tinylfu.New[string, string](tinylfu.Config{MaxCost: 100000})
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value", 1)
	}
	// TinyLFU uses async set, wait for items to be processed
	time.Sleep(100 * time.Millisecond)
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
	c.Set("hotkey", "value", 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("hotkey")
	}
}

func BenchmarkCompare_GetHit_TinyLFU(b *testing.B) {
	c := tinylfu.New[string, string](tinylfu.Config{MaxCost: 100000})
	defer c.Close()
	c.Set("hotkey", "value", 1)
	time.Sleep(50 * time.Millisecond)
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
			c.Set(fmt.Sprintf("key%d", i%benchKeys), "value", 1)
			i++
		}
	})
}

func BenchmarkCompare_Set_TinyLFU(b *testing.B) {
	c := tinylfu.New[string, string](tinylfu.Config{MaxCost: 100000})
	defer c.Close()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(fmt.Sprintf("key%d", i%benchKeys), "value", 1)
			i++
		}
	})
}

// -- SetWithTTL --

func BenchmarkCompare_SetTTL_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	ttl := 5 * time.Minute
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.SetWithTTL(fmt.Sprintf("key%d", i%benchKeys), "value", 1, ttl)
			i++
		}
	})
}

func BenchmarkCompare_SetTTL_TinyLFU(b *testing.B) {
	c := tinylfu.New[string, string](tinylfu.Config{MaxCost: 100000})
	defer c.Close()
	ttl := 5 * time.Minute
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.SetWithTTL(fmt.Sprintf("key%d", i%benchKeys), "value", 1, ttl)
			i++
		}
	})
}

// -- Mixed 90% Read / 10% Write --

func BenchmarkCompare_Mixed90R10W_Ember(b *testing.B) {
	c := ember.New[string, string](ember.WithMaxEntries(100000))
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value", 1)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%benchKeys)
			if i%10 == 0 {
				c.Set(key, "value", 1)
			} else {
				c.Get(key)
			}
			i++
		}
	})
}

func BenchmarkCompare_Mixed90R10W_TinyLFU(b *testing.B) {
	c := tinylfu.New[string, string](tinylfu.Config{MaxCost: 100000})
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value", 1)
	}
	time.Sleep(100 * time.Millisecond)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%benchKeys)
			if i%10 == 0 {
				c.Set(key, "value", 1)
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
		c.Set(fmt.Sprintf("key%d", i), i, 1)
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
					c.Set(key, i, 1)
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

func BenchmarkCompare_ConcurrentMixed_TinyLFU(b *testing.B) {
	c := tinylfu.New[string, int](tinylfu.Config{MaxCost: 50000})
	defer c.Close()
	for i := 0; i < benchKeys; i++ {
		c.Set(fmt.Sprintf("key%d", i), i, 1)
	}
	time.Sleep(100 * time.Millisecond)
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
					c.Set(key, i, 1)
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
