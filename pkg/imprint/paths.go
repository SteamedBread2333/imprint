package imprint

import (
	"os"
	"path/filepath"
)

const (
	// ImprintDirName is the project-local imprint home (config + vault + caches).
	ImprintDirName = ".imprint"
	// VaultDirName is the default vault folder inside ImprintDirName.
	VaultDirName = "memory"
	// ConfigFileName is plugins + host config inside ImprintDirName.
	ConfigFileName = "imprint.yaml"
)

// ImprintDir returns repo/.imprint for a project root.
func ImprintDir(projectRoot string) string {
	return filepath.Join(projectRoot, ImprintDirName)
}

// DefaultVaultDir returns repo/.imprint/memory.
func DefaultVaultDir(projectRoot string) string {
	return filepath.Join(ImprintDir(projectRoot), VaultDirName)
}

// ConfigPath returns repo/.imprint/imprint.yaml.
func ConfigPath(projectRoot string) string {
	return filepath.Join(ImprintDir(projectRoot), ConfigFileName)
}

// DefaultShelvesCacheDir returns repo/.imprint/.shelves/.cache.
func DefaultShelvesCacheDir(projectRoot string) string {
	return filepath.Join(ImprintDir(projectRoot), ".shelves", ".cache")
}

// FindProjectRoot walks up from cwd for .imprint/imprint.yaml or .imprint/memory.
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
		if st, err := os.Stat(ConfigPath(dir)); err == nil && !st.IsDir() {
			return dir, true, nil
		}
		if st, err := os.Stat(DefaultVaultDir(dir)); err == nil && st.IsDir() {
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
