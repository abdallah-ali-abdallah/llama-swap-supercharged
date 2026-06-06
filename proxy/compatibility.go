package proxy

import (
	"io"
	"strings"

	"github.com/mostlygeek/llama-swap/internal/logmon"
)

// LogMonitor is a compatibility alias for the upstream logmon.Monitor type.
type LogMonitor = logmon.Monitor

// NewLogMonitorWriter is a compatibility wrapper for logmon.NewWriter.
func NewLogMonitorWriter(w io.Writer) *LogMonitor {
	return logmon.NewWriter(w)
}

// hasImageIndicators checks if a request/response body contains image data indicators.
// This was moved here from the old metrics_monitor.go.
func hasImageIndicators(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	// Look for common image-related markers in the body
	s := string(body)
	return strings.Contains(s, "data:image/") ||
		strings.Contains(s, "image_url") ||
		strings.Contains(s, "b64_json") ||
		strings.Contains(s, "image/jpeg") ||
		strings.Contains(s, "image/png") ||
		strings.Contains(s, "image/webp") ||
		strings.Contains(s, "image/gif")
}
