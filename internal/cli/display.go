package cli

import (
	"fmt"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
)

func formatRoots(roots []string) string {
	if len(roots) == 0 {
		return "docs"
	}
	return strings.Join(roots, ", ")
}

func printHostBlock(c *console, listening bool, hostURL string, rules int) {
	if !listening {
		port := hostURL
		if port == "" {
			port = "not listening"
		}
		c.Row("host", stateOff, "stopped", port)
		c.Action("start vault API and shelves", "imprint up")
		return
	}
	detail := hostURL
	switch rules {
	case 1:
		detail += " · 1 rule"
	case 0:
		detail += " · 0 rules"
	default:
		detail += fmt.Sprintf(" · %d rules", rules)
	}
	c.Row("host", stateOK, "running", detail)
}

func printShelvesBlock(c *console, st shelves.Status, roots []string, hostListening bool) {
	rootsLabel := formatRoots(roots)
	switch {
	case st.Enabled && st.Indexed:
		c.Row("shelves", stateOK, "ready",
			fmt.Sprintf("%d files · %d chunks · %s", st.FileCount, st.ChunkCount, rootsLabel))
	case st.Enabled && !hostListening:
		c.Row("shelves", stateWarn, "pending",
			fmt.Sprintf("will scan %s when host starts", rootsLabel))
	case st.Enabled:
		c.Row("shelves", stateWarn, "empty",
			fmt.Sprintf("0 chunks indexed under %s", rootsLabel))
		c.Action("docs were added or changed after host started — rebuild the index",
			"imprint down && imprint up")
	case st.ChunkCount > 0:
		c.Row("shelves", stateOff, "off",
			fmt.Sprintf("%d chunks cached (read-only)", st.ChunkCount))
		c.Action("turn shelves back on", "set shelves.enabled: true in .imprint/imprint.yaml, then imprint up")
	default:
		c.Row("shelves", stateOff, "off", "disabled in imprint.yaml")
	}
}

func printPluginBlock(c *console, id, name string, enabled, healthy bool, url, errMsg string) {
	label := id
	if name != "" && name != id {
		label = id + " · " + name
	}
	if !enabled {
		c.Row(label, stateOff, "off", "disabled in imprint.yaml")
		c.Action(fmt.Sprintf("enable the %s plugin", id),
			fmt.Sprintf("imprint plugin enable %s && imprint up", id))
		return
	}
	switch {
	case healthy:
		c.Row(label, stateOK, "running", url)
	case errMsg != "":
		c.Row(label, stateFail, "error", errMsg)
		c.Action("restart enabled plugins", "imprint up")
	default:
		c.Row(label, stateFail, "stopped", "not responding")
		c.Action("start enabled plugins", "imprint up")
	}
}

func printDeskBlock(c *console, port int, listening bool) {
	addr := fmt.Sprintf(":%d", port)
	if listening {
		c.Row("desk", stateOK, "running", addr)
		return
	}
	c.Row("desk", stateOff, "stopped", addr)
	c.Action("start desk with the local stack", "imprint up")
}

func deskPluginEnabled(cfg *plugin.Config) bool {
	if cfg == nil {
		return false
	}
	desk, ok := cfg.Plugins["desk"]
	return ok && desk.Enabled
}

func printUpNextSteps(c *console, deskEnabled, shelvesReady, hostListening bool) {
	switch {
	case !hostListening:
		return
	case !shelvesReady:
		return
	case deskEnabled:
		c.Action("open the graph UI in your browser", "imprint desk open")
	default:
		c.Action("check service health anytime", "imprint status")
	}
}

func statusHints(report statusReport, deskEnabled bool) (why, cmd string) {
	if report.Host.Listening && report.Host.Vault != "" && !vaultPathsEqual(report.Host.Vault, report.Vault) {
		return fmt.Sprintf("port :%d serves another project", report.Host.Port),
			fmt.Sprintf("cd %q && imprint up", report.Host.Vault)
	}
	if !report.Host.Listening ||
		(report.Shelves.Enabled && !report.Shelves.Indexed) ||
		(deskEnabled && report.Desk.Port > 0 && !report.Desk.Listening) {
		return "", ""
	}
	if deskEnabled && report.Desk.Listening {
		return "open the graph UI in your browser", "imprint desk open"
	}
	return "", ""
}
