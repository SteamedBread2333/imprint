package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/embed"
	"github.com/SteamedBread2333/imprint/internal/textseg"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
	"gopkg.in/yaml.v3"
)

// HostConfig is the vault HTTP server section in imprint.yaml.
type HostConfig struct {
	Listen string `yaml:"listen"`
}

// GlossaryConfig points at an optional domain word list for gse tokenization.
type GlossaryConfig struct {
	Path string `yaml:"path"`
}

type TelemetryConfig struct {
	Enabled       bool `yaml:"enabled"`
	RetentionDays int  `yaml:"retention_days"`
}

// SupersedeConfig tunes how the successor rule's confidence is derived.
type SupersedeConfig struct {
	InheritanceAlpha *float64 `yaml:"inheritance_alpha,omitempty"`
}

// Alpha is supersede inheritance_alpha in [0, 1]; nil uses DefaultInheritanceAlpha.
func (s SupersedeConfig) Alpha() float64 {
	if s.InheritanceAlpha == nil {
		return imprint.DefaultInheritanceAlpha
	}
	return imprint.ClampInheritanceAlpha(*s.InheritanceAlpha)
}

// ShelvesConfig is workspace doc indexing (host + MCP).
type ShelvesConfig struct {
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
	Defined bool           `yaml:"-"`
}

// UnmarshalYAML records whether shelves: was present in imprint.yaml.
func (s *ShelvesConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain struct {
		Enabled bool           `yaml:"enabled"`
		Config  map[string]any `yaml:"config"`
	}
	var p plain
	if err := value.Decode(&p); err != nil {
		return err
	}
	s.Enabled = p.Enabled
	s.Config = p.Config
	s.Defined = true
	return nil
}

// PluginEntry is one external plugin in imprint.yaml (e.g. desk).
type PluginEntry struct {
	Enabled bool           `yaml:"enabled"`
	Package string         `yaml:"package"`
	Config  map[string]any `yaml:"config"`
}

// Config is imprint.yaml at the project root (legacy: .imprint/imprint.yaml).
type Config struct {
	Vault     string                 `yaml:"vault"`
	Host      HostConfig             `yaml:"host"`
	Shelves   ShelvesConfig          `yaml:"shelves"`
	Glossary  GlossaryConfig         `yaml:"glossary"`
	Telemetry TelemetryConfig        `yaml:"telemetry"`
	Supersede SupersedeConfig        `yaml:"supersede"`
	Plugins   map[string]PluginEntry `yaml:"plugins"`
	filePath  string
	workspace string
}

// ResolveConfigPath walks up from cwd for imprint.yaml (root or legacy .imprint/).
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
		return imprint.ResolveConfigFile(root), nil
	}
	return imprint.ConfigPath(cwd), nil
}

// ResolveConfigPathForVault returns imprint.yaml for the project that owns vaultDir.
func ResolveConfigPathForVault(vaultDir string) (string, bool) {
	root, ok := imprint.FindProjectRootFromVault(vaultDir)
	if !ok {
		return "", false
	}
	p := imprint.ResolveConfigFile(root)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return imprint.ConfigPath(root), true
}

// Load reads imprint.yaml from path. Missing file yields defaults.
func Load(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("plugins config path is empty")
	}
	cfg := &Config{
		Host:      HostConfig{Listen: imprint.DefaultHostListen},
		Plugins:   map[string]PluginEntry{},
		Telemetry: TelemetryConfig{Enabled: true, RetentionDays: 30},
		filePath:  path,
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
	if cfg.Telemetry.RetentionDays <= 0 {
		cfg.Telemetry.RetentionDays = 30
	}
	normalizeLegacyShelves(cfg)
	cfg.Vault = imprint.NormalizeVaultRel(cfg.Vault)
	if strings.TrimSpace(cfg.Vault) == "" {
		cfg.Vault = defaultVaultRel(cfg.workspace)
	}
	return cfg, nil
}

// normalizeLegacyShelves moves plugins.shelves into top-level shelves: (in memory only).
// Top-level shelves wins when both are present.
func normalizeLegacyShelves(cfg *Config) {
	entry, ok := cfg.Plugins["shelves"]
	if !ok {
		return
	}
	delete(cfg.Plugins, "shelves")
	if cfg.Shelves.Defined {
		return
	}
	cfg.Shelves.Enabled = entry.Enabled
	if entry.Config != nil {
		cfg.Shelves.Config = entry.Config
	}
}

func defaultVaultRel(workspace string) string {
	_ = workspace
	return imprint.DefaultVaultRel()
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

// Workspace returns the project repo root (directory that contains imprint.yaml).
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
	v := imprint.NormalizeVaultRel(c.Vault)
	if v == "" {
		v = defaultVaultRel(c.Workspace())
	}
	if filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(c.Workspace(), filepath.FromSlash(v))
}

// DefaultShelvesStateDir is .imprint/state under the project root.
func DefaultShelvesStateDir(projectRoot string) string {
	return imprint.DefaultShelvesCacheDir(projectRoot)
}

// ApplyGlossary loads glossary.path when configured (relative to workspace).
func (c *Config) ApplyGlossary() error {
	path := strings.TrimSpace(c.Glossary.Path)
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.Workspace(), filepath.FromSlash(path))
	}
	return textseg.LoadGlossary(path)
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

// ApplyToOpenOptions copies yaml vault tunables onto Open.
// It also wires the optional embed sidecar when plugins.embed is enabled.
func (c *Config) ApplyToOpenOptions(opts *imprint.OpenOptions) {
	if c == nil || opts == nil {
		return
	}
	a := c.Supersede.Alpha()
	opts.InheritanceAlpha = &a
	entry, ok := c.Plugins["embed"]
	if !ok || !entry.Enabled {
		return
	}
	cfg := EmbedConfigFrom(entry)
	opts.Embed = embed.NewClient(cfg.Port, cfg.Model, time.Duration(cfg.TimeoutSeconds)*time.Second)
	if cfg.DuplicateThreshold > 0 {
		opts.EmbedDuplicateThreshold = cfg.DuplicateThreshold
	}
	opts.EmbedCrossScopePolicy = cfg.CrossScopePolicy
}

// EmbedConfig is the parsed plugins.embed.config section.
type EmbedConfig struct {
	Port               int
	Model              string
	TimeoutSeconds     int
	DuplicateThreshold float64
	// CrossScopePolicy controls whether the semantic duplicate gate runs
	// across all rules (advisory_only, default) or only across rules whose
	// scope overlaps the candidate (strict). advisory_only keeps the
	// cross-project recall signal — a Chinese rule in one project can still
	// surface as a paraphrase of an English rule in another — at the cost
	// of occasionally surfacing a foreign-scope advisory. strict trades that
	// recall for zero cross-scope noise.
	CrossScopePolicy string
}

// EmbedCrossScopePolicy values. Empty / unknown = default.
const (
	EmbedCrossScopeAdvisoryOnly = "advisory_only"
	EmbedCrossScopeStrict       = "strict"
)

// EmbedConfigFrom reads typed values from a plugin entry with defaults.
func EmbedConfigFrom(entry PluginEntry) EmbedConfig {
	cfg := EmbedConfig{
		Port:               imprint.DefaultEmbedPort,
		Model:              imprint.EmbedModel,
		TimeoutSeconds:     imprint.DefaultEmbedTimeoutSeconds,
		DuplicateThreshold: imprint.DefaultEmbedDuplicateThreshold,
		CrossScopePolicy:   EmbedCrossScopeAdvisoryOnly,
	}
	if entry.Config == nil {
		return cfg
	}
	if v, ok := entry.Config["port"].(int); ok && v > 0 {
		cfg.Port = v
	} else if v, ok := entry.Config["port"].(float64); ok && v > 0 {
		cfg.Port = int(v)
	}
	if v, ok := entry.Config["model"].(string); ok && strings.TrimSpace(v) != "" {
		cfg.Model = strings.TrimSpace(v)
	}
	if v, ok := entry.Config["timeout_seconds"].(int); ok && v > 0 {
		cfg.TimeoutSeconds = v
	} else if v, ok := entry.Config["timeout_seconds"].(float64); ok && v > 0 {
		cfg.TimeoutSeconds = int(v)
	}
	if v, ok := entry.Config["duplicate_threshold"].(float64); ok && v > 0 && v < 1 {
		cfg.DuplicateThreshold = v
	}
	if v, ok := entry.Config["cross_scope_policy"].(string); ok {
		v = strings.TrimSpace(v)
		if v == EmbedCrossScopeAdvisoryOnly || v == EmbedCrossScopeStrict {
			cfg.CrossScopePolicy = v
		}
	}
	return cfg
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
