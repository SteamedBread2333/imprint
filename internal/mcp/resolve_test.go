package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolveVaultDirFromProject(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(imprint.DefaultVaultDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	oldGetwd := Getwd
	oldEnv := Env
	Getwd = func() (string, error) { return "/tmp/wrong-cwd", nil }
	Env = func(k string) string { return "" }
	t.Cleanup(func() { Getwd = oldGetwd; Env = oldEnv })

	got, err := resolveVaultDir(Config{Project: root})
	if err != nil {
		t.Fatal(err)
	}
	want := imprint.DefaultVaultDir(root)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveVaultDirFromProjectRelativeVault(t *testing.T) {
	root := t.TempDir()
	vaultRel := ".imprint/memory"
	vaultAbs := filepath.Join(root, vaultRel)
	if err := os.MkdirAll(vaultAbs, 0o755); err != nil {
		t.Fatal(err)
	}
	oldGetwd := Getwd
	Getwd = func() (string, error) { return "/tmp/wrong-cwd", nil }
	t.Cleanup(func() { Getwd = oldGetwd })

	got, err := resolveVaultDir(Config{Project: root, Vault: vaultRel})
	if err != nil {
		t.Fatal(err)
	}
	if got != vaultAbs {
		t.Fatalf("got %q want %q", got, vaultAbs)
	}
}

func TestResolvePluginConfigFromProjectIgnoresCwd(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	for _, root := range []string{rootA, rootB} {
		cfgPath := imprint.ConfigPath(root)
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cfgPath, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldGetwd := Getwd
	Getwd = func() (string, error) { return rootB, nil }
	t.Cleanup(func() { Getwd = oldGetwd })

	got := resolvePluginConfigPathForRun(Config{Project: rootA}, imprint.DefaultVaultDir(rootA))
	want := imprint.ConfigPath(rootA)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveVaultDirFromIMPRINTProjectEnv(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(imprint.DefaultVaultDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	oldEnv := Env
	Env = func(k string) string {
		if k == "IMPRINT_PROJECT" {
			return root
		}
		return ""
	}
	t.Cleanup(func() { Env = oldEnv })

	got, err := resolveVaultDir(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got != imprint.DefaultVaultDir(root) {
		t.Fatalf("got %q", got)
	}
}
