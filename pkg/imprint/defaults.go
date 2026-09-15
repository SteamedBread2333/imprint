package imprint

import "fmt"

const (
	// LoopbackHost is the default bind address for host and plugin HTTP servers.
	LoopbackHost = "127.0.0.1"
	// DefaultHostPort is the vault read-only API port.
	DefaultHostPort = 9470
	// DefaultDeskPort is the imprint-desk plugin port.
	DefaultDeskPort = 4173
	// DefaultShelvesPort is the imprint-shelves plugin port.
	DefaultShelvesPort = 4174
	// DefaultPluginPort is the fallback port for unknown plugins.
	DefaultPluginPort = 4180
)

// DefaultHostListen is host serve listen address (host:port, no scheme).
const DefaultHostListen = LoopbackHost + ":9470"

// LocalURL returns http://127.0.0.1:port.
func LocalURL(port int) string {
	return fmt.Sprintf("http://%s:%d", LoopbackHost, port)
}

// DefaultPortForPlugin returns the conventional port for a plugin id.
func DefaultPortForPlugin(id string) int {
	switch id {
	case "desk":
		return DefaultDeskPort
	case "shelves":
		return DefaultShelvesPort
	default:
		return DefaultPluginPort
	}
}
