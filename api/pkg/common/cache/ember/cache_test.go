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

	c.Set("hello", "world", 1)
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

	c.Set("key", 42, 1)
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
		c.Set(fmt.Sprintf("key%d", i), i, 1)
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

	c.SetWithTTL("ttlkey", "value", 1, 50*time.Millisecond)

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

	c.SetWithTTL("k", "v", 1, 1*time.Second)

	ttl, err := c.TTL("k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ttl <= 0 || ttl > 1*time.Second {
		t.Fatalf("unexpected TTL: %v", ttl)
	}

	// Key without TTL
	c.Set("persistent", "val", 1)
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

	c.Set("k", "v", 1)
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

	c.SetWithTTL("k", "v", 1, 1*time.Second)
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
		c.Set(fmt.Sprintf("key%d", i), i, 1)
	}

	// Access key0 to make it recently used
	c.Get("key0")

	// Add more items to trigger eviction
	for i := 5; i < 10; i++ {
		c.Set(fmt.Sprintf("key%d", i), i, 1)
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
		c.Set(fmt.Sprintf("key%d", i), i, 1)
	}

	// Access key0 many times to increase its LFU counter
	for j := 0; j < 20; j++ {
		c.Get("key0")
	}

	// Add more items
	for i := 5; i < 10; i++ {
		c.Set(fmt.Sprintf("key%d", i), i, 1)
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
		c.Set(fmt.Sprintf("key%d", i), i, 1)
	}

	if c.DBSize() > 5 {
		t.Fatalf("expected max 5 keys, got %d", c.DBSize())
	}
}

func TestEvictNoEviction(t *testing.T) {
	c := New[string, int](WithMaxEntries(3), WithEvictPolicy(EvictNoEviction))
	defer c.Close()

	c.Set("a", 1, 1)
	c.Set("b", 2, 1)
	c.Set("c", 3, 1)

	// Cache is full, next set should fail
	ok := c.Set("d", 4, 1)
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

	c.Set("a", 1, 1)
	c.Set("b", 2, 1)
	c.Set("c", 3, 1)
	c.Set("d", 4, 1) // triggers eviction

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

	c.Set("a", "1", 1)
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

	ok := c.SetNX("key", "first", 1, 0)
	if !ok {
		t.Fatal("expected SetNX to succeed on new key")
	}

	ok = c.SetNX("key", "second", 1, 0)
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
	val, existed := c.GetOrSet("key", func() (int, int64, time.Duration) {
		return 42, 1, 0
	})
	if existed || val != 42 {
		t.Fatalf("expected new value 42, got %d, existed=%v", val, existed)
	}

	// Second call: returns cached value
	val, existed = c.GetOrSet("key", func() (int, int64, time.Duration) {
		return 99, 1, 0 // should not be used
	})
	if !existed || val != 42 {
		t.Fatalf("expected cached 42, got %d, existed=%v", val, existed)
	}
}

func TestGetDel(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.Set("key", "value", 1)

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
	c.MSet(keys, values, 1)

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

func TestIncrByFloat(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	val, err := c.IncrByFloat("fkey", 1.5)
	if err != nil || val != 1.5 {
		t.Fatalf("expected 1.5, got %f, err=%v", val, err)
	}

	val, err = c.IncrByFloat("fkey", 2.3)
	if err != nil || val != 3.8 {
		t.Fatalf("expected 3.8, got %f, err=%v", val, err)
	}
}

func TestAppend(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	n, err := c.Append("key", "hello")
	if err != nil || n != 5 {
		t.Fatalf("expected len 5, got %d, err=%v", n, err)
	}

	n, err = c.Append("key", " world")
	if err != nil || n != 11 {
		t.Fatalf("expected len 11, got %d, err=%v", n, err)
	}
}

// -- Hash --

func TestHash(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.HSet("myhash", "field1", "value1")
	c.HSet("myhash", "field2", "value2")

	val, err := c.HGet("myhash", "field1")
	if err != nil || val != "value1" {
		t.Fatalf("expected 'value1', got '%s', err=%v", val, err)
	}

	hlen, _ := c.HLen("myhash")
	if hlen != 2 {
		t.Fatalf("expected 2 fields, got %d", hlen)
	}

	exists, _ := c.HExists("myhash", "field1")
	if !exists {
		t.Fatal("expected field1 to exist")
	}

	all, _ := c.HGetAll("myhash")
	if len(all) != 2 {
		t.Fatalf("expected 2 fields in HGetAll, got %d", len(all))
	}

	deleted, _ := c.HDel("myhash", "field1")
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}
}

// -- List --

func TestList(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	c.LPush("list", 1, 2, 3)  // [3, 2, 1]
	c.RPush("list", 4, 5)     // [3, 2, 1, 4, 5]

	llen, _ := c.LLen("list")
	if llen != 5 {
		t.Fatalf("expected 5, got %d", llen)
	}

	val, _ := c.LPop("list") // [2, 1, 4, 5]
	if val != 3 {
		t.Fatalf("expected 3 from LPop, got %d", val)
	}

	val, _ = c.RPop("list") // [2, 1, 4]
	if val != 5 {
		t.Fatalf("expected 5 from RPop, got %d", val)
	}

	rng, _ := c.LRange("list", 0, -1)
	if len(rng) != 3 {
		t.Fatalf("expected 3 items in range, got %d", len(rng))
	}
}

// -- Set --

func TestSet(t *testing.T) {
	c := New[string, string](WithMaxEntries(100))
	defer c.Close()

	c.SAdd("myset", "a", "b", "c")

	card, _ := c.SCard("myset")
	if card != 3 {
		t.Fatalf("expected 3, got %d", card)
	}

	isMember, _ := c.SIsMember("myset", "a")
	if !isMember {
		t.Fatal("expected 'a' to be a member")
	}

	c.SRem("myset", "a")
	card, _ = c.SCard("myset")
	if card != 2 {
		t.Fatalf("expected 2 after remove, got %d", card)
	}

	members, _ := c.SMembers("myset")
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

// -- Sorted Set --

func TestZSet(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	c.ZAdd("zs", 1.0, "alice")
	c.ZAdd("zs", 2.0, "bob")
	c.ZAdd("zs", 3.0, "charlie")

	score, err := c.ZScore("zs", "bob")
	if err != nil || score != 2.0 {
		t.Fatalf("expected 2.0, got %f, err=%v", score, err)
	}

	rank, err := c.ZRank("zs", "alice")
	if err != nil || rank != 0 {
		t.Fatalf("expected rank 0, got %d, err=%v", rank, err)
	}

	card, _ := c.ZCard("zs")
	if card != 3 {
		t.Fatalf("expected 3, got %d", card)
	}

	members, _ := c.ZRange("zs", 0, 1)
	if len(members) != 2 {
		t.Fatalf("expected 2, got %d", len(members))
	}

	c.ZRem("zs", "bob")
	card, _ = c.ZCard("zs")
	if card != 2 {
		t.Fatalf("expected 2 after remove, got %d", card)
	}
}

// -- Bitmap --

func TestBitmap(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	c.SetBit("bm", 7, 1)
	c.SetBit("bm", 0, 1)

	bit, _ := c.GetBit("bm", 7)
	if bit != 1 {
		t.Fatalf("expected bit 1, got %d", bit)
	}

	bit, _ = c.GetBit("bm", 3)
	if bit != 0 {
		t.Fatalf("expected bit 0, got %d", bit)
	}

	count, _ := c.BitCount("bm", 0, 0) // first byte
	if count != 2 {
		t.Fatalf("expected 2 bits set, got %d", count)
	}
}

// -- Exists / Type / Keys / DBSize --

func TestExistsAndType(t *testing.T) {
	c := New[string, any](WithMaxEntries(100))
	defer c.Close()

	c.Set("str", "hello", 1)
	c.HSet("hsh", "f", "v")

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

	typ, _ = c.Type("hsh")
	if typ != "hash" {
		t.Fatalf("expected 'hash', got '%s'", typ)
	}
}

func TestKeysAndDBSize(t *testing.T) {
	c := New[string, int](WithMaxEntries(100))
	defer c.Close()

	c.Set("a", 1, 1)
	c.Set("b", 2, 1)
	c.Set("c", 3, 1)

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
				c.Set(key, i*100+j, 1)
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
				c.Set(key, j, 1)
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
		c.SetWithTTL(fmt.Sprintf("key%d", i), "val", 1, 30*time.Millisecond)
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
