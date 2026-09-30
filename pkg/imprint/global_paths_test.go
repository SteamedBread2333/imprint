package imprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectKeyStable(t *testing.T) {
	root := t.TempDir()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	k1, err := ProjectKey(abs)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ProjectKey(abs + string(os.PathSeparator))
	if err != nil {
		t.Fatal(err)
	}
	if k1 != k2 || len(k1) != 16 {
		t.Fatalf("keys = %q %q", k1, k2)
	}
}

func TestFindProjectRootFromGlobalVault(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	wabs, err := filepath.Abs(workspace)
	if err != nil {
		t.Fatal(err)
	}
	vaultDir, err := ProjectVaultDir(wabs, func() (string, error) { return home, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureProjectRootLink(vaultDir, wabs); err != nil {
		t.Fatal(err)
	}
	got, ok := FindProjectRootFromVault(vaultDir)
	if !ok || got != wabs {
		t.Fatalf("FindProjectRootFromVault = %q, %v want %q, true", got, ok, wabs)
	}
}
