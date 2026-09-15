package mcp

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPluginToolInputSchema(t *testing.T) {
	got := pluginToolInputSchema(nil)
	if got["type"] != "object" {
		t.Fatalf("nil schema type = %v", got["type"])
	}
	got = pluginToolInputSchema(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
	})
	if got["type"] != "object" {
		t.Fatalf("object schema type = %v", got["type"])
	}
}

func TestRegisterPluginToolsShelvesManifest(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "shelves-plugin")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(plugDir, "imprint.plugin.json")
	if err := os.WriteFile(manifest, []byte(`{
  "apiVersion": 1,
  "id": "shelves",
  "name": "imprint-shelves-plugin",
  "version": "0.1.0",
  "capabilities": ["tools"],
  "entry": { "start": "echo", "health": "GET http://127.0.0.1:${port}/health" },
  "tools": [{
    "name": "doc_search",
    "description": "search docs",
    "inputSchema": {
      "type": "object",
      "properties": { "query": { "type": "string" } },
      "required": ["query"]
    },
    "handler": "POST /search"
  }]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(dir, ".imprint")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "imprint.yaml")
	body := "vault: .imprint/memory\nplugins:\n  shelves:\n    enabled: true\n    package: " + plugDir + "\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "imprint-test", Version: "test"}, nil)
	registerPluginTools(server, cfgPath)
}

func TestRegisterPluginToolsMissingPackage(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".imprint")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "imprint.yaml")
	body := `vault: .imprint/memory
plugins:
  shelves:
    enabled: true
    package: /nonexistent/imprint-shelves-plugin
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "imprint-test", Version: "test"}, nil)
		registerPluginTools(server, cfgPath)
	})
	if !strings.Contains(stderr, `plugin "shelves" skipped`) {
		t.Fatalf("expected skip message, got %q", stderr)
	}
}

func TestRegisterPluginToolsDisabledPlugin(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".imprint")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "imprint.yaml")
	body := `vault: .imprint/memory
plugins:
  shelves:
    enabled: false
    package: ../imprint-shelves-plugin
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "imprint-test", Version: "test"}, nil)
		registerPluginTools(server, cfgPath)
	})
	if strings.Contains(stderr, "skipped") {
		t.Fatalf("disabled plugin should not log skip, got %q", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}
