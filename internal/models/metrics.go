// Package models defines data structures for the metrics server API.
package models

// Metrics represents the structure for JSON API communication with the metrics server.
// It supports both gauge (floating-point) and counter (integer) metric types.
// Only one of Delta or Value should be set depending on the metric type.
type Metrics struct {
	// ID is the unique name/identifier of the metric
	ID string `json:"id"`

	// MType specifies the metric type: "gauge" or "counter"
	MType string `json:"type"`

	// Delta contains the value for counter metrics (integer)
	// This field is omitted from JSON if nil
	Delta *int64 `json:"delta,omitempty"`

	// Value contains the value for gauge metrics (floating-point)
	// This field is omitted from JSON if nil
	Value *float64 `json:"value,omitempty"`
}

// generate:reset
type TestResetStruct struct {
	Counter int
	Name    string
	Active  bool
	Tags    []string
	Data    map[string]int
	Value   *float64
}

// generate:reset
type ComplexResetStruct struct {
	ID       int64
	Label    string
	Items    []int
	Config   map[string]string
	Child    *TestResetStruct
	ChildPtr *ComplexResetStruct
}
