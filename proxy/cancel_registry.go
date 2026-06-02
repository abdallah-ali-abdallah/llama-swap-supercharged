package proxy

import (
	"context"
	"sync"
)

// requestCancelRegistry maps live activity IDs to request cancellation functions.
// It allows the server-side UI to cancel in-flight requests.
type requestCancelRegistry struct {
	mu       sync.RWMutex
	requests map[string]context.CancelFunc
}

func newRequestCancelRegistry() *requestCancelRegistry {
	return &requestCancelRegistry{
		requests: make(map[string]context.CancelFunc),
	}
}

func (r *requestCancelRegistry) Register(id string, cancel context.CancelFunc) {
	r.mu.Lock()
	r.requests[id] = cancel
	r.mu.Unlock()
}

// Cancel calls the cancel func for the given id, removes it from the registry,
// and returns true if the id was found.
func (r *requestCancelRegistry) Cancel(id string) bool {
	r.mu.Lock()
	cancel, ok := r.requests[id]
	if ok {
		delete(r.requests, id)
	}
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (r *requestCancelRegistry) Deregister(id string) {
	r.mu.Lock()
	delete(r.requests, id)
	r.mu.Unlock()
}
