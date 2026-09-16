package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/shelves"
)

type runtimeProbe struct {
	Port      int    `json:"port"`
	Listening bool   `json:"listening"`
	Vault     string `json:"vault,omitempty"`
	Rules     int    `json:"rules,omitempty"`
	Error     string `json:"error,omitempty"`
}

type statusReport struct {
	Project string         `json:"project"`
	Vault   string         `json:"vault"`
	Host    runtimeProbe   `json:"host"`
	Desk    runtimeProbe   `json:"desk,omitempty"`
	Shelves shelves.Status `json:"shelves,omitempty"`
	Hint    string         `json:"hint,omitempty"`
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
