package wsbridge

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/adityalohuni/mcp-server/internal/protocol"
)

var ErrNoActiveSession = errors.New("no active browser session")

// Bridge manages websocket sessions and command/response routing.
type Bridge struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	activeID  string
	pending   map[string]chan protocol.Response
	upgrader  websocket.Upgrader
	writeWait time.Duration
}

// Options configures the websocket bridge.
type Options struct {
	CheckOrigin     func(*http.Request) bool
	ReadBufferSize  int
	WriteBufferSize int
	WriteWait       time.Duration
}

// Session represents a connected browser extension.
type Session struct {
	ID          string
	Conn        *websocket.Conn
	mu          sync.Mutex
	RemoteAddr  string
	UserAgent   string
	ConnectedAt time.Time
	LastSeen    time.Time
}

func NewBridge(opts Options) *Bridge {
	up := websocket.Upgrader{
		ReadBufferSize:  opts.ReadBufferSize,
		WriteBufferSize: opts.WriteBufferSize,
		CheckOrigin:     opts.CheckOrigin,
	}
	if up.ReadBufferSize == 0 {
		up.ReadBufferSize = 2048
	}
	if up.WriteBufferSize == 0 {
		up.WriteBufferSize = 2048
	}
	writeWait := opts.WriteWait
	if writeWait == 0 {
		writeWait = 5 * time.Second
	}

	return &Bridge{
		sessions:  make(map[string]*Session),
		pending:   make(map[string]chan protocol.Response),
		upgrader:  up,
		writeWait: writeWait,
	}
}

func (b *Bridge) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		http.Error(w, "could not open websocket", http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	now := time.Now()
	session := &Session{
		ID:          id,
		Conn:        conn,
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		ConnectedAt: now,
		LastSeen:    now,
	}

	b.mu.Lock()
	b.sessions[id] = session
	b.activeID = id
	b.mu.Unlock()

	log.Printf("ws connected: %s", id)
	b.readLoop(session)

	b.mu.Lock()
	delete(b.sessions, id)
	if b.activeID == id {
		b.activeID = ""
		for sid := range b.sessions {
			b.activeID = sid
			break
		}
	}
	b.mu.Unlock()

	conn.Close()
	log.Printf("ws disconnected: %s", id)
}

func (b *Bridge) readLoop(session *Session) {
	for {
		_, message, err := session.Conn.ReadMessage()
		if err != nil {
			return
		}
		session.mu.Lock()
		session.LastSeen = time.Now()
		session.mu.Unlock()
		var resp protocol.Response
		if err := json.Unmarshal(message, &resp); err != nil {
			log.Printf("ws invalid message: %v", err)
			continue
		}
		if resp.ID == "" {
			continue
		}
		b.deliver(resp)
	}
}

func (b *Bridge) deliver(resp protocol.Response) {
	b.mu.Lock()
	ch := b.pending[resp.ID]
	if ch != nil {
		delete(b.pending, resp.ID)
	}
	b.mu.Unlock()

	if ch != nil {
		ch <- resp
		close(ch)
	}
}

func (b *Bridge) activeSession() (*Session, error) {
	b.mu.RLock()
	id := b.activeID
	session := b.sessions[id]
	b.mu.RUnlock()
	if session == nil {
		return nil, ErrNoActiveSession
	}
	return session, nil
}

func (b *Bridge) sessionByID(id string) (*Session, error) {
	if id == "" {
		return b.activeSession()
	}
	b.mu.RLock()
	session := b.sessions[id]
	b.mu.RUnlock()
	if session == nil {
		return nil, ErrNoActiveSession
	}
	return session, nil
}

type SessionInfo struct {
	ID          string    `json:"id"`
	RemoteAddr  string    `json:"remote_addr,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
	Active      bool      `json:"active"`
}

func (b *Bridge) ListSessions() []SessionInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]SessionInfo, 0, len(b.sessions))
	for id, s := range b.sessions {
		s.mu.Lock()
		info := SessionInfo{
			ID:          id,
			RemoteAddr:  s.RemoteAddr,
			UserAgent:   s.UserAgent,
			ConnectedAt: s.ConnectedAt,
			LastSeen:    s.LastSeen,
			Active:      id == b.activeID,
		}
		s.mu.Unlock()
		out = append(out, info)
	}
	return out
}

func (b *Bridge) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.sessions)
}

// SendCommand sends a command to the active browser session and waits for a response.
func (b *Bridge) SendCommand(ctx context.Context, cmd protocol.Command) (protocol.Response, error) {
	session, err := b.sessionByID(cmd.SessionID)
	if err != nil {
		return protocol.Response{}, err
	}

	msg, err := json.Marshal(cmd)
	if err != nil {
		return protocol.Response{}, err
	}

	ch := make(chan protocol.Response, 1)
	b.mu.Lock()
	b.pending[cmd.ID] = ch
	b.mu.Unlock()

	session.mu.Lock()
	_ = session.Conn.SetWriteDeadline(time.Now().Add(b.writeWait))
	err = session.Conn.WriteMessage(websocket.TextMessage, msg)
	session.mu.Unlock()
	if err != nil {
		b.mu.Lock()
		delete(b.pending, cmd.ID)
		b.mu.Unlock()
		return protocol.Response{}, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.pending, cmd.ID)
		b.mu.Unlock()
		return protocol.Response{}, ctx.Err()
	}
}
