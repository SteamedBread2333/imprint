package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolveConfigPathIMPRINTWorkspace(t *testing.T) {
	t.Setenv("IMPRINT_WORKSPACE", "/tmp/my-project")
	t.Cleanup(func() { os.Unsetenv("IMPRINT_WORKSPACE") })

	got, err := ResolveConfigPath(func() (string, error) { return "/elsewhere", nil })
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/my-project", imprint.ImprintDirName, imprint.ConfigFileName)
	if got != want {
		t.Fatalf("ResolveConfigPath = %q want %q", got, want)
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

func TestPluginPort(t *testing.T) {
	if got := PluginPort(PluginEntry{}, imprint.DefaultDeskPort); got != imprint.DefaultDeskPort {
		t.Fatalf("empty config port = %d", got)
	}
	got := PluginPort(PluginEntry{Config: map[string]any{"port": float64(9001)}}, imprint.DefaultDeskPort)
	if got != 9001 {
		t.Fatalf("configured port = %d", got)
	}
}
