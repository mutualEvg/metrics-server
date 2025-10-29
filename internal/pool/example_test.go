package pool_test

import (
	"fmt"

	"github.com/mutualEvg/metrics-server/internal/models"
	"github.com/mutualEvg/metrics-server/internal/pool"
)

// Example demonstrates basic usage of the Pool with a resetable struct
func ExamplePool() {
	// Create a pool for TestResetStruct (which has a generated Reset method)
	p := pool.New(func() *models.TestResetStruct {
		return &models.TestResetStruct{
			Tags: make([]string, 0, 10),
			Data: make(map[string]int),
		}
	})

	// Get an object from the pool
	obj := p.Get()

	// Use the object
	obj.Counter = 42
	obj.Name = "example"
	obj.Tags = append(obj.Tags, "tag1", "tag2")

	fmt.Printf("Before Put: Counter=%d, Name=%s, Tags=%v\n", obj.Counter, obj.Name, obj.Tags)

	// Return the object to the pool (automatically resets it)
	p.Put(obj)

	// Get an object again (might be the same one, now reset)
	obj2 := p.Get()
	fmt.Printf("After Get: Counter=%d, Name=%s, Tags=%v\n", obj2.Counter, obj2.Name, obj2.Tags)

	// Output:
	// Before Put: Counter=42, Name=example, Tags=[tag1 tag2]
	// After Get: Counter=0, Name=, Tags=[]
}

// Example_complexStruct demonstrates using Pool with nested resetable structs
func Example_complexStruct() {
	// Create a pool for ComplexResetStruct
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

	// Get and use an object
	obj := p.Get()
	obj.ID = 123
	obj.Label = "parent"
	obj.Items = append(obj.Items, 1, 2, 3)
	obj.Child.Counter = 999
	obj.Child.Name = "child"

	fmt.Printf("Before Put: ID=%d, Label=%s, Items=%v, Child.Counter=%d\n",
		obj.ID, obj.Label, obj.Items, obj.Child.Counter)

	// Return to pool (resets both parent and child)
	p.Put(obj)

	// Get again
	obj2 := p.Get()
	fmt.Printf("After Get: ID=%d, Label=%s, Items=%v, Child.Counter=%d\n",
		obj2.ID, obj2.Label, obj2.Items, obj2.Child.Counter)

	// Output:
	// Before Put: ID=123, Label=parent, Items=[1 2 3], Child.Counter=999
	// After Get: ID=0, Label=, Items=[], Child.Counter=0
}

// Example_reuse demonstrates capacity preservation and reuse
func Example_reuse() {
	p := pool.New(func() *models.TestResetStruct {
		return &models.TestResetStruct{
			Tags: make([]string, 0, 100), // Pre-allocated capacity
			Data: make(map[string]int),
		}
	})

	obj := p.Get()
	
	// Add many elements
	for i := 0; i < 50; i++ {
		obj.Tags = append(obj.Tags, fmt.Sprintf("tag%d", i))
	}

	fmt.Printf("Before Put: len=%d, cap=%d\n", len(obj.Tags), cap(obj.Tags))
	
	// Return to pool
	p.Put(obj)

	// Get again - capacity is preserved!
	obj2 := p.Get()
	fmt.Printf("After Get: len=%d, cap=%d\n", len(obj2.Tags), cap(obj2.Tags))

	// Output:
	// Before Put: len=50, cap=100
	// After Get: len=0, cap=100
}

