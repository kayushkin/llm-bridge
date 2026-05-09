package identity

import "sync"

// MemoryStore is an in-process Store backed by a sync.Map. Adapters that
// only run for the lifetime of a single harness subprocess (no resume across
// adapter restarts) can use this directly; adapters that need crash-safe
// resume should back Store with SQLite or another durable store.
type MemoryStore struct {
	mu sync.RWMutex
	m  map[string]string
}

// NewMemoryStore constructs an empty in-memory Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{m: map[string]string{}}
}

// Lookup implements Store.
func (s *MemoryStore) Lookup(harnessID string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[harnessID]
	return v, ok, nil
}

// Put implements Store.
func (s *MemoryStore) Put(harnessID, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[harnessID] = messageID
	return nil
}
