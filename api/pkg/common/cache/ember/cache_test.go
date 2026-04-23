package ember

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// -- Core Get/Set/Delete --

func TestGetSet(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.Set("hello", "world")
	val, ok := c.Get("hello")
	if !ok || val != "world" {
		t.Fatalf("expected 'world', got '%s', ok=%v", val, ok)
	}
}

func TestGetMiss(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected miss for nonexistent key")
	}
}

func TestDelete(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	c.Set("key", 42)
	c.Delete("key")

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestClear(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	for i := 0; i < 50; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
	c.Clear()

	if c.DBSize() != 0 {
		t.Fatalf("expected 0 keys after clear, got %d", c.DBSize())
	}
}

// -- TTL / Expiry --

func TestSetWithTTL(t *testing.T) {
	c := New[string, string](WithMaxEntries(100), WithCleanupInterval(10*time.Millisecond))
	defer c.Close()

	c.SetWithTTL("ttlkey", "value", 50*time.Millisecond)

	val, ok := c.Get("ttlkey")
	if !ok || val != "value" {
		t.Fatal("expected key to exist before TTL")
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("ttlkey")
	if ok {
		t.Fatal("expected key to expire after TTL")
	}
}

func TestTTLCommand(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.SetWithTTL("k", "v", 1*time.Second)

	ttl, err := c.TTL("k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl <= 0 || ttl > 1*time.Second {
		t.Fatalf("unexpected TTL: %v", ttl)
	}

	// Key without TTL
	c.Set("persistent", "val")
	ttl, err = c.TTL("persistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl != -1 {
		t.Fatalf("expected -1 for persistent key, got %v", ttl)
	}

	// Nonexistent key
	_, err = c.TTL("nope")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestExpire(t *testing.T) {
	c := New[string, string](WithMaxEntries(100), WithCleanupInterval(10*time.Millisecond))
	defer c.Close()

	c.Set("k", "v")
	ok, err := c.Expire("k", 50*time.Millisecond)
	if err != nil || !ok {
		t.Fatal("Expire failed")
	}

	time.Sleep(100 * time.Millisecond)

	_, found := c.Get("k")
	if found {
		t.Fatal("key should have expired")
	}
}

func TestPersist(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.SetWithTTL("k", "v", 1*time.Second)
	ok, err := c.Persist("k")
	if err != nil || !ok {
		t.Fatal("Persist failed")
	}

	ttl, _ := c.TTL("k")
	if ttl != -1 {
		t.Fatalf("expected -1 after Persist, got %v", ttl)
	}
}

// -- Eviction Policies --

func TestEvictLRU(t *testing.T) {
	c := New[string, int](WithMaxEntries(5), WithEvictPolicy(EvictLRU))
	defer c.Close()

	for i := 0; i < 5; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	// Access key0 to make it recently used
	c.Get("key0")

	// Add more items to trigger eviction
	for i := 5; i < 10; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	if c.DBSize() > 5 {
		t.Fatalf("expected max 5 keys, got %d", c.DBSize())
	}

	stats := c.Stats()
	if stats.Evictions == 0 {
		t.Fatal("expected evictions to occur")
	}
}

func TestEvictLFU(t *testing.T) {
	c := New[string, int](WithMaxEntries(5), WithEvictPolicy(EvictLFU))
	defer c.Close()

	for i := 0; i < 5; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	// Access key0 many times to increase its LFU counter
	for j := 0; j < 20; j++ {
		c.Get("key0")
	}

	// Add more items
	for i := 5; i < 10; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	if c.DBSize() > 5 {
		t.Fatalf("expected max 5 keys, got %d", c.DBSize())
	}

	stats := c.Stats()
	if stats.Evictions == 0 {
		t.Fatal("expected evictions to occur")
	}
}

func TestEvictRandom(t *testing.T) {
	c := New[string, int](WithMaxEntries(5), WithEvictPolicy(EvictRandom))
	defer c.Close()

	for i := 0; i < 10; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	if c.DBSize() > 5 {
		t.Fatalf("expected max 5 keys, got %d", c.DBSize())
	}
}

func TestEvictNoEviction(t *testing.T) {
	c := New[string, int](WithMaxEntries(3), WithEvictPolicy(EvictNoEviction))
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	// Cache is full, next set should fail
	ok := c.Set("d", 4)
	if ok {
		t.Fatal("expected Set to return false when noeviction and full")
	}

	if c.DBSize() != 3 {
		t.Fatalf("expected 3 keys, got %d", c.DBSize())
	}
}

func TestOnEvictCallback(t *testing.T) {
	evicted := make(map[string]any)
	var mu sync.Mutex

	c := New[string, int](
		WithMaxEntries(3),
		WithEvictPolicy(EvictRandom),
		WithOnEvict(func(key string, value any) {
			mu.Lock()
			evicted[key] = value
			mu.Unlock()
		}),
	)
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4) // triggers eviction

	mu.Lock()
	if len(evicted) == 0 {
		t.Fatal("expected OnEvict callback to fire")
	}
	mu.Unlock()
}

// -- Stats --

func TestStats(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.Set("a", "1")
	c.Get("a")        // hit
	c.Get("a")        // hit
	c.Get("notexist") // miss

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.KeyCount != 1 {
		t.Fatalf("expected 1 key, got %d", stats.KeyCount)
	}
}

// -- Extended Operations --

func TestSetNX(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	ok := c.SetNX("key", "first", 0)
	if !ok {
		t.Fatal("expected SetNX to succeed on new key")
	}

	ok = c.SetNX("key", "second", 0)
	if ok {
		t.Fatal("expected SetNX to fail on existing key")
	}

	val, _ := c.Get("key")
	if val != "first" {
		t.Fatalf("expected 'first', got '%s'", val)
	}
}

func TestGetOrSet(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	// First call: computes value
	val, existed := c.GetOrSet("key", func() (int, time.Duration) {
		return 42, 0
	})
	if existed || val != 42 {
		t.Fatalf("expected new value 42, got %d, existed=%v", val, existed)
	}

	// Second call: returns cached value
	val, existed = c.GetOrSet("key", func() (int, time.Duration) {
		return 99, 0 // should not be used
	})
	if !existed || val != 42 {
		t.Fatalf("expected cached 42, got %d, existed=%v", val, existed)
	}
}

func TestGetDel(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.Set("key", "value")

	val, ok := c.GetDel("key")
	if !ok || val != "value" {
		t.Fatal("expected GetDel to return value")
	}

	_, ok = c.Get("key")
	if ok {
		t.Fatal("expected key to be deleted after GetDel")
	}
}

func TestMGetMSet(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	keys := []string{"a", "b", "c"}
	values := []int{1, 2, 3}
	c.MSet(keys, values)

	results := c.MGet("a", "b", "c", "d")
	if results[0] != 1 || results[1] != 2 || results[2] != 3 {
		t.Fatalf("unexpected MGet results: %v", results)
	}
	if results[3] != 0 { // zero value for missing key
		t.Fatalf("expected zero for missing key, got %d", results[3])
	}
}

// -- String Operations --

func TestIncrDecr(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	val, err := c.Incr("counter")
	if err != nil || val != 1 {
		t.Fatalf("expected 1, got %d, err=%v", val, err)
	}

	val, err = c.IncrBy("counter", 5)
	if err != nil || val != 6 {
		t.Fatalf("expected 6, got %d, err=%v", val, err)
	}

	val, err = c.Decr("counter")
	if err != nil || val != 5 {
		t.Fatalf("expected 5, got %d, err=%v", val, err)
	}
}

// -- Exists / Type / Keys / DBSize --

func TestExistsAndType(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	c.Set("str", "hello")

	if !c.Exists("str") {
		t.Fatal("expected str to exist")
	}
	if c.Exists("nope") {
		t.Fatal("expected nope to not exist")
	}

	typ, _ := c.Type("str")
	if typ != "string" {
		t.Fatalf("expected 'string', got '%s'", typ)
	}
}

func TestKeysAndDBSize(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	keys := c.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	if c.DBSize() != 3 {
		t.Fatalf("expected 3, got %d", c.DBSize())
	}
}

// -- Concurrency Safety --

func TestConcurrentGetSet(t *testing.T) {
	c := New[string, int](WithMaxEntries(10000))
	defer c.Close()

	var wg sync.WaitGroup
	n := 100

	// Concurrent writers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key%d", j)
				c.Set(key, i*100+j)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key%d", j)
				c.Get(key)
			}
		}()
	}

	wg.Wait()

	stats := c.Stats()
	if stats.KeyCount <= 0 {
		t.Fatal("expected keys to exist after concurrent ops")
	}
}

func TestConcurrentEviction(t *testing.T) {
	c := New[string, int](WithMaxEntries(50), WithEvictPolicy(EvictLRU))
	defer c.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("g%d_k%d", i, j)
				c.Set(key, j)
			}
		}(i)
	}
	wg.Wait()

	if c.DBSize() > 50 {
		t.Fatalf("expected max 50 keys, got %d", c.DBSize())
	}
}

// -- Active Expiry --

func TestActiveExpiry(t *testing.T) {
	c := New[string, string](
		WithMaxEntries(1000),
		WithCleanupInterval(10*time.Millisecond),
	)
	defer c.Close()

	for i := 0; i < 100; i++ {
		c.SetWithTTL(fmt.Sprintf("key%d", i), "val", 30*time.Millisecond)
	}

	if c.DBSize() != 100 {
		t.Fatalf("expected 100 keys, got %d", c.DBSize())
	}

	time.Sleep(200 * time.Millisecond)

	// Active expiry should have cleaned most of them
	remaining := c.DBSize()
	if remaining > 10 { // Allow some tolerance
		t.Fatalf("expected most keys expired, still have %d", remaining)
	}
}
