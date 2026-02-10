package wsbridge

import (
	"log"
	"os"
	"strings"
)

var wsbridgeDebug = func() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("MCP_WSBRIDGE_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}()

func debugf(format string, args ...any) {
	if wsbridgeDebug {
		log.Printf(format, args...)
	}
}
