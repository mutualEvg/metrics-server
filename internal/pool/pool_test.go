package pool

import (
	"sync"
	"testing"
)

// TestStruct is a test structure that implements Resetable
type TestStruct struct {
	Counter int
	Name    string
	Tags    []string
	Data    map[string]int
}

func (ts *TestStruct) Reset() {
	ts.Counter = 0
	ts.Name = ""
	ts.Tags = ts.Tags[:0]
	clear(ts.Data)
}

func TestPool_New(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestPool_GetPut(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	// Get an object from the pool
	obj := p.Get()
	if obj == nil {
		t.Fatal("Get() returned nil")
	}

	// Modify the object
	obj.Counter = 42
	obj.Name = "test"
	obj.Tags = append(obj.Tags, "tag1", "tag2")
	obj.Data["key1"] = 100

	// Verify modifications
	if obj.Counter != 42 {
		t.Errorf("Expected Counter to be 42, got %d", obj.Counter)
	}
	if len(obj.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(obj.Tags))
	}
	if len(obj.Data) != 1 {
		t.Errorf("Expected 1 data entry, got %d", len(obj.Data))
	}

	// Put the object back in the pool (should call Reset())
	p.Put(obj)

	// Get the object again
	obj2 := p.Get()

	// Verify that the object was reset
	if obj2.Counter != 0 {
		t.Errorf("Expected Counter to be 0 after reset, got %d", obj2.Counter)
	}
	if obj2.Name != "" {
		t.Errorf("Expected Name to be empty after reset, got %s", obj2.Name)
	}
	if len(obj2.Tags) != 0 {
		t.Errorf("Expected Tags to be empty after reset, got length %d", len(obj2.Tags))
	}
	if len(obj2.Data) != 0 {
		t.Errorf("Expected Data to be empty after reset, got length %d", len(obj2.Data))
	}
}

func TestPool_MultipleObjects(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	// Get multiple objects
	obj1 := p.Get()
	obj2 := p.Get()
	obj3 := p.Get()

	// Modify each object differently
	obj1.Counter = 1
	obj2.Counter = 2
	obj3.Counter = 3

	// Put them all back
	p.Put(obj1)
	p.Put(obj2)
	p.Put(obj3)

	// Get objects again and verify they are reset
	for i := 0; i < 3; i++ {
		obj := p.Get()
		if obj.Counter != 0 {
			t.Errorf("Object %d: Expected Counter to be 0 after reset, got %d", i, obj.Counter)
		}
	}
}

func TestPool_Concurrent(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Get object from pool
				obj := p.Get()

				// Modify it
				obj.Counter = id
				obj.Name = "goroutine"
				obj.Tags = append(obj.Tags, "tag")
				obj.Data["key"] = id

				// Put it back
				p.Put(obj)
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, verify that objects in pool are reset
	obj := p.Get()
	if obj.Counter != 0 {
		t.Errorf("Expected Counter to be 0 after concurrent operations, got %d", obj.Counter)
	}
	if obj.Name != "" {
		t.Errorf("Expected Name to be empty after concurrent operations, got %s", obj.Name)
	}
	if len(obj.Tags) != 0 {
		t.Errorf("Expected Tags to be empty after concurrent operations, got length %d", len(obj.Tags))
	}
	if len(obj.Data) != 0 {
		t.Errorf("Expected Data to be empty after concurrent operations, got length %d", len(obj.Data))
	}
}

// ComplexTestStruct is a more complex test structure
type ComplexTestStruct struct {
	ID     int64
	Active bool
	Items  []string
	Config map[string]string
	Nested *TestStruct
}

func (cts *ComplexTestStruct) Reset() {
	cts.ID = 0
	cts.Active = false
	cts.Items = cts.Items[:0]
	clear(cts.Config)
	if cts.Nested != nil {
		cts.Nested.Reset()
	}
}

func TestPool_WithGeneratedResetStruct(t *testing.T) {
	// This test demonstrates using the pool with a more complex struct
	p := New(func() *ComplexTestStruct {
		return &ComplexTestStruct{
			Items:  make([]string, 0, 10),
			Config: make(map[string]string),
			Nested: &TestStruct{
				Tags: make([]string, 0, 10),
				Data: make(map[string]int),
			},
		}
	})

	obj := p.Get()
	obj.ID = 999
	obj.Active = true
	obj.Items = append(obj.Items, "item1", "item2")
	obj.Config["key"] = "value"
	obj.Nested.Counter = 100

	p.Put(obj)

	obj2 := p.Get()
	if obj2.ID != 0 {
		t.Errorf("Expected ID to be 0 after reset, got %d", obj2.ID)
	}
	if obj2.Active {
		t.Error("Expected Active to be false after reset")
	}
	if len(obj2.Items) != 0 {
		t.Errorf("Expected Items to be empty after reset, got length %d", len(obj2.Items))
	}
	if obj2.Nested.Counter != 0 {
		t.Errorf("Expected nested Counter to be 0 after reset, got %d", obj2.Nested.Counter)
	}
}

func TestPool_CapacityPreservation(t *testing.T) {
	// Test that slices preserve their capacity after reset
	p := New(func() *TestStruct {
		return &TestStruct{
			Tags: make([]string, 0, 100), // Capacity of 100
			Data: make(map[string]int, 50),
		}
	})

	obj := p.Get()
	
	// Add elements
	for i := 0; i < 50; i++ {
		obj.Tags = append(obj.Tags, "tag")
	}

	initialCap := cap(obj.Tags)
	if initialCap < 50 {
		t.Fatalf("Expected initial capacity >= 50, got %d", initialCap)
	}

	// Put back (should reset but preserve capacity)
	p.Put(obj)

	// Get again
	obj2 := p.Get()

	// Verify length is 0 but capacity is preserved
	if len(obj2.Tags) != 0 {
		t.Errorf("Expected length 0, got %d", len(obj2.Tags))
	}
	if cap(obj2.Tags) < 50 {
		t.Errorf("Expected capacity to be preserved (>= 50), got %d", cap(obj2.Tags))
	}
}

