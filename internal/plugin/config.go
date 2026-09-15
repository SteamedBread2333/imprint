package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// HostConfig is the imprint plugin host section.
type HostConfig struct {
	Listen string `yaml:"listen"`
}

// PluginEntry is one plugin in plugins.yaml.
type PluginEntry struct {
	Enabled bool           `yaml:"enabled"`
	Package string         `yaml:"package"`
	Config  map[string]any `yaml:"config"`
}

// Config is the full .imprint/plugins.yaml file.
type Config struct {
	Vault    string                 `yaml:"vault"`
	Host     HostConfig             `yaml:"host"`
	Plugins  map[string]PluginEntry `yaml:"plugins"`
	filePath string
	workspace string
}

// DefaultHostListen is the vault read-only API address.
const DefaultHostListen = "127.0.0.1:9470"

// ResolveConfigPath walks up from cwd for .imprint/plugins.yaml.
func ResolveConfigPath(getwd func() (string, error)) (string, error) {
	if getwd == nil {
		getwd = os.Getwd
	}
	cwd, err := getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		cand := filepath.Join(dir, ".imprint", "plugins.yaml")
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(cwd, ".imprint", "plugins.yaml"), nil
}

// Load reads plugins.yaml from path. Missing file yields defaults.
func Load(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("plugins config path is empty")
	}
	cfg := &Config{
		Host: HostConfig{Listen: DefaultHostListen},
		Plugins: map[string]PluginEntry{},
		filePath: path,
	}
	workspace := filepath.Dir(filepath.Dir(path))
	cfg.workspace = workspace
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Host.Listen == "" {
		cfg.Host.Listen = DefaultHostListen
	}
	if cfg.Plugins == nil {
		cfg.Plugins = map[string]PluginEntry{}
	}
	return cfg, nil
}

// Save writes plugins.yaml atomically.
func (c *Config) Save() error {
	if c.filePath == "" {
		return fmt.Errorf("config has no file path")
	}
	if err := os.MkdirAll(filepath.Dir(c.filePath), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp := c.filePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.filePath)
}

// Workspace returns the project root (parent of .imprint).
func (c *Config) Workspace() string {
	if c.workspace != "" {
		return c.workspace
	}
	if c.filePath != "" {
		return filepath.Dir(filepath.Dir(c.filePath))
	}
	return "."
}

// HostURL returns http:// listen address for the vault API.
func (c *Config) HostURL() string {
	listen := c.Host.Listen
	if listen == "" {
		listen = DefaultHostListen
	}
	if strings.HasPrefix(listen, "http://") || strings.HasPrefix(listen, "https://") {
		return listen
	}
	return "http://" + listen
}

// PluginPort returns the configured port for a plugin id.
func PluginPort(entry PluginEntry, def int) int {
	if entry.Config == nil {
		return def
	}
	switch v := entry.Config["port"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return def
	}
}
