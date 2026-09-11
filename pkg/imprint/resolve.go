package imprint

import (
	"os"
	"path/filepath"
)

// ResolveDir picks the vault directory.
//
// Order: flagVault, --global (~/.imprint), IMPRINT_VAULT, a parent memory/
// directory walking up from cwd, otherwise ./memory.
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
		return filepath.Join(h, ".imprint"), nil
	}
	if env := getenv("IMPRINT_VAULT"); env != "" {
		return env, nil
	}
	cwd, err := getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		cand := filepath.Join(dir, "memory")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(cwd, "memory"), nil
}
