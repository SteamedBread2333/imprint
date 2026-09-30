package imprint

import (
	"os"
	"path/filepath"
)

// ResolveDir picks the vault directory (the folder that contains vault.db).
//
// Order: flagVault, walk-up imprint.yaml → global per-project vault,
// --global (cwd per-project vault), IMPRINT_VAULT, else cwd per-project vault.
func ResolveDir(flagVault string, global bool, getenv func(string) string, getwd func() (string, error), home func() (string, error)) (string, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if getwd == nil {
		getwd = os.Getwd
	}
	if home == nil {
		home = os.UserHomeDir
	}
	if flagVault != "" {
		if filepath.IsAbs(flagVault) {
			return filepath.Clean(flagVault), nil
		}
		cwd, err := getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, filepath.FromSlash(flagVault)), nil
	}
	root, found, err := FindProjectRoot(getwd)
	if err != nil {
		return "", err
	}
	if found {
		return ResolveVaultDirForWorkspace(root, "", home)
	}
	if global {
		cwd, err := getwd()
		if err != nil {
			return "", err
		}
		return ResolveVaultDirForWorkspace(cwd, "", home)
	}
	if env := getenv("IMPRINT_VAULT"); env != "" {
		return env, nil
	}
	cwd, err := getwd()
	if err != nil {
		return "", err
	}
	return ResolveVaultDirForWorkspace(cwd, "", home)
}
