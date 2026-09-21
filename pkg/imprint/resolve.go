package imprint

import (
	"os"
	"path/filepath"
)

// ResolveDir picks the vault directory (the folder that contains vault.db).
//
// Order: flagVault, --global (~/.imprint), walk-up .imprint,
// IMPRINT_VAULT, otherwise ./.imprint under cwd.
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
		root := filepath.Join(h, ImprintDirName)
		_ = MigrateLegacyLayout(h)
		return root, nil
	}
	root, found, err := FindProjectRoot(getwd)
	if err != nil {
		return "", err
	}
	if found {
		_ = MigrateLegacyLayout(root)
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
