package models

import "testing"

func TestResetStruct_Reset(t *testing.T) {
	// Create a struct with non-zero values
	value := 3.14
	s := &TestResetStruct{
		Counter: 42,
		Name:    "test",
		Active:  true,
		Tags:    []string{"tag1", "tag2", "tag3"},
		Data:    map[string]int{"key1": 1, "key2": 2},
		Value:   &value,
	}

	// Verify initial state
	if s.Counter != 42 {
		t.Errorf("Expected Counter to be 42, got %d", s.Counter)
	}
	if len(s.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(s.Tags))
	}
	if len(s.Data) != 2 {
		t.Errorf("Expected 2 data entries, got %d", len(s.Data))
	}

	// Reset the struct
	s.Reset()

	// Verify reset state
	if s.Counter != 0 {
		t.Errorf("Expected Counter to be 0 after reset, got %d", s.Counter)
	}
	if s.Name != "" {
		t.Errorf("Expected Name to be empty after reset, got %s", s.Name)
	}
	if s.Active != false {
		t.Errorf("Expected Active to be false after reset, got %v", s.Active)
	}
	if len(s.Tags) != 0 {
		t.Errorf("Expected Tags to be empty after reset, got length %d", len(s.Tags))
	}
	if s.Tags == nil {
		t.Error("Expected Tags slice to not be nil after reset (should be [:0])")
	}
	if len(s.Data) != 0 {
		t.Errorf("Expected Data map to be empty after reset, got length %d", len(s.Data))
	}
	if s.Value == nil {
		t.Error("Expected Value pointer to not be nil after reset")
	} else if *s.Value != 0 {
		t.Errorf("Expected dereferenced Value to be 0 after reset, got %f", *s.Value)
	}
}

func TestResetStruct_ResetNil(t *testing.T) {
	// Test that Reset on nil pointer doesn't panic
	var s *TestResetStruct
	s.Reset() // Should not panic
}

func TestResetStruct_ResetNilPointerField(t *testing.T) {
	// Test reset with nil pointer field
	s := &TestResetStruct{
		Counter: 42,
		Name:    "test",
		Value:   nil,
	}

	s.Reset()

	if s.Counter != 0 {
		t.Errorf("Expected Counter to be 0 after reset, got %d", s.Counter)
	}
	if s.Value != nil {
		t.Error("Expected Value to remain nil after reset")
	}
}

func TestComplexResetStruct_Reset(t *testing.T) {
	// Create nested structs with non-zero values
	value := 99.99
	child := &TestResetStruct{
		Counter: 100,
		Name:    "child",
		Active:  true,
		Tags:    []string{"child-tag"},
		Data:    map[string]int{"child-key": 10},
		Value:   &value,
	}

	childPtr := &ComplexResetStruct{
		ID:     200,
		Label:  "nested",
		Items:  []int{1, 2, 3},
		Config: map[string]string{"key": "value"},
	}

	s := &ComplexResetStruct{
		ID:       1,
		Label:    "parent",
		Items:    []int{10, 20, 30},
		Config:   map[string]string{"config1": "value1", "config2": "value2"},
		Child:    child,
		ChildPtr: childPtr,
	}

	// Reset the struct
	s.Reset()

	// Verify parent struct is reset
	if s.ID != 0 {
		t.Errorf("Expected ID to be 0 after reset, got %d", s.ID)
	}
	if s.Label != "" {
		t.Errorf("Expected Label to be empty after reset, got %s", s.Label)
	}
	if len(s.Items) != 0 {
		t.Errorf("Expected Items to be empty after reset, got length %d", len(s.Items))
	}
	if len(s.Config) != 0 {
		t.Errorf("Expected Config to be empty after reset, got length %d", len(s.Config))
	}

	// Verify child struct was also reset (via Reset() method)
	if s.Child == nil {
		t.Fatal("Expected Child to not be nil")
	}
	if s.Child.Counter != 0 {
		t.Errorf("Expected Child.Counter to be 0 after reset, got %d", s.Child.Counter)
	}
	if s.Child.Name != "" {
		t.Errorf("Expected Child.Name to be empty after reset, got %s", s.Child.Name)
	}
	if len(s.Child.Tags) != 0 {
		t.Errorf("Expected Child.Tags to be empty after reset, got length %d", len(s.Child.Tags))
	}

	// Verify nested child pointer was also reset
	if s.ChildPtr == nil {
		t.Fatal("Expected ChildPtr to not be nil")
	}
	if s.ChildPtr.ID != 0 {
		t.Errorf("Expected ChildPtr.ID to be 0 after reset, got %d", s.ChildPtr.ID)
	}
	if s.ChildPtr.Label != "" {
		t.Errorf("Expected ChildPtr.Label to be empty after reset, got %s", s.ChildPtr.Label)
	}
	if len(s.ChildPtr.Items) != 0 {
		t.Errorf("Expected ChildPtr.Items to be empty after reset, got length %d", len(s.ChildPtr.Items))
	}
}
