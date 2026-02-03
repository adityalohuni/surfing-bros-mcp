package page

import (
	"sync"

	"github.com/google/uuid"
)

type Store struct {
	mu     sync.RWMutex
	items  map[string]Snapshot
	latest string
}

func NewStore() *Store {
	return &Store{items: make(map[string]Snapshot)}
}

func (s *Store) Put(snapshot Snapshot) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := snapshot.ID
	if id == "" {
		id = uuid.New().String()
		snapshot.ID = id
	}
	s.items[id] = snapshot
	s.latest = id
	return id
}

func (s *Store) Get(id string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.items[id]
	return snap, ok
}

func (s *Store) Latest() (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest == "" {
		return Snapshot{}, false
	}
	snap, ok := s.items[s.latest]
	return snap, ok
}
