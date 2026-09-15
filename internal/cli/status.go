package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

type runtimeProbe struct {
	Port      int    `json:"port"`
	Listening bool   `json:"listening"`
	Vault     string `json:"vault,omitempty"`
	Rules     int    `json:"rules,omitempty"`
	Error     string `json:"error,omitempty"`
}

type statusReport struct {
	Project string       `json:"project"`
	Vault   string       `json:"vault"`
	Host    runtimeProbe `json:"host"`
	Desk    runtimeProbe `json:"desk,omitempty"`
	Shelves runtimeProbe `json:"shelves,omitempty"`
	Hint    string       `json:"hint,omitempty"`
}

func (a *App) cmdStatus(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint status\n")
		return 2
	}
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	vaultDir, err := a.resolveHostVault(g, cfg)
	if err != nil {
		return a.fail(g.json, err)
	}

	hostPort := plugin.ParseListenPort(cfg.Host.Listen)
	if hostPort <= 0 {
		hostPort = plugin.ParseListenPort(imprint.DefaultHostListen)
	}
	hostURL := cfg.HostURL()

	report := statusReport{
		Project: cfg.Workspace(),
		Vault:   vaultDir,
		Host:    probeHost(hostPort, hostURL),
	}
	if desk, ok := cfg.Plugins["desk"]; ok && desk.Enabled {
		port := plugin.PluginPort(desk, imprint.DefaultDeskPort)
		report.Desk = probeDesk(port, hostURL)
	}
	if shelves, ok := cfg.Plugins["shelves"]; ok && shelves.Enabled {
		port := plugin.PluginPort(shelves, imprint.DefaultShelvesPort)
		report.Shelves = probePluginHealth(port)
	}

	if report.Host.Listening && report.Host.Vault != "" && !vaultPathsEqual(report.Host.Vault, vaultDir) {
		report.Hint = fmt.Sprintf(
			"host :%d is serving a different vault — run imprint up here, or cd to the project that owns %s",
			hostPort,
			report.Host.Vault,
		)
	} else if !report.Host.Listening {
		report.Hint = "nothing on host port — run: imprint up"
	} else if report.Desk.Port > 0 && !report.Desk.Listening {
		report.Hint = "host is up but desk is down — run: imprint up"
	}

	if g.json {
		return boolExit(a.writeJSON(report))
	}

	fmt.Fprintf(a.out(), "project: %s\n", report.Project)
	fmt.Fprintf(a.out(), "vault:   %s\n", report.Vault)
	printProbe(a, "host", report.Host)
	if report.Desk.Port > 0 {
		printProbe(a, "desk", report.Desk)
	}
	if report.Shelves.Port > 0 {
		printProbe(a, "shelves", report.Shelves)
	}
	if report.Hint != "" {
		fmt.Fprintf(a.out(), "\n→ %s\n", report.Hint)
	}
	return 0
}

func printProbe(a *App, label string, p runtimeProbe) {
	line := fmt.Sprintf("%-7s :%d", label, p.Port)
	if !p.Listening {
		line += "  (not running)"
	} else {
		if p.Vault != "" {
			line += "  vault " + p.Vault
		}
		if p.Rules > 0 {
			line += fmt.Sprintf("  (%d rules)", p.Rules)
		}
	}
	if p.Error != "" {
		line += "  [" + p.Error + "]"
	}
	fmt.Fprintln(a.out(), line)
}

func probeHost(port int, hostURL string) runtimeProbe {
	p := runtimeProbe{Port: port}
	vault, ok := fetchHostHealth(hostURL)
	if !ok {
		return p
	}
	p.Listening = true
	p.Vault = vault
	p.Rules = fetchRuleCount(hostURL)
	return p
}

func probeDesk(port int, hostURL string) runtimeProbe {
	p := probePluginHealth(port)
	if !p.Listening {
		return p
	}
	// Prefer host vault via desk proxy — reflects live graph source.
	u := fmt.Sprintf("http://127.0.0.1:%d/api/host/health", port)
	if vault, ok := fetchJSONVault(u); ok {
		p.Vault = vault
		p.Rules = fetchRuleCount(hostURL)
	}
	return p
}

func probePluginHealth(port int) runtimeProbe {
	p := runtimeProbe{Port: port}
	u := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return p
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return p
	}
	p.Listening = true
	var h struct {
		Vault string `json:"vault"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&h); err == nil && h.Vault != "" {
		p.Vault = h.Vault
	}
	return p
}

func fetchJSONVault(url string) (string, bool) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
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

func fetchRuleCount(hostURL string) int {
	u := strings.TrimSuffix(hostURL, "/") + "/graph"
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0
	}
	var g struct {
		Nodes []any `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return 0
	}
	return len(g.Nodes)
}
