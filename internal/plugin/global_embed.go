package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
	"gopkg.in/yaml.v3"
)

var legacyEmbedWarn sync.Once

// globalEmbedDoc is ~/.imprint/embed.yaml (same shape as plugins.embed).
type globalEmbedDoc struct {
	Enabled bool           `yaml:"enabled"`
	Package string         `yaml:"package"`
	Config  map[string]any `yaml:"config"`
}

// GlobalEmbedConfigPath returns ~/.imprint/embed.yaml.
func GlobalEmbedConfigPath() (string, error) {
	imprint.EnsureHomeEnv()
	h, err := imprint.UserHome(os.UserHomeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(h, imprint.ImprintDirName, "embed.yaml"), nil
}

// GlobalEmbedPluginStateDir is ~/.imprint/state/plugins/<id> (not vault-scoped).
func GlobalEmbedPluginStateDir(id string) (string, error) {
	imprint.EnsureHomeEnv()
	h, err := imprint.UserHome(os.UserHomeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(h, imprint.ImprintDirName, "state", imprint.PluginsStateDirName, id), nil
}

func pluginStateDir(cfg *Config, id string) (string, error) {
	if id == "embed" {
		return GlobalEmbedPluginStateDir(id)
	}
	if cfg == nil {
		return "", fmt.Errorf("plugin config missing")
	}
	vaultDir := cfg.ResolveVaultAbs()
	return filepath.Join(imprint.VaultStateDir(vaultDir), imprint.PluginsStateDirName, id), nil
}

// LoadGlobalEmbedEntry reads ~/.imprint/embed.yaml. Missing file → disabled entry.
func LoadGlobalEmbedEntry() (PluginEntry, string, bool, error) {
	path, err := GlobalEmbedConfigPath()
	if err != nil {
		return PluginEntry{}, "", false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PluginEntry{Enabled: false, Config: map[string]any{}}, path, false, nil
		}
		return PluginEntry{}, path, false, err
	}
	var doc globalEmbedDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return PluginEntry{}, path, true, fmt.Errorf("parse %s: %w", path, err)
	}
	entry := PluginEntry{
		Enabled: doc.Enabled,
		Package: strings.TrimSpace(doc.Package),
		Config:  doc.Config,
	}
	if entry.Config == nil {
		entry.Config = map[string]any{}
	}
	return entry, path, true, nil
}

// SaveGlobalEmbedEntry writes ~/.imprint/embed.yaml.
func SaveGlobalEmbedEntry(entry PluginEntry) error {
	path, err := GlobalEmbedConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	doc := globalEmbedDoc{
		Enabled: entry.Enabled,
		Package: strings.TrimSpace(entry.Package),
		Config:  entry.Config,
	}
	if doc.Config == nil {
		doc.Config = map[string]any{}
	}
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func warnLegacyProjectEmbed() {
	legacyEmbedWarn.Do(func() {
		path, err := GlobalEmbedConfigPath()
		if err != nil {
			path = "~/.imprint/embed.yaml"
		}
		fmt.Fprintf(os.Stderr,
			"imprint: plugins.embed in project imprint.yaml is ignored; configure embed in %s\n",
			path,
		)
	})
}

// MergeGlobalEmbed replaces in-memory plugins.embed with ~/.imprint/embed.yaml.
func MergeGlobalEmbed(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config missing")
	}
	if cfg.Plugins == nil {
		cfg.Plugins = map[string]PluginEntry{}
	}
	if _, ok := cfg.Plugins["embed"]; ok {
		warnLegacyProjectEmbed()
		delete(cfg.Plugins, "embed")
	}
	entry, path, _, err := LoadGlobalEmbedEntry()
	if err != nil {
		return err
	}
	cfg.Plugins["embed"] = entry
	// Embed sidecar reads IMPRINT_PLUGIN_CONFIG; point env at global file when starting embed.
	cfg.embedGlobalConfigPath = path
	return nil
}

// embedGlobalConfigPath on Config holds ~/.imprint/embed.yaml when merged.
func (c *Config) embedConfigFileForPlugin(id string) string {
	if id == "embed" && c.embedGlobalConfigPath != "" {
		return c.embedGlobalConfigPath
	}
	return c.filePath
}

// LoadEmbedManagerConfig builds a minimal config for embed prepare/start without a project yaml.
func LoadEmbedManagerConfig() (*Config, error) {
	entry, path, exists, err := LoadGlobalEmbedEntry()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("global embed config missing (%s)", path)
	}
	if !entry.Enabled {
		return nil, fmt.Errorf("embed is disabled in %s", path)
	}
	if strings.TrimSpace(entry.Package) == "" {
		return nil, fmt.Errorf("embed package path is empty in %s", path)
	}
	pkgDir := entry.Package
	if !filepath.IsAbs(pkgDir) {
		return nil, fmt.Errorf("embed package must be an absolute path in %s", path)
	}
	cfg := &Config{
		Host:      HostConfig{Listen: imprint.DefaultHostListen},
		Plugins:   map[string]PluginEntry{"embed": entry},
		filePath:  path,
		workspace: filepath.Clean(pkgDir),
	}
	cfg.embedGlobalConfigPath = path
	return cfg, nil
}

// ApplyGlobalEmbedToOpenOptions wires the embed HTTP client from ~/.imprint/embed.yaml.
func ApplyGlobalEmbedToOpenOptions(opts *imprint.OpenOptions) {
	if opts == nil {
		return
	}
	entry, _, _, err := LoadGlobalEmbedEntry()
	if err != nil || !entry.Enabled {
		return
	}
	applyEmbedEntryToOpenOptions(opts, entry)
}
