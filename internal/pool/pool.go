// Package pool provides a generic object pool for types with Reset() method.
package pool

import "sync"

// Resetable is an interface that defines types that can be reset to their initial state.
type Resetable interface {
	Reset()
}

// Pool is a generic container for storing and reusing objects of a specific type.
// The type parameter T must implement the Resetable interface.
// Pool automatically calls Reset() on objects when they are returned via Put().
type Pool[T Resetable] struct {
	pool sync.Pool
	new  func() T
}

// New creates and returns a pointer to a new Pool for type T.
// The newFunc parameter is a factory function that creates new instances of T
// when the pool is empty and Get() is called.
func New[T Resetable](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return newFunc()
			},
		},
		new: newFunc,
	}
}

// Get retrieves an object from the pool.
// If the pool is empty, a new object is created using the factory function
// provided to New().
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool.
// Before adding the object to the pool, its Reset() method is called
// to ensure the object is in a clean state for reuse.
func (p *Pool[T]) Put(obj T) {
	// Reset the object before returning it to the pool
	obj.Reset()
	p.pool.Put(obj)
}

