package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type ClientInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	Transport   string    `json:"transport,omitempty"`
	RemoteAddr  string    `json:"remote_addr,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
}

type Registry struct {
	mu      sync.RWMutex
	clients map[string]*ClientInfo
}

func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]*ClientInfo)}
}

func (r *Registry) Register(id string, info ClientInfo) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == "" {
		id = uuid.New().String()
	}
	now := time.Now()
	info.ID = id
	if info.ConnectedAt.IsZero() {
		info.ConnectedAt = now
	}
	info.LastSeen = now
	r.clients[id] = &info
	return id
}

func (r *Registry) Touch(id string, info ClientInfo) {
	if id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if existing, ok := r.clients[id]; ok {
		if info.Name != "" {
			existing.Name = info.Name
		}
		if info.Transport != "" {
			existing.Transport = info.Transport
		}
		if info.RemoteAddr != "" {
			existing.RemoteAddr = info.RemoteAddr
		}
		if info.UserAgent != "" {
			existing.UserAgent = info.UserAgent
		}
		existing.LastSeen = now
		return
	}
	info.ID = id
	info.ConnectedAt = now
	info.LastSeen = now
	r.clients[id] = &info
}

func (r *Registry) Unregister(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, id)
}

func (r *Registry) List() []ClientInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ClientInfo, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, *c)
	}
	return out
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (r *Registry) Prune(maxIdle time.Duration) {
	if maxIdle <= 0 {
		return
	}
	cutoff := time.Now().Add(-maxIdle)
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, c := range r.clients {
		if c.LastSeen.Before(cutoff) {
			delete(r.clients, id)
		}
	}
}
