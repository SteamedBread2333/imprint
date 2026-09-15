package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
	"gopkg.in/yaml.v3"
)

// HostConfig is the imprint plugin host section.
type HostConfig struct {
	Listen string `yaml:"listen"`
}

// PluginEntry is one plugin in imprint.yaml.
type PluginEntry struct {
	Enabled bool           `yaml:"enabled"`
	Package string         `yaml:"package"`
	Config  map[string]any `yaml:"config"`
}

// Config is imprint.yaml under .imprint/ in the project.
type Config struct {
	Vault     string                 `yaml:"vault"`
	Host      HostConfig             `yaml:"host"`
	Plugins   map[string]PluginEntry `yaml:"plugins"`
	filePath  string
	workspace string
}

// ResolveConfigPath walks up from cwd for .imprint/imprint.yaml.
func ResolveConfigPath(getwd func() (string, error)) (string, error) {
	if getwd == nil {
		getwd = os.Getwd
	}
	cwd, err := getwd()
	if err != nil {
		return "", err
	}
	root, found, err := imprint.FindProjectRoot(getwd)
	if err != nil {
		return "", err
	}
	if found {
		return imprint.ConfigPath(root), nil
	}
	return imprint.ConfigPath(cwd), nil
}

// ResolveConfigPathForVault returns imprint.yaml for the project that owns vaultDir.
func ResolveConfigPathForVault(vaultDir string) (string, bool) {
	if root, ok := imprint.FindProjectRootFromVault(vaultDir); ok {
		return imprint.ConfigPath(root), true
	}
	return "", false
}

// Load reads imprint.yaml from path. Missing file yields defaults.
func Load(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("plugins config path is empty")
	}
	cfg := &Config{
		Host:     HostConfig{Listen: imprint.DefaultHostListen},
		Plugins:  map[string]PluginEntry{},
		filePath: path,
	}
	cfg.workspace = workspaceForConfig(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.Vault = defaultVaultRel(cfg.workspace)
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Host.Listen == "" {
		cfg.Host.Listen = imprint.DefaultHostListen
	}
	if cfg.Plugins == nil {
		cfg.Plugins = map[string]PluginEntry{}
	}
	if strings.TrimSpace(cfg.Vault) == "" {
		cfg.Vault = defaultVaultRel(cfg.workspace)
	}
	return cfg, nil
}

func defaultVaultRel(workspace string) string {
	_ = workspace
	return filepath.Join(imprint.ImprintDirName, imprint.VaultDirName)
}

// FileExists reports whether imprint.yaml was loaded from an on-disk file.
func (c *Config) FileExists() bool {
	if c.filePath == "" {
		return false
	}
	st, err := os.Stat(c.filePath)
	return err == nil && !st.IsDir()
}

// Save writes imprint.yaml atomically.
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

func workspaceForConfig(path string) string {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	if filepath.Base(dir) == imprint.ImprintDirName {
		return filepath.Dir(dir)
	}
	return dir
}

// Workspace returns the project repo root (parent of .imprint/).
func (c *Config) Workspace() string {
	if c.workspace != "" {
		return c.workspace
	}
	if c.filePath != "" {
		return workspaceForConfig(c.filePath)
	}
	return "."
}

// ResolveVaultAbs returns the absolute vault directory for this config.
func (c *Config) ResolveVaultAbs() string {
	v := strings.TrimSpace(c.Vault)
	if v == "" {
		v = defaultVaultRel(c.Workspace())
	}
	if filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(c.Workspace(), filepath.FromSlash(v))
}

// DefaultShelvesStateDir is .imprint/.shelves/.cache under the project root.
func DefaultShelvesStateDir(projectRoot string) string {
	return imprint.DefaultShelvesCacheDir(projectRoot)
}

// HostURL returns http:// listen address for the vault API.
func (c *Config) HostURL() string {
	listen := c.Host.Listen
	if listen == "" {
		listen = imprint.DefaultHostListen
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
