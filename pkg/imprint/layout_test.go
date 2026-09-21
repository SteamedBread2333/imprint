package imprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeVaultRel(t *testing.T) {
	if got := NormalizeVaultRel(".imprint/memory"); got != ".imprint" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeVaultRel(".imprint"); got != ".imprint" {
		t.Fatalf("got %q", got)
	}
}

func TestMigrateLegacyVaultDB(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, ".imprint", "memory")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldDB := filepath.Join(oldDir, "vault.db")
	if err := os.WriteFile(oldDB, []byte("sqlite-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyLayout(root); err != nil {
		t.Fatal(err)
	}
	newDB := filepath.Join(root, ".imprint", "vault.db")
	raw, err := os.ReadFile(newDB)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "sqlite-bytes" {
		t.Fatalf("migrated contents = %q", raw)
	}
	if _, err := os.Stat(oldDB); !os.IsNotExist(err) {
		t.Fatal("legacy vault.db should be gone")
	}
}

func TestMigrateLegacyShelvesDB(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, ".imprint", ".shelves", ".cache")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "index.db"), []byte("idx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyLayout(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".imprint", "state", "shelves.db"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "idx" {
		t.Fatalf("got %q", got)
	}
}
