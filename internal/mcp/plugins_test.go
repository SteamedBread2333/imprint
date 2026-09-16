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

func TestLoadMigratesLegacyPluginsShelvesYAML(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".imprint")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "imprint.yaml")
	body := "vault: .imprint/memory\nplugins:\n  shelves:\n    enabled: true\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "imprint-test", Version: "test"}, nil)
		registerPluginTools(server, cfgPath)
	})
	if strings.Contains(stderr, "plugin config") && strings.Contains(stderr, "invalid") {
		t.Fatalf("legacy plugins.shelves should migrate, stderr=%q", stderr)
	}
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
  desk:
    enabled: true
    package: /nonexistent/imprint-desk-plugin
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "imprint-test", Version: "test"}, nil)
		registerPluginTools(server, cfgPath)
	})
	if !strings.Contains(stderr, `plugin "desk" skipped`) {
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
  desk:
    enabled: false
    package: /nonexistent/imprint-desk-plugin
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
