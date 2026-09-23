package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// resolveHostVault picks the vault directory for host operations.
func (a *App) resolveHostVault(g globals, pcfg *plugin.Config) (string, error) {
	if strings.TrimSpace(g.vault) != "" {
		return g.vault, nil
	}
	if g.global {
		return imprint.ResolveDir("", true, a.Environ, a.Getwd, a.Home)
	}
	if pcfg != nil && pcfg.FileExists() {
		return pcfg.ResolveVaultAbs(), nil
	}
	return imprint.ResolveDir("", false, a.Environ, a.Getwd, a.Home)
}

type hostRuntime struct {
	URL    string
	Vault  string
	Health host.HealthResponse
}

func (a *App) projectConfig(g globals) (*plugin.Config, string, error) {
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return nil, "", err
	}
	vaultDir, err := a.resolveHostVault(g, cfg)
	if err != nil {
		return nil, "", err
	}
	return cfg, vaultDir, nil
}

func (a *App) hostStart(g globals) (hostRuntime, error) {
	cfg, vaultDir, err := a.projectConfig(g)
	if err != nil {
		return hostRuntime{}, err
	}
	return a.ensureHostRunning(g, cfg, vaultDir)
}

func (a *App) hostStop(g globals) (string, error) {
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return "", err
	}
	listen := strings.TrimSpace(cfg.Host.Listen)
	if listen == "" {
		listen = imprint.DefaultHostListen
	}
	plugin.StopBackgroundHost(cfg)
	return listen, nil
}

func (a *App) ensureHostRunning(g globals, cfg *plugin.Config, vaultDir string) (hostRuntime, error) {
	hostURL := cfg.HostURL()
	listen := strings.TrimSpace(cfg.Host.Listen)
	if listen == "" {
		listen = imprint.DefaultHostListen
	}
	wantShelves := shelvesEnabled(cfg)

	if health, ok := fetchHostHealthFull(hostURL); ok {
		if vaultPathsEqual(health.Vault, vaultDir) && health.Shelves.Enabled == wantShelves {
			return hostRuntime{URL: hostURL, Vault: vaultDir, Health: health}, nil
		}
		if !g.json {
			c := a.console()
			if !vaultPathsEqual(health.Vault, vaultDir) {
				c.Action("switch host to this project's vault",
					fmt.Sprintf("imprint up   # was %s", health.Vault))
			} else if health.Shelves.Enabled != wantShelves {
				c.Action("restart host to apply shelves config",
					fmt.Sprintf("imprint up   # shelves %v → %v", health.Shelves.Enabled, wantShelves))
			}
		}
		_ = plugin.FreeListenPort(listen, 5*time.Second)
	}
	if err := a.startHostBackground(g, cfg, vaultDir); err != nil {
		return hostRuntime{}, err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if health, ok := fetchHostHealthFull(hostURL); ok &&
			vaultPathsEqual(health.Vault, vaultDir) &&
			health.Shelves.Enabled == wantShelves {
			return hostRuntime{URL: hostURL, Vault: vaultDir, Health: health}, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return hostRuntime{}, fmt.Errorf("host did not become healthy at %s", hostURL)
}

func (a *App) startHostBackground(g globals, cfg *plugin.Config, vaultDir string) error {
	plugin.StopBackgroundHost(cfg)

	listen := strings.TrimSpace(cfg.Host.Listen)
	if listen == "" {
		listen = imprint.DefaultHostListen
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"--vault", vaultDir, "host", "serve", "--listen", listen}
	if g.global {
		args = append([]string{"--global"}, args...)
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = cfg.Workspace()
	cmd.Env = os.Environ()
	nullOut, _ := os.Open(os.DevNull)
	cmd.Stdout = nullOut
	cmd.Stderr = nullOut
	plugin.DetachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start host: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func fetchHostHealth(hostURL string) (vault string, ok bool) {
	health, ok := fetchHostHealthFull(hostURL)
	return health.Vault, ok && health.OK
}

func fetchHostHealthFull(hostURL string) (host.HealthResponse, bool) {
	u := strings.TrimSuffix(hostURL, "/") + "/health"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return host.HealthResponse{}, false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return host.HealthResponse{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return host.HealthResponse{}, false
	}
	var h host.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return host.HealthResponse{}, false
	}
	return h, h.OK
}

func vaultPathsEqual(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	aa, errA := filepath.EvalSymlinks(a)
	bb, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		return aa == bb
	}
	return false
}

func shelvesEnabled(cfg *plugin.Config) bool {
	return shelves.ConfigFrom(cfg).Enabled
}

func configuredPluginIDs(cfg *plugin.Config) []string {
	if cfg == nil {
		return nil
	}
	var ids []string
	for id := range cfg.Plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func configuredPluginIDsExcept(cfg *plugin.Config, omit ...string) []string {
	if cfg == nil {
		return nil
	}
	skip := make(map[string]struct{}, len(omit))
	for _, id := range omit {
		skip[strings.TrimSpace(id)] = struct{}{}
	}
	var ids []string
	for id := range cfg.Plugins {
		if _, isOmitted := skip[id]; isOmitted {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
