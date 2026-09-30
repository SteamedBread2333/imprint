package imprint

import (
	"os"
	"path/filepath"
	"strings"
)

// NormalizeVaultRel maps the legacy .imprint/memory vault path to .imprint.
func NormalizeVaultRel(v string) string {
	v = strings.TrimSpace(filepath.ToSlash(v))
	v = strings.TrimSuffix(v, "/")
	if v == ImprintDirName+"/"+LegacyVaultDirName {
		return ImprintDirName
	}
	return v
}

// MigrateLegacyLayout moves pre-flatten files into the current layout under vaultDir.
// Missing sources are skipped; existing destinations are left untouched.
func MigrateLegacyLayout(vaultDir string) error {
	if strings.TrimSpace(vaultDir) == "" {
		return nil
	}
	root := filepath.Clean(vaultDir)
	if err := migrateLegacyVaultDB(root); err != nil {
		return err
	}
	if err := migrateLegacyShelvesDB(root); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(VaultStateDir(root), PluginsStateDirName), 0o755)
}

func migrateLegacyVaultDB(vaultDir string) error {
	dest := filepath.Join(vaultDir, VaultFileName)
	if fileExists(dest) {
		return nil
	}
	srcDir := filepath.Join(vaultDir, LegacyVaultDirName)
	src := filepath.Join(srcDir, VaultFileName)
	if !fileExists(src) {
		return nil
	}
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		return err
	}
	if err := moveSQLite(src, dest); err != nil {
		return err
	}
	_ = os.Remove(srcDir)
	return nil
}

func migrateLegacyShelvesDB(vaultDir string) error {
	destDir := VaultStateDir(vaultDir)
	dest := filepath.Join(destDir, ShelvesDBName)
	if fileExists(dest) {
		return nil
	}
	legacyDir := filepath.Join(vaultDir, ".shelves", ".cache")
	src := filepath.Join(legacyDir, "index.db")
	if !fileExists(src) {
		src = filepath.Join(legacyDir, ShelvesDBName)
	}
	if !fileExists(src) {
		return nil
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	if err := moveSQLite(src, dest); err != nil {
		return err
	}
	return nil
}

func moveSQLite(src, dest string) error {
	if err := moveFile(src, dest); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		s, d := src+suffix, dest+suffix
		if fileExists(s) {
			_ = moveFile(s, d)
		}
	}
	return nil
}

func moveFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		return err
	}
	return os.Remove(src)
}
