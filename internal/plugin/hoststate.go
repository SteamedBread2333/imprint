package plugin

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// HostRuntimeState tracks a background host serve process for this project.
type HostRuntimeState struct {
	PID    int    `json:"pid"`
	Listen string `json:"listen"`
	Vault  string `json:"vault"`
}

// HostStatePath is the JSON file recording a background host serve PID.
func HostStatePath(cfg *Config) string {
	return filepath.Join(cfg.Workspace(), imprint.ImprintDirName, ".plugins", "host.json")
}

func loadHostState(path string) (HostRuntimeState, error) {
	var st HostRuntimeState
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return HostRuntimeState{}, nil
	}
	return st, nil
}

// SaveHostState writes host runtime metadata atomically.
func SaveHostState(path string, st HostRuntimeState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ClearHostState removes host runtime metadata.
func ClearHostState(path string) {
	_ = os.Remove(path)
}

// StopBackgroundHost kills the recorded host process and frees its listen port.
func StopBackgroundHost(cfg *Config) {
	if cfg == nil {
		return
	}
	statePath := HostStatePath(cfg)
	st, _ := loadHostState(statePath)
	if st.PID > 0 {
		killPID(st.PID)
	}
	listen := st.Listen
	if listen == "" {
		listen = cfg.Host.Listen
		if listen == "" {
			listen = imprint.DefaultHostListen
		}
	}
	if port := ParseListenPort(listen); port > 0 {
		_ = freePort(port, 2*time.Second)
	}
	ClearHostState(statePath)
}

// ParseListenPort extracts the TCP port from a host:port listen address.
func ParseListenPort(listen string) int {
	_, portStr, err := net.SplitHostPort(listen)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}
