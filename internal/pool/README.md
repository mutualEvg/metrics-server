# Generic Pool Package

A type-safe, generic object pool implementation for Go that works with types implementing the `Reset()` method.

## Overview

This package provides a generic `Pool[T]` structure that efficiently reuses objects to reduce memory allocations and garbage collection pressure. It's particularly useful for "heavy" objects that are expensive to create and can be reset to a clean state for reuse.

## Features

- **Type-safe**: Uses Go generics for compile-time type safety
- **Automatic reset**: Calls `Reset()` on objects before returning them to the pool
- **Zero allocations**: Reusing objects eliminates allocation overhead
- **Thread-safe**: Built on top of `sync.Pool` for concurrent access
- **Integration with code generation**: Works seamlessly with the reset generator from Increment 21

## Installation

The package is part of the metrics-server project. Import it as:

```go
import "github.com/mutualEvg/metrics-server/internal/pool"
```

## Usage

### Basic Usage

```go
// Define a type with a Reset() method
type MyStruct struct {
    Counter int
    Tags    []string
    Data    map[string]int
}

func (m *MyStruct) Reset() {
    m.Counter = 0
    m.Tags = m.Tags[:0]
    clear(m.Data)
}

// Create a pool
p := pool.New(func() *MyStruct {
    return &MyStruct{
        Tags: make([]string, 0, 10),
        Data: make(map[string]int),
    }
})

// Get an object from the pool
obj := p.Get()

// Use the object
obj.Counter = 42
obj.Tags = append(obj.Tags, "tag1", "tag2")

// Return to pool (automatically calls Reset())
p.Put(obj)
```

### With Generated Reset Methods

The pool works perfectly with structs that have generated `Reset()` methods from the reset generator:

```go
import (
    "github.com/mutualEvg/metrics-server/internal/models"
    "github.com/mutualEvg/metrics-server/internal/pool"
)

// Create a pool for a struct with generated Reset() method
p := pool.New(func() *models.TestResetStruct {
    return &models.TestResetStruct{
        Tags: make([]string, 0, 10),
        Data: make(map[string]int),
    }
})

obj := p.Get()
obj.Counter = 100
obj.Name = "example"
p.Put(obj) // Automatically reset before returning to pool
```

### Nested Structs

The pool handles nested structs that also have `Reset()` methods:

```go
p := pool.New(func() *models.ComplexResetStruct {
    return &models.ComplexResetStruct{
        Items:  make([]int, 0, 10),
        Config: make(map[string]string),
        Child: &models.TestResetStruct{
            Tags: make([]string, 0, 10),
            Data: make(map[string]int),
        },
    }
})

obj := p.Get()
obj.ID = 123
obj.Child.Counter = 999  // Nested struct
p.Put(obj) // Resets both parent and child
```

## API Reference

### Type: `Resetable`

```go
type Resetable interface {
    Reset()
}
```

Interface that types must implement to be used with the Pool.

### Type: `Pool[T Resetable]`

```go
type Pool[T Resetable] struct {
    // ... internal fields
}
```

Generic pool container for objects of type `T`.

### Function: `New[T Resetable](newFunc func() T) *Pool[T]`

Creates a new Pool with the specified factory function.

**Parameters:**
- `newFunc`: Function that creates new instances of `T` when the pool is empty

**Returns:**
- Pointer to a new `Pool[T]`

**Example:**
```go
p := pool.New(func() *MyStruct {
    return &MyStruct{
        Tags: make([]string, 0, 100),
    }
})
```

### Method: `Get() T`

Retrieves an object from the pool. If the pool is empty, creates a new object using the factory function.

**Returns:**
- An object of type `T`

**Example:**
```go
obj := p.Get()
```

### Method: `Put(obj T)`

Returns an object to the pool. Automatically calls `obj.Reset()` before adding it to the pool.

**Parameters:**
- `obj`: The object to return to the pool

**Example:**
```go
p.Put(obj)
```

## Performance

The pool provides significant performance benefits by reducing allocations and GC pressure:

### Benchmark Results

```
BenchmarkPool_GetPut-16                   38.15 ns/op       0 B/op       0 allocs/op
BenchmarkPool_NoReuse-16                 111.9 ns/op      416 B/op       3 allocs/op

BenchmarkPool_ComplexStruct-16            42.43 ns/op       0 B/op       0 allocs/op
BenchmarkPool_ComplexStruct_NoReuse-16   269.1 ns/op      928 B/op       6 allocs/op

BenchmarkPool_LargeSlices-16             768.1 ns/op        0 B/op       0 allocs/op
BenchmarkPool_LargeSlices_NoReuse-16    2791 ns/op      19928 B/op       5 allocs/op
```

**Key Takeaways:**
- **Simple structs**: ~3x faster, 0 allocations vs 3 allocations
- **Complex structs**: ~6x faster, 0 allocations vs 6 allocations
- **Large objects**: ~3.6x faster, 0 allocations vs 5 allocations

### When to Use

Use the pool when:
- Objects are expensive to create (many fields, large slices/maps)
- Objects are created and discarded frequently
- You want to reduce GC pressure
- Objects can be safely reset to a clean state

Don't use the pool when:
- Objects are simple and cheap to create
- Objects are long-lived
- Object creation rate is low

## Best Practices

### 1. Pre-allocate Capacity

Pre-allocate slice and map capacity in the factory function:

```go
p := pool.New(func() *MyStruct {
    return &MyStruct{
        Tags: make([]string, 0, 100),  // Pre-allocate capacity
        Data: make(map[string]int, 50),
    }
})
```

### 2. Proper Reset Implementation

Ensure your `Reset()` method:
- Resets all fields to zero values
- Uses `slice[:0]` to preserve capacity
- Uses `clear()` for maps
- Calls `Reset()` on nested resetable structs

```go
func (m *MyStruct) Reset() {
    m.Counter = 0
    m.Name = ""
    m.Tags = m.Tags[:0]      // Preserves capacity
    clear(m.Data)             // Clears map
    if m.Child != nil {
        m.Child.Reset()       // Reset nested struct
    }
}
```

### 3. Use defer for Automatic Return

Use `defer` to ensure objects are returned to the pool:

```go
func processData() error {
    obj := p.Get()
    defer p.Put(obj)
    
    // Use obj...
    return nil
}
```

### 4. Don't Hold References

Don't keep references to pooled objects after calling `Put()`:

```go
// BAD: Don't do this
obj := p.Get()
savedRef := obj
p.Put(obj)
// savedRef might be reused by another goroutine!

// GOOD: Release all references
obj := p.Get()
// Use obj...
p.Put(obj)
obj = nil  // Optional but clear
```

## Thread Safety

The pool is safe for concurrent use. Multiple goroutines can call `Get()` and `Put()` simultaneously without external synchronization.

```go
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        obj := p.Get()
        defer p.Put(obj)
        // Use obj...
    }()
}
wg.Wait()
```

## Integration with Reset Generator

This pool is designed to work seamlessly with the reset generator from Increment 21:

1. Mark your structs with `// generate:reset`
2. Run `go run cmd/reset/main.go` to generate `Reset()` methods
3. Use the structs with this pool

```go
// In your code:
// generate:reset
type MyStruct struct {
    // ...fields
}

// After running the generator, use with pool:
p := pool.New(func() *MyStruct {
    return &MyStruct{/* initialize */}
})
```

## Examples

See `example_test.go` for complete working examples, or run:

```bash
go test ./internal/pool/ -run Example -v
```

## Benchmarks

Run benchmarks to see performance improvements:

```bash
go test ./internal/pool/ -bench=. -benchmem
```

## License

Part of the metrics-server project.

