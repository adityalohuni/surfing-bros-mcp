package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/adityalohuni/mcp-server/internal/browser"
	"github.com/adityalohuni/mcp-server/internal/config"
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
	ConfigPath  string
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

func (h *Handlers) DisconnectClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	h.Clients.Unregister(id)
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

func (h *Handlers) DisconnectBrowser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	useActive := id == "active"
	if id == "active" {
		id = ""
	}
	if id == "" && !useActive {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := h.Bridge.DisconnectSession(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if useActive {
		id = "active"
	}
	writeJSON(w, map[string]any{"ok": true, "id": id})
}

type ConfigPayload struct {
	Path               string `json:"path,omitempty"`
	DaemonAddr         string `json:"daemon_addr"`
	MCPToken           string `json:"mcp_token"`
	AdminToken         string `json:"admin_token"`
	ClientMaxIdle      string `json:"client_max_idle"`
	AdminBaseURL       string `json:"admin_base_url"`
	TUIRefreshInterval string `json:"tui_refresh_interval"`
}

func (h *Handlers) ConfigGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	settings, err := config.LoadOrCreate(h.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, payloadFromSettings(settings))
}

func (h *Handlers) ConfigSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload ConfigPayload
	if err := decodeJSON(r.Body, &payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	maxIdle, err := time.ParseDuration(strings.TrimSpace(payload.ClientMaxIdle))
	if err != nil {
		http.Error(w, "invalid client_max_idle", http.StatusBadRequest)
		return
	}
	refresh, err := time.ParseDuration(strings.TrimSpace(payload.TUIRefreshInterval))
	if err != nil {
		http.Error(w, "invalid tui_refresh_interval", http.StatusBadRequest)
		return
	}

	next := config.Settings{
		Path:               strings.TrimSpace(payload.Path),
		DaemonAddr:         strings.TrimSpace(payload.DaemonAddr),
		MCPToken:           strings.TrimSpace(payload.MCPToken),
		AdminToken:         strings.TrimSpace(payload.AdminToken),
		ClientMaxIdle:      maxIdle,
		AdminBaseURL:       strings.TrimSpace(payload.AdminBaseURL),
		TUIRefreshInterval: refresh,
	}
	if next.Path == "" {
		next.Path = h.ConfigPath
	}

	saved, err := config.Save(next)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, payloadFromSettings(saved))
}

func payloadFromSettings(settings config.Settings) ConfigPayload {
	return ConfigPayload{
		Path:               settings.Path,
		DaemonAddr:         settings.DaemonAddr,
		MCPToken:           settings.MCPToken,
		AdminToken:         settings.AdminToken,
		ClientMaxIdle:      settings.ClientMaxIdle.String(),
		AdminBaseURL:       settings.AdminBaseURL,
		TUIRefreshInterval: settings.TUIRefreshInterval.String(),
	}
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

func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(io.LimitReader(r, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid json payload")
	}
	return nil
}
