package imprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ProjectRootFileName links a global vault directory back to the workspace.
	ProjectRootFileName = "project.root"
)

// ProjectKey returns the first 16 hex chars of SHA256(normalized workspace abs path).
func ProjectKey(workspace string) (string, error) {
	abs, err := normalizeWorkspacePath(workspace)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:])[:16], nil
}

func normalizeWorkspacePath(workspace string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(workspace))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// EnsureHomeEnv sets HOME when the MCP/CLI parent omitted it (some editors spawn with a minimal env).
func EnsureHomeEnv() {
	if strings.TrimSpace(os.Getenv("HOME")) != "" {
		return
	}
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		_ = os.Setenv("HOME", h)
	}
}

// UserHome returns the user home directory (UserHomeDir, then $HOME).
func UserHome(home func() (string, error)) (string, error) {
	if home == nil {
		home = os.UserHomeDir
	}
	h, err := home()
	if err == nil && strings.TrimSpace(h) != "" {
		return filepath.Clean(h), nil
	}
	if env := strings.TrimSpace(os.Getenv("HOME")); env != "" {
		return filepath.Clean(env), nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("home directory is empty")
}

// GlobalProjectsRoot returns ~/.imprint/projects.
func GlobalProjectsRoot(home func() (string, error)) (string, error) {
	h, err := UserHome(home)
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ImprintDirName, "projects"), nil
}

// ProjectVaultDir returns ~/.imprint/projects/<ProjectKey(workspace)>.
func ProjectVaultDir(workspace string, home func() (string, error)) (string, error) {
	key, err := ProjectKey(workspace)
	if err != nil {
		return "", err
	}
	projects, err := GlobalProjectsRoot(home)
	if err != nil {
		return "", err
	}
	return filepath.Join(projects, key), nil
}

// VaultStateDir returns <vaultDir>/state.
func VaultStateDir(vaultDir string) string {
	return filepath.Join(filepath.Clean(vaultDir), StateDirName)
}

// ProjectRootFile returns the path to project.root inside a vault directory.
func ProjectRootFile(vaultDir string) string {
	return filepath.Join(filepath.Clean(vaultDir), ProjectRootFileName)
}

// ReadLinkedProjectRoot reads workspace path from project.root.
func ReadLinkedProjectRoot(vaultDir string) (string, bool) {
	raw, err := os.ReadFile(ProjectRootFile(vaultDir))
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return "", false
	}
	abs, err := filepath.Abs(line)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

// EnsureProjectRootLink writes project.root when missing or stale.
func EnsureProjectRootLink(vaultDir, workspace string) error {
	abs, err := normalizeWorkspacePath(workspace)
	if err != nil {
		return err
	}
	vaultDir = filepath.Clean(vaultDir)
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		return err
	}
	if existing, ok := ReadLinkedProjectRoot(vaultDir); ok && existing == abs {
		return nil
	}
	tmp := ProjectRootFile(vaultDir) + ".tmp"
	if err := os.WriteFile(tmp, []byte(abs+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ProjectRootFile(vaultDir))
}

// ResolveVaultDirForWorkspace picks vault dir from optional yaml vault rel/abs or global default.
func ResolveVaultDirForWorkspace(workspace string, vaultFromYAML string, home func() (string, error)) (string, error) {
	absWorkspace, err := normalizeWorkspacePath(workspace)
	if err != nil {
		return "", err
	}
	v := NormalizeVaultRel(vaultFromYAML)
	if v != "" {
		if filepath.IsAbs(v) {
			dir := filepath.Clean(v)
			_ = MigrateLegacyLayout(dir)
			_ = EnsureProjectRootLink(dir, absWorkspace)
			return dir, nil
		}
		dir := filepath.Join(absWorkspace, filepath.FromSlash(v))
		_ = MigrateLegacyLayout(dir)
		_ = EnsureProjectRootLink(dir, absWorkspace)
		return dir, nil
	}
	dir, err := ProjectVaultDir(absWorkspace, home)
	if err != nil {
		return "", err
	}
	_ = MigrateLegacyLayout(dir)
	_ = EnsureProjectRootLink(dir, absWorkspace)
	return dir, nil
}
