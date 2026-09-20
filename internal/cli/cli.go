package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// App is the imprint command-line interface.
type App struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Stdin   io.Reader
	Environ func(string) string
	Getwd   func() (string, error)
	Home    func() (string, error)
	Now     func() time.Time
}

// New returns an App bound to the process stdio and clock.
func New() *App {
	return &App{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Environ: os.Getenv,
		Getwd:   os.Getwd,
		Home:    os.UserHomeDir,
		Now:     time.Now,
	}
}

// Run is the process entrypoint used by cmd/imprint.
func Run(args []string) int {
	return New().Run(args)
}

func (a *App) out() io.Writer {
	if a.Stdout == nil {
		return os.Stdout
	}
	return a.Stdout
}

func (a *App) errw() io.Writer {
	if a.Stderr == nil {
		return os.Stderr
	}
	return a.Stderr
}

func (a *App) now() time.Time {
	if a.Now == nil {
		return time.Now()
	}
	return a.Now()
}

type globals struct {
	vault   string
	global  bool
	json    bool
	help    bool
	version bool
}

func parseGlobals(args []string) (g globals, cmd string, rest []string, err error) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			rest = args[i+1:]
			return g, cmd, rest, nil
		case a == "-h" || a == "--help":
			g.help = true
			i++
		case a == "--version" || a == "-V":
			g.version = true
			i++
		case a == "--json":
			g.json = true
			i++
		case a == "--global":
			g.global = true
			i++
		case a == "--vault":
			if i+1 >= len(args) {
				return g, cmd, rest, fmt.Errorf("--vault needs a path")
			}
			g.vault = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--vault="):
			g.vault = strings.TrimPrefix(a, "--vault=")
			i++
		case strings.HasPrefix(a, "-") && a != "-":
			if cmd == "" {
				return g, cmd, rest, fmt.Errorf("unknown flag %s", a)
			}
			rest = append(rest, a)
			i++
		default:
			if cmd == "" {
				cmd = a
				i++
				continue
			}
			rest = append(rest, a)
			i++
		}
	}
	return g, cmd, rest, nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) writeJSON(v any) error {
	enc := json.NewEncoder(a.out())
	enc.SetEscapeHTML(true)
	return enc.Encode(v)
}

func (a *App) fail(asJSON bool, err error) int {
	if err == nil {
		return 0
	}
	if asJSON {
		_ = a.writeJSON(imprint.ErrorBody{Error: err.Error()})
		return 1
	}
	c := newConsole(a.errw())
	c.blank()
	c.Error(err.Error())
	c.blank()
	return 1
}

func (a *App) openVault(g globals) (*imprint.Vault, error) {
	dir, err := imprint.ResolveDir(g.vault, g.global, a.Environ, a.Getwd, a.Home)
	if err != nil {
		return nil, err
	}
	return imprint.OpenWithNow(dir, a.now)
}

// Run dispatches a command. Returns a process exit code.
func (a *App) Run(args []string) int {
	g, cmd, rest, err := parseGlobals(args)
	if err != nil {
		return a.fail(false, err)
	}
	if g.version && cmd == "" {
		fmt.Fprintln(a.out(), "imprint", imprint.Version)
		return 0
	}
	if cmd == "" || cmd == "help" || g.help && cmd == "" {
		if cmd == "help" && len(rest) > 0 {
			fmt.Fprint(a.out(), commandHelp(rest[0]))
			return 0
		}
		fmt.Fprint(a.out(), rootHelp)
		return 0
	}
	if g.help {
		fmt.Fprint(a.out(), commandHelp(cmd))
		return 0
	}
	switch cmd {
	case "add":
		return a.cmdAdd(g, rest)
	case "find":
		return a.cmdFind(g, rest)
	case "reinforce":
		return a.cmdReinforce(g, rest)
	case "supersede":
		return a.cmdSupersede(g, rest)
	case "forget":
		return a.cmdForget(g, rest)
	case "list":
		return a.cmdList(g, rest)
	case "get":
		return a.cmdGet(g, rest)
	case "sweep":
		return a.cmdSweep(g, rest)
	case "show":
		return a.cmdShow(g, rest)
	case "init":
		return a.cmdInit(g, rest)
	case "export":
		return a.cmdExport(g, rest)
	case "migrate-shards":
		return a.cmdMigrateShards(g, rest)
	case "clear":
		return a.cmdClear(g, rest)
	case "up":
		return a.cmdUp(g, rest)
	case "down":
		return a.cmdDown(g, rest)
	case "host":
		return a.cmdHost(g, rest)
	case "status":
		return a.cmdStatus(g, rest)
	case "plugin":
		return a.cmdPlugin(g, rest)
	case "desk":
		return a.cmdDesk(g, rest)
	case "version":
		fmt.Fprintln(a.out(), "imprint", imprint.Version)
		return 0
	default:
		fmt.Fprintf(a.errw(), "imprint: unknown command %q (run imprint --help)\n", cmd)
		return 2
	}
}

const rootHelp = `imprint — agent long-term memory (SQLite vault)

Usage:
  imprint [global flags] <command> [flags]

Full reference: README.md (中文: README.zh.md)

Global flags:
  --vault PATH   Vault directory (default: ./.imprint/memory, or walk-up)
  --global       Use ~/.imprint
  --json         Machine-readable JSON on stdout

Environment:
  IMPRINT_VAULT  Default vault path when --vault is omitted

Everyday — local stack:
  up          Start vault API, shelves, and enabled plugins (then: desk open)
  down        Stop the local stack
  status      Snapshot of services (vault API, shelves, desk)
  desk open   Open desk in browser (does not start services)

Everyday — vault (no local stack required):
  init        Write agent rules for Cursor, Claude Code, Codex, Trae, Workbuddy
  find        Recall by scope and optional query
  add         Create a new rule
  reinforce   Strengthen an existing rule
  supersede   Replace a rule (marks the old one superseded)
  forget      Delete a rule permanently
  list        Short listing
  get         Full record by id
  show        User-facing listing
  sweep       Decay stale rules and mark low-confidence ones dormant
  export      Dump every record as JSON

Debug & advanced:
  host start | host stop | host serve   Split host control (prefer up/down)
  plugin list | enable | disable | start | stop   Split plugin control (prefer up/down)
  host serve  Foreground vault + shelves API (Ctrl+C)
  migrate-shards   Import legacy imprint-*.md into vault.db (one-off)
  clear       Delete every rule (requires --confirm --yes)
  version     Print version

Vault:
  .imprint/memory/vault.db   Rule store (gitignore via .imprint/)
  imprint desk open          Interactive graph (desk plugin)
`

func commandHelp(cmd string) string {
	switch cmd {
	case "add":
		return "Usage: imprint add CLAIM --scope tag,tag --text ORIG [--confidence 0.6] [--query-local TERMS]\n"
	case "find":
		return "Usage: imprint find [--scope tag,tag] [--query TEXT] [--query-local TERMS] [--top-k 5]\n"
	case "reinforce":
		return "Usage: imprint reinforce ID [--evidence TEXT] [--query-local TERMS]\n"
	case "supersede":
		return "Usage: imprint supersede OLD_ID --claim NEW --scope tag,tag [--reason TEXT] [--text ORIG] [--query-local TERMS]\n"
	case "forget":
		return "Usage: imprint forget ID\n"
	case "list":
		return "Usage: imprint list [--status active|dormant|superseded] [--scope tag,tag] [--query TEXT] [--min-confidence 0] [--since YYYY-MM-DD] [--limit N]\n"
	case "get":
		return "Usage: imprint get ID\n"
	case "sweep":
		return "Usage: imprint sweep [--decay-days 90] [--decay-amount 0.05] [--dormant-threshold 0.3]\n"
	case "show":
		return "Usage: imprint show [--limit N] [--format table|json]\n"
	case "init":
		return `Usage: imprint init [--force] [--cursor] [--claude] [--codex] [--trae] [--workbuddy]

Writes imprint memory rules for AI editors (default: all). Does not write MCP config.

  Cursor      .cursor/rules/imprint-memory.mdc
  Claude Code .claude/rules/imprint-memory.md
  Codex       AGENTS.md or AGENTS.override.md — ## imprint memory section
  Trae        .trae/rules/imprint-memory.md
  Workbuddy   .codebuddy/rules/imprint-memory/RULE.mdc

See docs/editors.md.
`
	case "export":
		return "Usage: imprint export\n"
	case "migrate-shards":
		return "Usage: imprint migrate-shards [--dry-run]\n\nImport legacy imprint-NNNN.md shards (and archive/) into vault.db. Markdown wins on id conflict; unmatched vault.db rules are kept.\n"
	case "clear":
		return "Usage: imprint clear --confirm --yes\n\nIrreversible. Both flags are required.\n"
	case "up":
		return "Usage: imprint up\n\nStarts host (vault API + shelves) and every enabled external plugin.\n"
	case "down":
		return "Usage: imprint down\n\nStops external plugins and the host.\n"
	case "status":
		return "Usage: imprint status\n\nRead-only snapshot of host, shelves, and external plugins.\n"
	case "host":
		return hostUsage
	case "plugin":
		return pluginUsage
	case "desk":
		return "Usage: imprint desk open\n\nOpens the desk UI in your browser. Requires imprint up first.\n"
	default:
		return rootHelp
	}
}
