package workflow

import (
	"encoding/json"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/adityalohuni/mcp-server/internal/browser"
)

type Workflow struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Steps       []browser.RecordedAction `json:"steps"`
	CreatedAt   time.Time                `json:"createdAt"`
}

type Store struct {
	mu    sync.RWMutex
	items map[string]Workflow
	path  string
}

func NewStore(path string) *Store {
	s := &Store{items: make(map[string]Workflow), path: path}
	s.load()
	return s
}

func (s *Store) Add(w Workflow) Workflow {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now().UTC()
	}
	s.items[w.ID] = w
	_ = s.saveLocked()
	return w
}

func (s *Store) Get(id string) (Workflow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.items[id]
	return w, ok
}

func (s *Store) List() []Workflow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Workflow, 0, len(s.items))
	for _, w := range s.items {
		out = append(out, w)
	}
	return out
}

func (s *Store) Compact(limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 1000
	}
	if len(s.items) <= limit {
		return 0, nil
	}
	items := make([]Workflow, 0, len(s.items))
	for _, w := range s.items {
		items = append(items, w)
	}
	slices.SortFunc(items, func(a, b Workflow) int {
		if a.CreatedAt.Equal(b.CreatedAt) {
			return 0
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		return 1
	})
	removeCount := len(items) - limit
	for i := 0; i < removeCount; i++ {
		delete(s.items, items[i].ID)
	}
	if err := s.saveLocked(); err != nil {
		return 0, err
	}
	return removeCount, nil
}

func (s *Store) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var items []Workflow
	if err := json.Unmarshal(data, &items); err != nil {
		return
	}
	for _, w := range items {
		s.items[w.ID] = w
	}
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	items := make([]Workflow, 0, len(s.items))
	for _, w := range s.items {
		items = append(items, w)
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
