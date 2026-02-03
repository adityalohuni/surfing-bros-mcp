package wsbridge

import (
	"context"
	"testing"

	"github.com/adityalohuni/mcp-server/internal/protocol"
)

func TestSendCommandWithoutSession(t *testing.T) {
	b := NewBridge(Options{})
	_, err := b.SendCommand(context.Background(), protocol.Command{ID: "1", Type: protocol.CommandClick})
	if err == nil {
		t.Fatalf("expected error when no session is active")
	}
}
