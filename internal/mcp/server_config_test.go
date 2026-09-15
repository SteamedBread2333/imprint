package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolvePluginConfigPathFromVault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := imprint.ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := imprint.DefaultVaultDir(dir)
	got := resolvePluginConfigPath(vault)
	if got != cfgPath {
		t.Fatalf("resolvePluginConfigPath = %q want %q", got, cfgPath)
	}
}
