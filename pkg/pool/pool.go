// Package pool provides a generic object pool with a reset constraint.
package pool

import (
	"sync"
)

// Resetter is the constraint interface for Pool.
// Any type stored in Pool must implement this interface.
type Resetter interface {
	Reset()
}

// Pool is a generic object pool that stores objects of type T.
// T must implement Resetter interface.
// Pool is safe for concurrent use.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New creates a new Pool.
func New[T Resetter]() *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				var zero T
				return zero
			},
		},
	}
}

// Get retrieves an object from the pool.
// If pool is empty, a new zero value of T is returned.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool.
// Reset() method is called before storing it.
func (p *Pool[T]) Put(item T) {
	item.Reset()
	p.pool.Put(item)
}
