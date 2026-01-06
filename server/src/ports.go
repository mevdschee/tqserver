package main

import (
	"errors"
	"sync"
)

// PortPool manages allocation of ports for workers
type PortPool struct {
	available chan int
	mu        sync.Mutex
}

// NewPortPool creates a new port pool with the given range
func NewPortPool(start, end int) *PortPool {
	pool := &PortPool{
		available: make(chan int, end-start+1),
	}
	for port := start; port <= end; port++ {
		pool.available <- port
	}
	return pool
}

// Acquire gets an available port from the pool
func (p *PortPool) Acquire() (int, error) {
	select {
	case port := <-p.available:
		return port, nil
	default:
		return 0, errors.New("no ports available")
	}
}

// Release returns a port to the pool
func (p *PortPool) Release(port int) {
	select {
	case p.available <- port:
	// Port released successfully
	default:
		// Port pool is full, ignore (should not happen)
	}
}
