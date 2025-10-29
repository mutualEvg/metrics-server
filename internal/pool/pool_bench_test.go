package pool

import "testing"

// Global variables to prevent compiler optimizations in benchmarks
var (
	benchResult    int
	benchResultStr string
)

func BenchmarkPool_GetPut(b *testing.B) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := p.Get()
		obj.Counter = i
		obj.Name = "benchmark"
		obj.Tags = append(obj.Tags, "tag1", "tag2", "tag3")
		obj.Data["key"] = i
		// Use the values to prevent optimization
		benchResult = obj.Counter
		benchResultStr = obj.Name
		p.Put(obj)
	}
}

func BenchmarkPool_NoReuse(b *testing.B) {
	// Benchmark without pool (creating new objects each time)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
		obj.Counter = i
		obj.Name = "benchmark"
		obj.Tags = append(obj.Tags, "tag1", "tag2", "tag3")
		obj.Data["key"] = i
		// Use the values to prevent optimization
		benchResult = obj.Counter
		benchResultStr = obj.Name
		// No reuse - object will be garbage collected
	}
}

func BenchmarkPool_Parallel(b *testing.B) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			obj := p.Get()
			obj.Counter = i
			obj.Name = "parallel"
			obj.Tags = append(obj.Tags, "tag")
			obj.Data["key"] = i
			p.Put(obj)
			i++
		}
	})
}

func BenchmarkPool_ComplexStruct(b *testing.B) {
	p := New(func() *ComplexTestStruct {
		return &ComplexTestStruct{
			Items:  make([]string, 0, 20),
			Config: make(map[string]string),
			Nested: &TestStruct{
				Tags: make([]string, 0, 10),
				Data: make(map[string]int),
			},
		}
	})

	var result int64
	var active bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := p.Get()
		obj.ID = int64(i)
		obj.Active = true
		obj.Items = append(obj.Items, "item1", "item2", "item3")
		obj.Config["key"] = "value"
		obj.Nested.Counter = i
		obj.Nested.Name = "nested"
		// Use the values to prevent optimization
		result = obj.ID
		active = obj.Active
		p.Put(obj)
	}
	// Prevent compiler from optimizing away the variables
	_ = result
	_ = active
}

func BenchmarkPool_ComplexStruct_NoReuse(b *testing.B) {
	// Benchmark without pool for complex struct
	var result int64
	var active bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := &ComplexTestStruct{
			Items:  make([]string, 0, 20),
			Config: make(map[string]string),
			Nested: &TestStruct{
				Tags: make([]string, 0, 10),
				Data: make(map[string]int),
			},
		}
		obj.ID = int64(i)
		obj.Active = true
		obj.Items = append(obj.Items, "item1", "item2", "item3")
		obj.Config["key"] = "value"
		obj.Nested.Counter = i
		obj.Nested.Name = "nested"
		// Use the values to prevent optimization
		result = obj.ID
		active = obj.Active
		// No reuse
	}
	// Prevent compiler from optimizing away the variables
	_ = result
	_ = active
}

func BenchmarkPool_LargeSlices(b *testing.B) {
	// Test with objects that have large pre-allocated slices
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 1000),
			Data: make(map[string]int, 100),
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := p.Get()
		// Add many elements
		for j := 0; j < 100; j++ {
			obj.Tags = append(obj.Tags, "tag")
		}
		for j := 0; j < 50; j++ {
			obj.Data["key"] = j
		}
		p.Put(obj)
	}
}

func BenchmarkPool_LargeSlices_NoReuse(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := &TestStruct{
			Tags: make([]string, 0, 1000),
			Data: make(map[string]int, 100),
		}
		// Add many elements
		for j := 0; j < 100; j++ {
			obj.Tags = append(obj.Tags, "tag")
		}
		for j := 0; j < 50; j++ {
			obj.Data["key"] = j
		}
		// No reuse
	}
}

