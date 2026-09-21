package imprint

import (
	"os"
	"path/filepath"
)

const (
	// ImprintDirName is the gitignored private runtime directory.
	ImprintDirName = ".imprint"
	// ConfigFileName is team convention at the repo root (roots, plugin switches).
	ConfigFileName = "imprint.yaml"
	// VaultFileName is the SQLite vault inside ImprintDirName.
	VaultFileName = "vault.db"
	// StateDirName holds rebuildable derived data under ImprintDirName.
	StateDirName = "state"
	// ExportDirName holds optional md/json projections under ImprintDirName.
	ExportDirName = "export"
	// ShelvesDBName is the shelves SQLite file under StateDirName.
	ShelvesDBName = "shelves.db"
	// PluginsStateDirName is per-plugin derived state under StateDirName.
	PluginsStateDirName = "plugins"

	// LegacyVaultDirName is the pre-flatten vault folder (.imprint/memory/).
	LegacyVaultDirName = "memory"
)

// ImprintDir returns repo/.imprint for a project root.
func ImprintDir(projectRoot string) string {
	return filepath.Join(projectRoot, ImprintDirName)
}

// DefaultVaultDir returns repo/.imprint (directory that contains vault.db).
func DefaultVaultDir(projectRoot string) string {
	return ImprintDir(projectRoot)
}

// DefaultVaultRel is the vault directory relative to project root (for imprint.yaml).
func DefaultVaultRel() string {
	return ImprintDirName
}

// ConfigPath returns repo/imprint.yaml.
func ConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ConfigFileName)
}

// LegacyConfigPath returns repo/.imprint/imprint.yaml.
func LegacyConfigPath(projectRoot string) string {
	return filepath.Join(ImprintDir(projectRoot), ConfigFileName)
}

// ResolveConfigFile returns the on-disk imprint.yaml for a project, preferring
// the repo-root file and falling back to the legacy .imprint/ copy.
func ResolveConfigFile(projectRoot string) string {
	p := ConfigPath(projectRoot)
	if fileExists(p) {
		return p
	}
	legacy := LegacyConfigPath(projectRoot)
	if fileExists(legacy) {
		return legacy
	}
	return p
}

// DefaultStateDir returns repo/.imprint/state.
func DefaultStateDir(projectRoot string) string {
	return filepath.Join(ImprintDir(projectRoot), StateDirName)
}

// DefaultShelvesDB returns repo/.imprint/state/shelves.db.
func DefaultShelvesDB(projectRoot string) string {
	return filepath.Join(DefaultStateDir(projectRoot), ShelvesDBName)
}

// DefaultShelvesCacheDir is the shelves index directory (same as DefaultStateDir).
func DefaultShelvesCacheDir(projectRoot string) string {
	return DefaultStateDir(projectRoot)
}

// DefaultPluginsStateDir returns repo/.imprint/state/plugins.
func DefaultPluginsStateDir(projectRoot string) string {
	return filepath.Join(DefaultStateDir(projectRoot), PluginsStateDirName)
}

// PluginStateDir returns repo/.imprint/state/plugins/<id>.
func PluginStateDir(projectRoot, pluginID string) string {
	return filepath.Join(DefaultPluginsStateDir(projectRoot), pluginID)
}

// DefaultExportDir returns repo/.imprint/export.
func DefaultExportDir(projectRoot string) string {
	return filepath.Join(ImprintDir(projectRoot), ExportDirName)
}

// FindProjectRootFromVault walks up from vaultDir for imprint.yaml (root or legacy).
func FindProjectRootFromVault(vaultDir string) (string, bool) {
	dir := filepath.Clean(vaultDir)
	if filepath.Base(dir) == ImprintDirName {
		parent := filepath.Dir(dir)
		return parent, true
	}
	for {
		if hasProjectMarker(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// FindProjectRoot walks up from cwd for imprint.yaml or .imprint/.
func FindProjectRoot(getwd func() (string, error)) (string, bool, error) {
	if getwd == nil {
		getwd = os.Getwd
	}
	cwd, err := getwd()
	if err != nil {
		return "", false, err
	}
	dir := cwd
	for {
		if hasProjectMarker(dir) {
			return dir, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, false, nil
}

func hasProjectMarker(dir string) bool {
	if filepath.Base(dir) == ImprintDirName {
		return false
	}
	if fileExists(ConfigPath(dir)) || fileExists(LegacyConfigPath(dir)) {
		return true
	}
	if st, err := os.Stat(ImprintDir(dir)); err == nil && st.IsDir() {
		return true
	}
	return false
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
