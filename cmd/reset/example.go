package main

// This file demonstrates how to use the reset generator

// Example 1: Simple struct with primitives, slices, and maps
// generate:reset
type ExampleStruct struct {
	ID       int
	Name     string
	Active   bool
	Tags     []string
	Metadata map[string]string
}

// Example 2: Struct with pointers
// generate:reset
type PointerStruct struct {
	Value  *int
	Text   *string
	Config *ExampleStruct
}

// Example 3: Nested struct that will call Reset() on children
// generate:reset
type NestedStruct struct {
	Parent   string
	Children []*ExampleStruct
	Primary  *PointerStruct
}

// To generate Reset() methods:
// 1. Run: go run cmd/reset/main.go
// 2. This will create reset.gen.go with Reset() methods for all marked structs
// 3. Use the Reset() methods in your code:
//
//    s := &ExampleStruct{
//        ID: 123,
//        Name: "test",
//        Tags: []string{"a", "b", "c"},
//    }
//    s.Reset() // Resets all fields to zero values
