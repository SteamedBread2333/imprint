package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolvePluginConfigPathIgnoresProcessCwd(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	for _, root := range []string{rootA, rootB} {
		cfgPath := imprint.ConfigPath(root)
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "vault: .imprint/memory\n"
		if root == rootB {
			body += "shelves:\n  enabled: true\n"
		}
		if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(imprint.DefaultVaultDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldGetwd := Getwd
	Getwd = func() (string, error) { return rootB, nil }
	t.Cleanup(func() { Getwd = oldGetwd })

	got := resolvePluginConfigPath(imprint.DefaultVaultDir(rootA))
	want := imprint.ConfigPath(rootA)
	if got != want {
		t.Fatalf("resolvePluginConfigPath = %q want %q (must not use cwd %q)", got, want, rootB)
	}
}

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
