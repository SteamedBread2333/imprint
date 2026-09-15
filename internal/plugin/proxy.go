package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CallTool forwards args to a plugin HTTP handler from manifest.
func CallTool(route ToolRoute, args json.RawMessage) (json.RawMessage, error) {
	method, path, err := parseHandler(route.Tool.Handler)
	if err != nil {
		return nil, err
	}
	path = substitutePath(path, args)
	url := strings.TrimRight(route.BaseURL, "/") + path
	var body io.Reader
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		body = bytes.NewReader(args)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: %w", route.PluginID, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("plugin %s: HTTP %d: %s", route.PluginID, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("plugin %s: non-JSON response", route.PluginID)
	}
	return json.RawMessage(raw), nil
}

func parseHandler(h string) (method, path string, err error) {
	h = strings.TrimSpace(h)
	parts := strings.Fields(h)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid handler %q", h)
	}
	return strings.ToUpper(parts[0]), parts[1], nil
}

func substitutePath(path string, args json.RawMessage) string {
	if !strings.Contains(path, "{") {
		return path
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return path
	}
	out := path
	for k, v := range m {
		placeholder := "{" + k + "}"
		if strings.Contains(out, placeholder) {
			out = strings.ReplaceAll(out, placeholder, fmt.Sprint(v))
		}
	}
	return out
}
