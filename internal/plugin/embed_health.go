package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/SteamedBread2333/imprint/internal/loopback"
)

// embedSidecarHealth is GET /health from imprint-embed-sidecar.
type embedSidecarHealth struct {
	OK    bool `json:"ok"`
	Ready bool `json:"ready"`
}

// embedPluginStartTimeout bounds plugin start while run.sh --warm loads models.
const embedPluginStartTimeout = 180 * time.Second

// EmbedSidecarReady reports whether the sidecar finished warm-up (embed dim loaded,
// and relate model loaded when configured). Missing "ready" falls back to HTTP 2xx only.
func EmbedSidecarReady(port int) bool {
	if port <= 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	resp, err := loopback.Client(3 * time.Second).Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	var h embedSidecarHealth
	if err := json.Unmarshal(body, &h); err != nil {
		return false
	}
	if !h.OK {
		return false
	}
	// Sidecar >= 0.2.1 sets ready after --warm; older builds omit the field.
	if !h.Ready {
		var raw map[string]any
		if json.Unmarshal(body, &raw) == nil {
			if _, has := raw["ready"]; !has {
				return true
			}
		}
		return false
	}
	return true
}

func pluginStartHealthTimeout(id string) time.Duration {
	if id == "embed" {
		return embedPluginStartTimeout
	}
	return 8 * time.Second
}

func pluginRunning(id, healthTpl string, port int) bool {
	if id == "embed" {
		return EmbedSidecarReady(port)
	}
	return checkHealth(healthTpl, port)
}
