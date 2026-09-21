package imprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRootFromVault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := ConfigPath(dir)
	if err := os.WriteFile(cfgPath, []byte("host:\n  listen: 127.0.0.1:9470\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := DefaultVaultDir(dir)
	root, ok := FindProjectRootFromVault(vault)
	if !ok || root != dir {
		t.Fatalf("FindProjectRootFromVault(%q) = %q, %v want %q, true", vault, root, ok, dir)
	}
}

func TestConfigPathIsRepoRoot(t *testing.T) {
	dir := t.TempDir()
	got := ConfigPath(dir)
	want := filepath.Join(dir, "imprint.yaml")
	if got != want {
		t.Fatalf("ConfigPath = %q want %q", got, want)
	}
	if DefaultVaultDir(dir) != filepath.Join(dir, ".imprint") {
		t.Fatalf("DefaultVaultDir = %q", DefaultVaultDir(dir))
	}
	if DefaultStateDir(dir) != filepath.Join(dir, ".imprint", "state") {
		t.Fatalf("DefaultStateDir = %q", DefaultStateDir(dir))
	}
	if DefaultShelvesDB(dir) != filepath.Join(dir, ".imprint", "state", "shelves.db") {
		t.Fatalf("DefaultShelvesDB = %q", DefaultShelvesDB(dir))
	}
	if DefaultExportDir(dir) != filepath.Join(dir, ".imprint", "export") {
		t.Fatalf("DefaultExportDir = %q", DefaultExportDir(dir))
	}
}

func TestHasProjectMarkerSkipsImprintDir(t *testing.T) {
	dir := t.TempDir()
	imprintDir := ImprintDir(dir)
	if err := os.MkdirAll(filepath.Join(imprintDir, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, ok, err := FindProjectRoot(func() (string, error) {
		return filepath.Join(imprintDir, "memory"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || root != dir {
		t.Fatalf("FindProjectRoot = %q, %v want %q, true", root, ok, dir)
	}
}

func TestResolveConfigFilePrefersRoot(t *testing.T) {
	dir := t.TempDir()
	legacy := LegacyConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("vault: .imprint/memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveConfigFile(dir); got != legacy {
		t.Fatalf("legacy only: got %q want %q", got, legacy)
	}
	root := ConfigPath(dir)
	if err := os.WriteFile(root, []byte("host:\n  listen: 127.0.0.1:9470\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveConfigFile(dir); got != root {
		t.Fatalf("root wins: got %q want %q", got, root)
	}
}
