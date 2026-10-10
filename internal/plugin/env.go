package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// pluginProcessEnv is the environment for sidecar subprocesses (start, prepare).
func pluginProcessEnv(cfg *Config, id string, entry PluginEntry, port int) []string {
	vaultDir := cfg.ResolveVaultAbs()
	pluginState := filepath.Join(imprint.VaultStateDir(vaultDir), imprint.PluginsStateDirName, id)
	_ = os.MkdirAll(pluginState, 0o755)
	base := append(os.Environ(),
		"PYTHONUNBUFFERED=1",
	)
	base = append(base, huggingFaceHubEnv()...)
	return append(base,
		fmt.Sprintf("IMPRINT_PLUGIN_PORT=%d", port),
		fmt.Sprintf("IMPRINT_HOST_URL=%s", cfg.HostURL()),
		fmt.Sprintf("IMPRINT_VAULT=%s", vaultDir),
		fmt.Sprintf("IMPRINT_WORKSPACE=%s", cfg.Workspace()),
		fmt.Sprintf("IMPRINT_PLUGIN_CONFIG=%s", cfg.filePath),
		fmt.Sprintf("IMPRINT_PLUGIN_STATE=%s", pluginState),
	)
}
