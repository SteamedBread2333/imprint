package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolveConfigPathWalkUpIgnoresIMPRINTWorkspace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := imprint.ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "pkg", "foo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("IMPRINT_WORKSPACE", "/tmp/other-project")
	t.Cleanup(func() { os.Unsetenv("IMPRINT_WORKSPACE") })

	got, err := ResolveConfigPath(func() (string, error) { return sub, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgPath {
		t.Fatalf("ResolveConfigPath = %q want %q (must not follow IMPRINT_WORKSPACE)", got, cfgPath)
	}
}

func TestResolveConfigPathForVault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := imprint.ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ResolveConfigPathForVault(imprint.DefaultVaultDir(dir))
	if !ok || got != cfgPath {
		t.Fatalf("ResolveConfigPathForVault = %q, %v want %q, true", got, ok, cfgPath)
	}
}

func TestLoadDefaultsMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, imprint.ImprintDirName, imprint.ConfigFileName)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host.Listen != imprint.DefaultHostListen {
		t.Fatalf("listen = %q", cfg.Host.Listen)
	}
	if cfg.Vault != imprint.DefaultVaultRel() {
		t.Fatalf("vault = %q want %q", cfg.Vault, imprint.DefaultVaultRel())
	}
}

func TestLoadPluginEnabledFalse(t *testing.T) {
	dir := t.TempDir()
	path := imprint.ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `vault: .imprint/memory
plugins:
  shelves:
    enabled: false
    package: ../imprint-shelves-plugin
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugins["shelves"].Enabled {
		t.Fatal("shelves should be disabled")
	}
	routes, skipped := EnabledTools(cfg)
	if len(routes) != 0 {
		t.Fatalf("expected no plugin tools, got %d: %+v", len(routes), routes)
	}
	if len(skipped) != 0 {
		t.Fatalf("disabled plugin should not appear in skipped: %v", skipped)
	}
}

func TestPluginPort(t *testing.T) {
	if got := PluginPort(PluginEntry{}, imprint.DefaultDeskPort); got != imprint.DefaultDeskPort {
		t.Fatalf("empty config port = %d", got)
	}
	got := PluginPort(PluginEntry{Config: map[string]any{"port": float64(9001)}}, imprint.DefaultDeskPort)
	if got != 9001 {
		t.Fatalf("configured port = %d", got)
	}
}
