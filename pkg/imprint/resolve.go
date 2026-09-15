package imprint

import (
	"os"
	"path/filepath"
)

// ResolveDir picks the vault directory.
//
// Order: flagVault, --global (~/.imprint), walk-up .imprint/memory,
// IMPRINT_VAULT, otherwise ./.imprint/memory under cwd.
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
		return flagVault, nil
	}
	if global {
		h, err := home()
		if err != nil {
			return "", err
		}
		return filepath.Join(h, ImprintDirName), nil
	}
	root, found, err := FindProjectRoot(getwd)
	if err != nil {
		return "", err
	}
	if found {
		return DefaultVaultDir(root), nil
	}
	if env := getenv("IMPRINT_VAULT"); env != "" {
		return env, nil
	}
	cwd, err := getwd()
	if err != nil {
		return "", err
	}
	return DefaultVaultDir(cwd), nil
}
