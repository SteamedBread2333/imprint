package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// RunEmbedPrepare downloads and loads embed + relate models in the foreground.
// Hugging Face / tqdm progress goes to the caller's terminal via inherited stdout/stderr.
func RunEmbedPrepare(ctx context.Context, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("plugin config missing")
	}
	entry, ok := cfg.Plugins["embed"]
	if !ok || !entry.Enabled {
		return fmt.Errorf("embed plugin is not enabled in imprint.yaml")
	}
	pkgDir := PackageDir(cfg.Workspace(), entry.Package)
	if pkgDir == "" {
		return fmt.Errorf("embed: package path is empty")
	}
	port := PluginPort(entry, imprint.DefaultPortForPlugin("embed"))
	vaultDir := cfg.ResolveVaultAbs()
	pluginState := filepath.Join(imprint.VaultStateDir(vaultDir), imprint.PluginsStateDirName, "embed")
	return withEmbedFileLock(pluginState, func() error {
		cmd := exec.CommandContext(ctx, "sh", "-c", "./run.sh prepare")
		cmd.Dir = pkgDir
		cmd.Env = pluginProcessEnv(cfg, "embed", entry, port)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("embed prepare: %w", err)
		}
		return nil
	})
}
