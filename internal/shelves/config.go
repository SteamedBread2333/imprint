package shelves

import (
	"path/filepath"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// Config controls built-in workspace document indexing.
type Config struct {
	Enabled   bool
	Roots     []string
	StateDir  string
	Workspace string
}

// DefaultRoots are indexed when shelves is enabled and roots are omitted.
var DefaultRoots = []string{"docs"}

// ConfigFrom reads top-level shelves: from imprint.yaml.
func ConfigFrom(cfg *plugin.Config) Config {
	out := Config{
		Enabled:   false,
		Roots:     append([]string(nil), DefaultRoots...),
	}
	if cfg == nil {
		return out
	}
	out.Workspace = cfg.Workspace()
	out.StateDir = plugin.DefaultShelvesStateDir(out.Workspace)
	out.Enabled = cfg.Shelves.Enabled
	if roots := parseRoots(cfg.Shelves.Config); len(roots) > 0 {
		out.Roots = roots
	}
	if cfg.Shelves.Config != nil {
		if s, ok := cfg.Shelves.Config["stateDir"].(string); ok && strings.TrimSpace(s) != "" {
			out.StateDir = strings.TrimSpace(s)
			if !filepath.IsAbs(out.StateDir) {
				out.StateDir = filepath.Join(out.Workspace, filepath.FromSlash(out.StateDir))
			}
		}
	}
	return out
}

func parseRoots(config map[string]any) []string {
	if config == nil {
		return nil
	}
	raw, ok := config["roots"]
	if !ok {
		return nil
	}
	switch t := raw.(type) {
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		var out []string
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

// ResolveStateDir returns an absolute shelves cache directory.
func (c Config) ResolveStateDir() string {
	if filepath.IsAbs(c.StateDir) {
		return c.StateDir
	}
	if strings.TrimSpace(c.StateDir) == "" {
		return imprint.DefaultShelvesCacheDir(c.Workspace)
	}
	return filepath.Join(c.Workspace, filepath.FromSlash(c.StateDir))
}
