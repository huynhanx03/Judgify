---
name: benchmark-common
description: Benchmarking skill for comparing multiple implementations. Creates comprehensive performance comparisons with proper methodology. Use when asked to benchmark or compare performance of different approaches.
---

# Benchmark Common Skill

This skill provides **comprehensive benchmarking standards** for performance comparison.
Designed to compare **multiple implementations** in a single benchmark file.

---

## When to Use

- Compare different implementations (e.g., RingBuffer vs LinkedListBuffer)
- Measure performance of Common packages
- Validate optimization effectiveness
- Find performance bottlenecks

---

## Quality Standards

| Metric | Requirement |
|--------|-------------|
| **Warm-up** | Reset state between iterations |
| **Fair Comparison** | Same data sizes, same conditions |
| **Multiple Sizes** | Test small, medium, large data |
| **Memory Reporting** | Use `b.ReportAllocs()` |
| **Clear Naming** | `BenchmarkOperation_Implementation_Size` |

---

## Workflow (Strict 3-Step Process)

### Step 1: Benchmark Design

**Identify:**

1. **Implementations to compare**: List all types/approaches
2. **Operations to benchmark**: Core operations (Read, Write, etc.)
3. **Data sizes**: Small (64B), Medium (1KB), Large (1MB)
4. **Setup requirements**: Pre-allocation, warm-up needs

**Output Format:**
```markdown
## Benchmark Design: [comparison name]

### Implementations
- [Implementation 1]: Description
- [Implementation 2]: Description

### Operations
| Operation | Description | Why benchmark? |
|-----------|-------------|----------------|
| Write | Append data | Core operation |
| Read | Consume data | Core operation |

### Data Sizes
- Small: 64 bytes (cache-friendly)
- Medium: 1KB (typical payload)
- Large: 1MB (stress test)
```

**STOP and wait for user approval.**

---

### Step 2: Benchmark Case Design

**Create benchmark matrix:**

| Benchmark ID | Operation | Implementation | Size | Setup |
|--------------|-----------|----------------|------|-------|
| B1.1 | Write | RingBuffer | 64B | New(1024) |
| B1.2 | Write | LinkedListBuffer | 64B | New() |
| B2.1 | Write | RingBuffer | 1KB | New(4096) |
| B2.2 | Write | LinkedListBuffer | 1KB | New() |

**Considerations:**
- Same data for all implementations
- Pre-allocate to avoid setup cost in measurement
- Reset between iterations
- Report memory allocations

**STOP and wait for user approval.**

---

### Step 3: Benchmark Code Implementation

**Follow these Go benchmarking standards:**

#### File Structure
```
benchmark_comparison_test.go
├── // Data generators
│   var smallData = make([]byte, 64)
│   var mediumData = make([]byte, 1024)
│   var largeData = make([]byte, 1<<20)
│
├── // Comparison benchmarks (grouped by operation)
│   func BenchmarkWrite(b *testing.B)
│   ├── b.Run("RingBuffer/64B", ...)
│   ├── b.Run("RingBuffer/1KB", ...)
│   ├── b.Run("LinkedList/64B", ...)
│   └── b.Run("LinkedList/1KB", ...)
│
├── func BenchmarkRead(b *testing.B)
│   ├── b.Run("RingBuffer/64B", ...)
│   └── ...
```

#### Code Patterns

**Comparison Benchmark (Grouped by Operation):**
```go
func BenchmarkWrite(b *testing.B) {
    sizes := []struct {
        name string
        data []byte
    }{
        {"64B", make([]byte, 64)},
        {"1KB", make([]byte, 1024)},
        {"1MB", make([]byte, 1<<20)},
    }

    for _, size := range sizes {
        // RingBuffer
        b.Run("RingBuffer/"+size.name, func(b *testing.B) {
            buf := NewRing(len(size.data) * 2)
            b.ResetTimer()
            b.ReportAllocs()
            for i := 0; i < b.N; i++ {
                buf.Write(size.data)
                buf.Reset()
            }
        })

        // LinkedListBuffer
        b.Run("LinkedList/"+size.name, func(b *testing.B) {
            buf := &LinkedListBuffer{}
            b.ResetTimer()
            b.ReportAllocs()
            for i := 0; i < b.N; i++ {
                buf.PushBack(size.data)
                buf.Reset()
            }
        })
    }
}
```

**Read After Write Pattern:**
```go
func BenchmarkRead(b *testing.B) {
    data := make([]byte, 1024)
    readBuf := make([]byte, 1024)

    b.Run("RingBuffer", func(b *testing.B) {
        buf := NewRing(2048)
        b.ResetTimer()
        b.ReportAllocs()
        for i := 0; i < b.N; i++ {
            buf.Write(data)
            buf.Read(readBuf)
        }
    })

    b.Run("LinkedList", func(b *testing.B) {
        buf := &LinkedListBuffer{}
        b.ResetTimer()
        b.ReportAllocs()
        for i := 0; i < b.N; i++ {
            buf.PushBack(data)
            buf.Read(readBuf)
        }
    })
}
```

**Memory-Heavy Benchmark:**
```go
func BenchmarkMemory(b *testing.B) {
    b.Run("RingBuffer/Grow", func(b *testing.B) {
        b.ReportAllocs()
        for i := 0; i < b.N; i++ {
            buf := NewRing(64)
            for j := 0; j < 1000; j++ {
                buf.Write(make([]byte, 100))
            }
        }
    })
}
```

---

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem

# Run specific comparison
go test -bench=BenchmarkWrite -benchmem

# Run with count for stability
go test -bench=. -benchmem -count=5

# Compare with benchstat
go test -bench=. -benchmem -count=10 > old.txt
# (make changes)
go test -bench=. -benchmem -count=10 > new.txt
benchstat old.txt new.txt
```

---

## Output Interpretation

```
BenchmarkWrite/RingBuffer/64B-8    10000000    120 ns/op    0 B/op    0 allocs/op
BenchmarkWrite/LinkedList/64B-8    5000000     250 ns/op    64 B/op   1 allocs/op
```

| Column | Meaning |
|--------|---------|
| `-8` | GOMAXPROCS (8 cores) |
| `10000000` | Iterations run |
| `120 ns/op` | Time per operation |
| `0 B/op` | Bytes allocated per op |
| `0 allocs/op` | Allocations per op |

**Interpretation**: RingBuffer ~2x faster, no allocations.

---

## Rules (Non-negotiable)

1. **Always use b.ResetTimer()** after setup code
2. **Always use b.ReportAllocs()** for memory analysis
3. **Same data sizes** for fair comparison
4. **Reset state** between iterations
5. **Clear naming** convention: `Operation/Implementation/Size`
6. **Run multiple times** for stable results

---

## Best Practices

| Practice | Why |
|----------|-----|
| Pre-allocate data outside loop | Avoid measurement pollution |
| Use `b.StopTimer()` / `b.StartTimer()` | Exclude setup from measurement |
| Reset buffers after write | Simulate real usage pattern |
| Test multiple sizes | Find performance characteristics |
| Run `-count=10` | Statistical stability |

---

## Approval Prompt

After each step, ask:

> "Please review the above and confirm if I should proceed to the next step."
