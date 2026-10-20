package plugin

import (
	"fmt"
	"os"
)

// pluginProcessEnv is the environment for sidecar subprocesses (start, prepare).
func pluginProcessEnv(cfg *Config, id string, entry PluginEntry, port int) []string {
	pluginState, err := pluginStateDir(cfg, id)
	if err != nil {
		pluginState = ""
	}
	if pluginState != "" {
		_ = os.MkdirAll(pluginState, 0o755)
	}
	vaultDir := ""
	if id != "embed" && cfg != nil {
		vaultDir = cfg.ResolveVaultAbs()
	}
	configFile := cfg.filePath
	if cfg != nil {
		configFile = cfg.embedConfigFileForPlugin(id)
	}
	base := append(os.Environ(),
		"PYTHONUNBUFFERED=1",
	)
	base = append(base, huggingFaceHubEnv()...)
	env := append(base,
		fmt.Sprintf("IMPRINT_PLUGIN_PORT=%d", port),
		fmt.Sprintf("IMPRINT_HOST_URL=%s", cfg.HostURL()),
		fmt.Sprintf("IMPRINT_WORKSPACE=%s", cfg.Workspace()),
		fmt.Sprintf("IMPRINT_PLUGIN_CONFIG=%s", configFile),
	)
	if vaultDir != "" {
		env = append(env, fmt.Sprintf("IMPRINT_VAULT=%s", vaultDir))
	}
	if pluginState != "" {
		env = append(env, fmt.Sprintf("IMPRINT_PLUGIN_STATE=%s", pluginState))
	}
	return env
}
