package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// Status is one plugin row for list/reload.
type Status struct {
	ID       string `json:"id"`
	Enabled  bool   `json:"enabled"`
	Package  string `json:"package"`
	Port     int    `json:"port"`
	Running  bool   `json:"running"`
	Healthy  bool   `json:"healthy"`
	URL      string `json:"url,omitempty"`
	Error    string `json:"error,omitempty"`
	Name     string `json:"name,omitempty"`
	Version  string `json:"version,omitempty"`
	Manifest *Manifest `json:"-"`
}

// Manager starts and stops plugin processes.
type Manager struct {
	cfg     *Config
	mu      sync.Mutex
	procs   map[string]*exec.Cmd
	status  map[string]Status
}

// NewManager returns a manager for cfg.
func NewManager(cfg *Config) *Manager {
	return &Manager{
		cfg:    cfg,
		procs:  map[string]*exec.Cmd{},
		status: map[string]Status{},
	}
}

// List returns status for every configured plugin.
func (m *Manager) List() ([]Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Status
	for id, entry := range m.cfg.Plugins {
		st := m.statusFor(id, entry)
		out = append(out, st)
	}
	return out, nil
}

func (m *Manager) statusFor(id string, entry PluginEntry) Status {
	port := PluginPort(entry, imprint.DefaultPortForPlugin(id))
	st := Status{
		ID:      id,
		Enabled: entry.Enabled,
		Package: entry.Package,
		Port:    port,
		URL:     BaseURL(port),
	}
	if cmd, ok := m.procs[id]; ok && cmd.Process != nil {
		st.Running = true
	}
	pkgDir := PackageDir(m.cfg.Workspace(), entry.Package)
	if pkgDir != "" {
		if man, err := LoadManifest(pkgDir); err == nil {
			st.Name = man.Name
			st.Version = man.Version
			st.Manifest = man
		} else {
			st.Error = err.Error()
		}
	}
	if st.Running && st.Manifest != nil {
		st.Healthy = checkHealth(st.Manifest.Entry.Health, port)
	}
	return st
}

// Reload stops enabled plugins and starts them again.
func (m *Manager) Reload(ctx context.Context) ([]Status, error) {
	m.StopAll()
	for id, entry := range m.cfg.Plugins {
		if !entry.Enabled {
			continue
		}
		if err := m.startOne(ctx, id, entry); err != nil {
			m.mu.Lock()
			st := m.statusFor(id, entry)
			st.Error = err.Error()
			m.status[id] = st
			m.mu.Unlock()
		}
	}
	return m.List()
}

// StartPlugin starts one plugin by id.
func (m *Manager) StartPlugin(ctx context.Context, id string) error {
	entry, ok := m.cfg.Plugins[id]
	if !ok {
		return fmt.Errorf("unknown plugin %q", id)
	}
	entry.Enabled = true
	m.cfg.Plugins[id] = entry
	return m.startOne(ctx, id, entry)
}

func (m *Manager) startOne(ctx context.Context, id string, entry PluginEntry) error {
	pkgDir := PackageDir(m.cfg.Workspace(), entry.Package)
	if pkgDir == "" {
		return fmt.Errorf("plugin %q: package path is empty", id)
	}
	man, err := LoadManifest(pkgDir)
	if err != nil {
		return err
	}
	port := PluginPort(entry, imprint.DefaultPortForPlugin(id))
	start := strings.TrimSpace(man.Entry.Start)
	if start == "" {
		return fmt.Errorf("plugin %q: entry.start is empty", id)
	}
	parts := strings.Fields(start)
	if len(parts) == 0 {
		return fmt.Errorf("plugin %q: invalid start command", id)
	}

	m.mu.Lock()
	if old, ok := m.procs[id]; ok && old.Process != nil {
		_ = old.Process.Kill()
	}
	m.mu.Unlock()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = pkgDir
	vaultDir := m.cfg.ResolveVaultAbs()
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("IMPRINT_PLUGIN_PORT=%d", port),
		fmt.Sprintf("IMPRINT_HOST_URL=%s", m.cfg.HostURL()),
		fmt.Sprintf("IMPRINT_VAULT=%s", vaultDir),
		fmt.Sprintf("IMPRINT_WORKSPACE=%s", m.cfg.Workspace()),
		fmt.Sprintf("IMPRINT_PLUGIN_CONFIG=%s", m.cfg.filePath),
	)
	if id == "shelves" {
		stateDir := DefaultShelvesStateDir(m.cfg.Workspace())
		if entry.Config != nil {
			if s, ok := entry.Config["stateDir"].(string); ok && strings.TrimSpace(s) != "" {
				stateDir = strings.TrimSpace(s)
			}
		}
		cmd.Env = append(cmd.Env, "IMPRINT_SHELVES_STATE="+stateDir)
	}
	if id == "desk" {
		if shelvesEntry, ok := m.cfg.Plugins["shelves"]; ok && shelvesEntry.Enabled {
			port := PluginPort(shelvesEntry, imprint.DefaultShelvesPort)
			cmd.Env = append(cmd.Env, "IMPRINT_SHELVES_URL="+BaseURL(port))
		}
	}
	if roots, ok := entry.Config["roots"]; ok {
		if b, err := jsonRoots(roots); err == nil {
			cmd.Env = append(cmd.Env, "IMPRINT_SHELVES_ROOTS="+b)
		}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", id, err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if checkHealth(man.Entry.Health, port) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	m.mu.Lock()
	m.procs[id] = cmd
	st := m.statusFor(id, entry)
	m.status[id] = st
	m.mu.Unlock()
	go func(pid string, c *exec.Cmd) {
		_ = c.Wait()
		m.mu.Lock()
		delete(m.procs, pid)
		m.mu.Unlock()
	}(id, cmd)
	return nil
}

func jsonRoots(v any) (string, error) {
	switch t := v.(type) {
	case []any:
		var parts []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ","), nil
	case []string:
		return strings.Join(t, ","), nil
	default:
		return "", fmt.Errorf("unsupported roots type")
	}
}

// StopAll kills all managed plugin processes.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cmd := range m.procs {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		delete(m.procs, id)
	}
}

func checkHealth(healthTpl string, port int) bool {
	healthTpl = strings.TrimSpace(healthTpl)
	if healthTpl == "" {
		return false
	}
	parts := strings.Fields(healthTpl)
	if len(parts) < 2 {
		return false
	}
	method := strings.ToUpper(parts[0])
	url := strings.Join(parts[1:], " ")
	url = strings.ReplaceAll(url, "${port}", fmt.Sprintf("%d", port))
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// DeskURL returns the UI URL when desk plugin is enabled and healthy.
func (m *Manager) DeskURL() (string, error) {
	entry, ok := m.cfg.Plugins["desk"]
	if !ok || !entry.Enabled {
		return "", fmt.Errorf("desk plugin is not enabled")
	}
	port := PluginPort(entry, imprint.DefaultDeskPort)
	if !checkHealth("GET http://127.0.0.1:${port}/health", port) {
		return "", fmt.Errorf("desk plugin is not running on port %d", port)
	}
	return BaseURL(port), nil
}

// EnabledTools returns proxied tools from enabled plugins with resolved names.
func EnabledTools(cfg *Config) ([]ToolRoute, error) {
	taken := map[string]struct{}{}
	var routes []ToolRoute
	for id, entry := range cfg.Plugins {
		if !entry.Enabled {
			continue
		}
		pkgDir := PackageDir(cfg.Workspace(), entry.Package)
		man, err := LoadManifest(pkgDir)
		if err != nil {
			continue
		}
		hasTools := false
		for _, cap := range man.Capabilities {
			if cap == "tools" {
				hasTools = true
				break
			}
		}
		if !hasTools {
			continue
		}
		port := PluginPort(entry, imprint.DefaultPortForPlugin(id))
		base := BaseURL(port)
		resolved := ResolveTools(id, man.Tools, taken)
		for _, t := range resolved {
			routes = append(routes, ToolRoute{
				PluginID:    id,
				Tool:        t,
				BaseURL:     base,
				Registered:  t.RegisteredAs,
			})
		}
	}
	return routes, nil
}

// ToolRoute is one MCP tool proxied to a plugin HTTP handler.
type ToolRoute struct {
	PluginID   string
	Tool       ManifestTool
	BaseURL    string
	Registered string
}
