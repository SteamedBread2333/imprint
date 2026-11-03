package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func (a *App) cmdEmbed(g globals, rest []string) int {
	if len(rest) == 0 {
		fmt.Fprint(a.out(), embedUsage)
		return 2
	}
	sub := rest[0]
	switch sub {
	case "prepare":
		return a.cmdEmbedPrepare(g)
	case "backfill":
		return a.cmdEmbedBackfill(g, rest[1:])
	default:
		fmt.Fprintf(a.errw(), "imprint embed: unknown subcommand %q\n", sub)
		fmt.Fprint(a.errw(), embedUsage)
		return 2
	}
}

const embedUsage = `Usage:
  imprint embed prepare
  imprint embed backfill [--limit N] [--force] [--dry-run]

prepare — download and load embed + relate weights in the foreground (Hugging
Face / tqdm on your terminal). No HTTP server. Run once after setup or when
the sidecar changes models. plugin start embed still warms on listen
but reuses the cache.

backfill — encodes every active+dormant rule whose claim has no vector for the
embed plugin's model. Use after enabling embed, after importing a vault, or
after a sidecar cold-restart that lost cache. --force re-encodes rules that
already have a vector (e.g. after a model upgrade). --dry-run reports the
scan without writing. The sidecar must be running. If it is down or the plugin
is off, the command prints a hint to run imprint up && imprint embed backfill.
`

func (a *App) cmdEmbedPrepare(g globals) int {
	pcfg, err := plugin.LoadEmbedManagerConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	if err := plugin.RunEmbedPrepare(context.Background(), pcfg); err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{"success": true}))
	}
	c := a.console()
	c.Heading("embed prepare")
	c.Done("models loaded (cache under ~/.cache/imprint/models)")
	c.blank()
	return 0
}

func (a *App) cmdEmbedBackfill(g globals, args []string) int {
	fs := newFlags()
	limit := fs.Int("limit", 0)
	force := fs.Bool("force", false)
	dryRun := fs.Bool("dry-run", false)
	if _, err := fs.parse(args); err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), embedUsage)
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	defer v.Close()
	res := v.BackfillEmbeddings(*limit, *force, *dryRun, 0)
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("embed backfill")
	if *dryRun {
		c.Done("dry run: %d rules would be encoded", res.Scanned)
		c.blank()
		return 0
	}
	c.Done("scanned %d  ·  encoded %d  ·  failed %d", res.Scanned, res.Encoded, res.Failed)
	printBackfillHint(c, res)
	c.blank()
	return 0
}

func printBackfillHint(c *console, res imprint.BackfillResult) {
	switch {
	case res.NoEmbedder:
		c.Action("embed is off; rules have no vectors until the sidecar is enabled and backfill runs",
			"set plugins.embed.enabled: true, then imprint up && imprint embed backfill")
	case res.Failed > 0:
		c.Action("sidecar missed some rules; vectors stay empty until backfill succeeds",
			"imprint up && imprint embed backfill")
		if len(res.FailedIDs) > 0 {
			c.Note("failed ids: %s", strings.Join(res.FailedIDs, ", "))
		}
	}
}
