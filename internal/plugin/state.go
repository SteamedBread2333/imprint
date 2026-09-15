package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

type pluginPIDState map[string]int

func pluginStatePath(cfg *Config) string {
	return filepath.Join(cfg.Workspace(), imprint.ImprintDirName, ".plugins", "pids.json")
}

func loadPluginState(path string) (pluginPIDState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pluginPIDState{}, nil
		}
		return nil, err
	}
	var st pluginPIDState
	if err := json.Unmarshal(raw, &st); err != nil {
		return pluginPIDState{}, nil
	}
	if st == nil {
		st = pluginPIDState{}
	}
	return st, nil
}

func savePluginState(path string, st pluginPIDState) error {
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

func killAllFromState(path string) {
	st, err := loadPluginState(path)
	if err != nil || len(st) == 0 {
		return
	}
	for _, pid := range st {
		killPID(pid)
	}
	_ = os.Remove(path)
}

func setPluginPID(path, id string, pid int) error {
	st, err := loadPluginState(path)
	if err != nil {
		st = pluginPIDState{}
	}
	st[id] = pid
	return savePluginState(path, st)
}

func clearPluginPID(path, id string) {
	st, err := loadPluginState(path)
	if err != nil || len(st) == 0 {
		return
	}
	delete(st, id)
	if len(st) == 0 {
		_ = os.Remove(path)
		return
	}
	_ = savePluginState(path, st)
}

func killPID(pid int) {
	if pid <= 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
