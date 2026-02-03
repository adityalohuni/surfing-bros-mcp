package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/adityalohuni/mcp-server/internal/browser/wsbrowser"
	"github.com/adityalohuni/mcp-server/internal/mcpserver"
	"github.com/adityalohuni/mcp-server/internal/page"
	"github.com/adityalohuni/mcp-server/internal/wsbridge"
)

func main() {
	bridge := wsbridge.NewBridge(wsbridge.Options{
		CheckOrigin: func(r *http.Request) bool { return true },
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", bridge.HandleWS)

	httpServer := &http.Server{
		Addr:    ":9099",
		Handler: mux,
	}

	go func() {
		log.Printf("websocket server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("websocket server error: %v", err)
		}
	}()

	store := page.NewStore()
	reducer := page.NewReducer(page.ReduceOptions{})
	browser := wsbrowser.NewClient(bridge, reducer, store, wsbrowser.Options{})

	server := mcpserver.New(browser, store, mcpserver.Options{
		Implementation: &mcp.Implementation{Name: "surfingbro-browser", Version: "v1.0.0"},
		Instructions:   "Use browser.snapshot to get an LLM-friendly page view. Use browser.click to interact with elements.",
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
