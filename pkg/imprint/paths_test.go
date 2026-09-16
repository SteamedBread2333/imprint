package imprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRootFromVault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := DefaultVaultDir(dir)
	root, ok := FindProjectRootFromVault(vault)
	if !ok || root != dir {
		t.Fatalf("FindProjectRootFromVault(%q) = %q, %v want %q, true", vault, root, ok, dir)
	}
}
