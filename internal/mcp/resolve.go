package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// resolveVaultDir picks the vault directory for MCP startup.
// --project pins the repo root so vault and plugins bind to this repo even when
// the host spawns MCP with an unexpected cwd (multi-root workspaces).
func resolveVaultDir(cfg Config) (string, error) {
	if cfg.Global {
		return imprint.ResolveDir("", true, Env, Getwd, Home)
	}
	project := strings.TrimSpace(cfg.Project)
	if project == "" {
		project = strings.TrimSpace(Env("IMPRINT_PROJECT"))
	}
	if project != "" {
		root, err := filepath.Abs(project)
		if err != nil {
			return "", err
		}
		vaultFlag := strings.TrimSpace(cfg.Vault)
		if vaultFlag != "" {
			if filepath.IsAbs(vaultFlag) {
				dir := filepath.Clean(vaultFlag)
				_ = imprint.MigrateLegacyLayout(dir)
				_ = imprint.EnsureProjectRootLink(dir, root)
				return dir, nil
			}
			dir := filepath.Join(root, filepath.FromSlash(vaultFlag))
			_ = imprint.MigrateLegacyLayout(dir)
			_ = imprint.EnsureProjectRootLink(dir, root)
			return dir, nil
		}
		cfgPath := imprint.ResolveConfigFile(root)
		pcfg, err := plugin.Load(cfgPath)
		if err != nil {
			return "", err
		}
		dir := pcfg.ResolveVaultAbs()
		_ = imprint.EnsureProjectRootLink(dir, root)
		return dir, nil
	}
	return imprint.ResolveDir(cfg.Vault, false, Env, Getwd, Home)
}

// resolvePluginConfigPathForRun returns imprint.yaml for this MCP process.
func resolvePluginConfigPathForRun(cfg Config, vaultDir string) string {
	project := strings.TrimSpace(cfg.Project)
	if project == "" {
		project = strings.TrimSpace(Env("IMPRINT_PROJECT"))
	}
	if project != "" {
		root, err := filepath.Abs(project)
		if err != nil {
			return ""
		}
		path := imprint.ResolveConfigFile(root)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
		return ""
	}
	return resolvePluginConfigPath(vaultDir)
}

func logProjectRoot(cfg Config) {
	project := strings.TrimSpace(cfg.Project)
	if project == "" {
		project = strings.TrimSpace(Env("IMPRINT_PROJECT"))
	}
	if project == "" {
		return
	}
	root, err := filepath.Abs(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "imprint-mcp: project %s (invalid: %v)\n", project, err)
		return
	}
	fmt.Fprintf(os.Stderr, "imprint-mcp: project %s\n", root)
}
