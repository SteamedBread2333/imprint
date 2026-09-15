package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// Manifest is imprint.plugin.json in a plugin package root.
type Manifest struct {
	APIVersion   int              `json:"apiVersion"`
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Capabilities []string         `json:"capabilities"`
	Entry        ManifestEntry    `json:"entry"`
	ConfigSchema map[string]any   `json:"configSchema,omitempty"`
	Tools        []ManifestTool   `json:"tools"`
	dir          string
}

// ManifestEntry describes how to start and health-check a plugin.
type ManifestEntry struct {
	Start  string `json:"start"`
	Health string `json:"health"`
}

// ManifestTool is one MCP tool proxied through imprint-mcp.
type ManifestTool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	Handler      string         `json:"handler"`
	RegisteredAs string         `json:"-"`
}

// LoadManifest reads imprint.plugin.json from packageDir.
func LoadManifest(packageDir string) (*Manifest, error) {
	packageDir = strings.TrimSpace(packageDir)
	if packageDir == "" {
		return nil, fmt.Errorf("plugin package path is empty")
	}
	path := filepath.Join(packageDir, "imprint.plugin.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m.ID == "" {
		return nil, fmt.Errorf("%s: missing id", path)
	}
	m.dir = packageDir
	return &m, nil
}

// PackageDir resolves entry.Package relative to workspace.
func PackageDir(workspace, pkg string) string {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return ""
	}
	if filepath.IsAbs(pkg) {
		return pkg
	}
	return filepath.Join(workspace, pkg)
}

// ResolveTools assigns RegisteredAs names with optional pluginId prefix on collision.
func ResolveTools(pluginID string, tools []ManifestTool, taken map[string]struct{}) []ManifestTool {
	if taken == nil {
		taken = map[string]struct{}{}
	}
	out := make([]ManifestTool, 0, len(tools))
	for _, t := range tools {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		reg := name
		if _, ok := taken[name]; ok {
			reg = pluginID + "_" + name
		}
		t.RegisteredAs = reg
		taken[reg] = struct{}{}
		out = append(out, t)
	}
	return out
}

// BaseURL returns the plugin HTTP root from port.
func BaseURL(port int) string {
	return imprint.LocalURL(port)
}
