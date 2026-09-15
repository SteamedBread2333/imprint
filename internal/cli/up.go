package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func (a *App) cmdUp(g globals, rest []string) int {
	fs := newFlags()
	noOpen := fs.Bool("no-open", false)
	if _, err := fs.parse(rest); err != nil {
		return a.fail(g.json, err)
	}

	cfg, err := a.loadPluginConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	vaultDir, err := a.resolveHostVault(g, cfg)
	if err != nil {
		return a.fail(g.json, err)
	}
	hostURL, err := a.ensureHostRunning(g, cfg, vaultDir)
	if err != nil {
		return a.fail(g.json, err)
	}

	mgr := plugin.NewManager(cfg)
	plugins, err := mgr.Reload(context.Background())
	if err != nil {
		return a.fail(g.json, err)
	}

	deskURL := ""
	if deskEntry, ok := cfg.Plugins["desk"]; ok && deskEntry.Enabled {
		deskURL, err = mgr.DeskURL()
		if err != nil {
			return a.fail(g.json, fmt.Errorf("desk: %w", err))
		}
	}

	if g.json {
		out := map[string]any{
			"host":    hostURL,
			"vault":   vaultDir,
			"plugins": plugins,
		}
		if deskURL != "" {
			out["desk"] = deskURL
		}
		return boolExit(a.writeJSON(out))
	}

	fmt.Fprintf(a.out(), "imprint up: host %s + plugins running in background (vault %s)\n", hostURL, vaultDir)
	for _, st := range plugins {
		if !st.Enabled {
			continue
		}
		line := fmt.Sprintf("  %s: ", st.ID)
		switch {
		case st.Healthy:
			line += "healthy"
		case st.Error != "":
			line += st.Error
		default:
			line += "not healthy"
		}
		if st.URL != "" {
			line += " @ " + st.URL
		}
		fmt.Fprintln(a.out(), line)
	}

	if deskURL == "" {
		fmt.Fprintln(a.out(), "desk: not enabled (imprint plugin enable desk)")
		return 0
	}
	if *noOpen {
		fmt.Fprintf(a.out(), "desk: %s (--no-open)\n", deskURL)
		fmt.Fprintln(a.out(), "logs: .imprint/.plugins/*.log — stop: imprint down")
		return 0
	}
	if err := openBrowser(deskURL); err != nil {
		fmt.Fprintf(a.out(), "open %s\n", deskURL)
		return a.fail(false, err)
	}
	fmt.Fprintf(a.out(), "opened %s\n", deskURL)
	fmt.Fprintln(a.out(), "logs: .imprint/.plugins/*.log — stop: imprint down")
	return 0
}

func (a *App) cmdDown(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint down\n")
		return 2
	}
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	hostListen := strings.TrimSpace(cfg.Host.Listen)
	if hostListen == "" {
		hostListen = imprint.DefaultHostListen
	}
	plugin.StopBackgroundHost(cfg)
	mgr := plugin.NewManager(cfg)
	mgr.StopAll()

	if g.json {
		return boolExit(a.writeJSON(map[string]any{
			"host":    hostListen,
			"plugins": pluginIDs(cfg),
		}))
	}
	fmt.Fprintf(a.out(), "imprint down: stopped host %s\n", hostListen)
	for _, id := range pluginIDs(cfg) {
		fmt.Fprintf(a.out(), "  %s: stopped\n", id)
	}
	return 0
}

func pluginIDs(cfg *plugin.Config) []string {
	var ids []string
	for id, entry := range cfg.Plugins {
		if entry.Enabled {
			ids = append(ids, id)
		}
	}
	return ids
}

func (a *App) ensureHostRunning(g globals, cfg *plugin.Config, vaultDir string) (string, error) {
	hostURL := cfg.HostURL()
	if v, ok := fetchHostHealth(hostURL); ok && vaultPathsEqual(v, vaultDir) {
		return hostURL, nil
	}
	if err := a.startHostBackground(g, cfg, vaultDir); err != nil {
		return "", err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if v, ok := fetchHostHealth(hostURL); ok && vaultPathsEqual(v, vaultDir) {
			return hostURL, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	logPath := hostLogPath(cfg)
	return "", fmt.Errorf("host did not become healthy at %s (see %s)", hostURL, logPath)
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
	plugin.DetachProcess(cmd)

	logPath := hostLogPath(cfg)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	ts := time.Now().Format(time.RFC3339)
	_, _ = fmt.Fprintf(logFile, "\n--- imprint host serve %s ---\n", ts)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start host: %w", err)
	}
	_ = logFile.Close()

	statePath := plugin.HostStatePath(cfg)
	st := plugin.HostRuntimeState{
		PID:    cmd.Process.Pid,
		Listen: listen,
		Vault:  vaultDir,
	}
	if err := plugin.SaveHostState(statePath, st); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("record host pid: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		plugin.ClearHostState(statePath)
	}()
	return nil
}

func fetchHostHealth(hostURL string) (vault string, ok bool) {
	u := strings.TrimSuffix(hostURL, "/") + "/health"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	var h host.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return "", false
	}
	return h.Vault, h.OK
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

func hostLogPath(cfg *plugin.Config) string {
	return filepath.Join(cfg.Workspace(), imprint.ImprintDirName, ".plugins", "host.log")
}
