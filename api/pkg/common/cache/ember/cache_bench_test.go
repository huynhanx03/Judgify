package ember

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func BenchmarkGet(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(fmt.Sprintf("key%d", i%10000))
			i++
		}
	})
}

func BenchmarkSet(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(fmt.Sprintf("key%d", i%10000), "value")
			i++
		}
	})
}

func BenchmarkSetWithTTL(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	ttl := 5 * time.Minute
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.SetWithTTL(fmt.Sprintf("key%d", i%10000), "value", ttl)
			i++
		}
	})
}

func BenchmarkGetHit(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	c.Set("hotkey", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("hotkey")
	}
}

func BenchmarkGetMiss(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("nonexistent")
	}
}

func BenchmarkMixed_50Read50Write(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	// Pre-populate half
	for i := 0; i < 5000; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%10000)
			if i%2 == 0 {
				c.Get(key)
			} else {
				c.Set(key, "value")
			}
			i++
		}
	})
}

func BenchmarkMixed_90Read10Write(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	for i := 0; i < 10000; i++ {
		c.Set(fmt.Sprintf("key%d", i), "value")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%10000)
			if i%10 == 0 {
				c.Set(key, "value")
			} else {
				c.Get(key)
			}
			i++
		}
	})
}

func BenchmarkEvictionLRU(b *testing.B) {
	c := New[string, int](WithMaxEntries(1000), WithEvictPolicy(EvictLRU))
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkEvictionLFU(b *testing.B) {
	c := New[string, int](WithMaxEntries(1000), WithEvictPolicy(EvictLFU))
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkEvictionRandom(b *testing.B) {
	c := New[string, int](WithMaxEntries(1000), WithEvictPolicy(EvictRandom))
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkIncr(b *testing.B) {
	c := New[string, any](WithMaxEntries(100000))
	defer c.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Incr(fmt.Sprintf("counter%d", i%1000))
	}
}

func BenchmarkGetOrSet(b *testing.B) {
	c := New[string, string](WithMaxEntries(100000))
	defer c.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%1000)
			c.GetOrSet(key, func() (string, time.Duration) {
				return "computed", 0
			})
			i++
		}
	})
}

func BenchmarkConcurrentMixed(b *testing.B) {
	c := New[string, int](WithMaxEntries(50000))
	defer c.Close()

	// Pre-populate
	for i := 0; i < 10000; i++ {
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
				case 0, 1: // 20% write
					c.Set(key, i)
				case 2: // 10% delete
					c.Delete(key)
				default: // 70% read
					c.Get(key)
				}
			}
		}(g)
	}
	wg.Wait()
}
