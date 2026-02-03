package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/adityalohuni/mcp-server/internal/browser"
	"github.com/adityalohuni/mcp-server/internal/session"
	"github.com/adityalohuni/mcp-server/internal/wsbridge"
)

type Status struct {
	Uptime          string `json:"uptime"`
	MCPClients      int    `json:"mcp_clients"`
	BrowserSessions int    `json:"browser_sessions"`
}

type Handlers struct {
	StartedAt   time.Time
	Clients     *session.Registry
	Bridge      *wsbridge.Bridge
	Browser     browser.Browser
	TabsTimeout time.Duration
	MaxIdle     time.Duration
}

func (h *Handlers) Status(w http.ResponseWriter, _ *http.Request) {
	h.prune()
	resp := Status{
		Uptime:          time.Since(h.StartedAt).String(),
		MCPClients:      h.Clients.Count(),
		BrowserSessions: h.Bridge.Count(),
	}
	writeJSON(w, resp)
}

func (h *Handlers) ClientsList(w http.ResponseWriter, _ *http.Request) {
	h.prune()
	writeJSON(w, h.Clients.List())
}

func (h *Handlers) BrowsersList(w http.ResponseWriter, _ *http.Request) {
	sessions := h.Bridge.ListSessions()
	resp := make([]BrowserSession, 0, len(sessions))
	for _, s := range sessions {
		entry := BrowserSession{
			SessionInfo: s,
		}
		if h.Browser != nil {
			ctx, cancel := context.WithTimeout(context.Background(), h.tabsTimeout())
			target := browser.Target{SessionID: s.ID}
			tabs, err := h.Browser.ListTabs(browser.WithTarget(ctx, target))
			cancel()
			if err != nil {
				entry.TabsError = err.Error()
			} else {
				entry.Tabs = tabs
			}
		}
		resp = append(resp, entry)
	}
	writeJSON(w, resp)
}

func (h *Handlers) prune() {
	if h.MaxIdle > 0 {
		h.Clients.Prune(h.MaxIdle)
	}
}

func (h *Handlers) tabsTimeout() time.Duration {
	if h.TabsTimeout <= 0 {
		return 2 * time.Second
	}
	return h.TabsTimeout
}

type BrowserSession struct {
	wsbridge.SessionInfo
	Tabs      []browser.TabInfo `json:"tabs,omitempty"`
	TabsError string            `json:"tabs_error,omitempty"`
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}
