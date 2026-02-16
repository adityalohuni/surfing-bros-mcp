package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/adityalohuni/mcp-server/internal/admin"
	"github.com/adityalohuni/mcp-server/internal/browser/wsbrowser"
	"github.com/adityalohuni/mcp-server/internal/config"
	"github.com/adityalohuni/mcp-server/internal/httpx"
	"github.com/adityalohuni/mcp-server/internal/mcpserver"
	"github.com/adityalohuni/mcp-server/internal/page"
	"github.com/adityalohuni/mcp-server/internal/session"
	"github.com/adityalohuni/mcp-server/internal/wsbridge"
)

func main() {
	settings, err := config.LoadOrCreate("")
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	log.Printf("loaded config: %s", settings.Path)

	bridge := wsbridge.NewBridge(wsbridge.Options{
		CheckOrigin: func(r *http.Request) bool { return true },
	})

	store := page.NewStore()
	reducer := page.NewReducer(page.ReduceOptions{})
	browser := wsbrowser.NewClient(bridge, reducer, store, wsbrowser.Options{})

	server := mcpserver.New(browser, store, mcpserver.Options{
		Implementation: &mcp.Implementation{Name: "surfingbro-browser", Version: "v1.0.0"},
		Instructions:   "Use browser.snapshot to get an LLM-friendly page view. Use browser.click to interact with elements.",
	})
	mcpServer := server.MCPServer()

	sseHandler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server { return mcpServer }, nil)
	streamHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return mcpServer }, nil)

	registry := session.NewRegistry()
	adminHandlers := &admin.Handlers{
		StartedAt:  time.Now(),
		Clients:    registry,
		Bridge:     bridge,
		Browser:    browser,
		MaxIdle:    settings.ClientMaxIdle,
		ConfigPath: settings.Path,
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", http.HandlerFunc(bridge.HandleWS))
	mux.Handle("/mcp/sse", httpx.RequireToken(settings.MCPToken)(trackSSE(registry, sseHandler)))
	mux.Handle("/mcp/stream", httpx.RequireToken(settings.MCPToken)(trackStreamable(registry, streamHandler)))
	mux.Handle("/admin/status", httpx.RequireToken(settings.AdminToken)(http.HandlerFunc(adminHandlers.Status)))
	mux.Handle("/admin/clients", httpx.RequireToken(settings.AdminToken)(http.HandlerFunc(adminHandlers.ClientsList)))
	mux.Handle("/admin/browsers", httpx.RequireToken(settings.AdminToken)(http.HandlerFunc(adminHandlers.BrowsersList)))
	mux.Handle("/admin/clients/disconnect", httpx.RequireToken(settings.AdminToken)(http.HandlerFunc(adminHandlers.DisconnectClient)))
	mux.Handle("/admin/browsers/disconnect", httpx.RequireToken(settings.AdminToken)(http.HandlerFunc(adminHandlers.DisconnectBrowser)))
	mux.Handle("/admin/config", httpx.RequireToken(settings.AdminToken)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			adminHandlers.ConfigGet(w, r)
		case http.MethodPut:
			adminHandlers.ConfigSet(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.Handle("/admin/ui", http.RedirectHandler("/admin/ui/", http.StatusFound))
	mux.Handle("/admin/ui/", http.StripPrefix("/admin/ui/", admin.UIHandler{Root: filepath.Join("web", "admin-ui", "dist")}))

	httpServer := &http.Server{
		Addr:    settings.DaemonAddr,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("mcp daemon listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func trackSSE(reg *session.Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := clientInfoFromRequest(r, "sse")
		clientID := ensureClient(reg, w, r, info)
		if clientID != "" {
			go func() {
				<-r.Context().Done()
				reg.Unregister(clientID)
			}()
		}
		next.ServeHTTP(w, r)
	})
}

func trackStreamable(reg *session.Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := clientInfoFromRequest(r, "streamable")
		if r.Method == http.MethodGet {
			clientID := ensureClient(reg, w, r, info)
			if clientID != "" {
				go func() {
					<-r.Context().Done()
					reg.Unregister(clientID)
				}()
			}
			next.ServeHTTP(w, r)
			return
		}
		clientID := clientIDFromRequest(r)
		if clientID != "" {
			reg.Touch(clientID, info)
		}
		next.ServeHTTP(w, r)
	})
}

func ensureClient(reg *session.Registry, w http.ResponseWriter, r *http.Request, info session.ClientInfo) string {
	clientID := clientIDFromRequest(r)
	if clientID == "" {
		clientID = reg.Register("", info)
		w.Header().Set("X-Assigned-Client-Id", clientID)
		return clientID
	}
	reg.Touch(clientID, info)
	return clientID
}

func clientInfoFromRequest(r *http.Request, transport string) session.ClientInfo {
	return session.ClientInfo{
		Name:       r.Header.Get("X-Client-Name"),
		Transport:  transport,
		RemoteAddr: httpx.ClientIP(r),
		UserAgent:  r.UserAgent(),
	}
}

func clientIDFromRequest(r *http.Request) string {
	if v := r.Header.Get("X-Client-Id"); v != "" {
		return v
	}
	if v := r.Header.Get("X-MCP-Client-Id"); v != "" {
		return v
	}
	return ""
}
