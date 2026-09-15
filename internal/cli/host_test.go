package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolveHostVault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := imprint.ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := plugin.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	yamlVault := cfg.ResolveVaultAbs()

	t.Run("explicit vault", func(t *testing.T) {
		app := &App{Environ: func(string) string { return "" }}
		got, err := app.resolveHostVault(globals{vault: "/explicit"}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got != "/explicit" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("env vault", func(t *testing.T) {
		app := &App{Environ: func(k string) string {
			if k == "IMPRINT_VAULT" {
				return "/env/vault"
			}
			return ""
		}}
		got, err := app.resolveHostVault(globals{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got != "/env/vault" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("yaml vault", func(t *testing.T) {
		app := &App{Environ: func(string) string { return "" }}
		got, err := app.resolveHostVault(globals{}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got != yamlVault {
			t.Fatalf("got %q want %q", got, yamlVault)
		}
	})
}
