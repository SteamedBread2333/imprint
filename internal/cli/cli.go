package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/mcp"
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
	fmt.Fprintln(a.errw(), "imprint:", err.Error())
	if asJSON {
		_ = a.writeJSON(imprint.ErrorBody{Error: err.Error()})
	}
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
	case "viz":
		return a.cmdViz(g, rest)
	case "mcp":
		return a.cmdMCP(g, rest)
	case "init":
		return a.cmdInit(g, rest)
	case "export":
		return a.cmdExport(g, rest)
	case "clear":
		return a.cmdClear(g, rest)
	case "version":
		fmt.Fprintln(a.out(), "imprint", imprint.Version)
		return 0
	default:
		fmt.Fprintf(a.errw(), "imprint: unknown command %q\n", cmd)
		fmt.Fprint(a.errw(), rootHelp)
		return 2
	}
}

func (a *App) cmdMCP(g globals, _ []string) int {
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	in := a.Stdin
	if in == nil {
		in = os.Stdin
	}
	if err := mcp.Serve(v, in, a.out()); err != nil {
		return a.fail(g.json, err)
	}
	return 0
}

const rootHelp = `imprint — portable markdown memory for agents

Usage:
  imprint [global flags] <command> [flags]

Vault files:
  imprint-NNNN.md shards (new file at 32768 lines or 1 MiB)

Commands:
  add         Create a new rule
  find        Recall by scope and optional query
  reinforce   Strengthen an existing rule
  supersede   Replace a rule (archives the old one)
  forget      Delete a rule permanently
  list        Short listing
  get         Full record by id
  sweep       Decay stale rules and archive low-confidence ones
  show        User-facing listing
  viz         HTML dashboard or mermaid graph
  mcp         Speak MCP over stdio
  init        Write Cursor MCP config + alwaysApply rule
  export      Dump every record as JSON
  clear       Delete every rule (requires --confirm --yes)
  version     Print version

Global flags:
  --vault PATH   Vault directory (default: ./memory, or walk-up to memory/)
  --global       Use ~/.imprint
  --json         Machine-readable §21 JSON on stdout

Environment:
  IMPRINT_VAULT  Default vault path when --vault is omitted
`

func commandHelp(cmd string) string {
	switch cmd {
	case "add":
		return "Usage: imprint add CLAIM --scope tag,tag --text ORIG [--confidence 0.6]\n"
	case "find":
		return "Usage: imprint find [--scope tag,tag] [--query TEXT] [--top-k 5]\n"
	case "reinforce":
		return "Usage: imprint reinforce ID [--evidence TEXT]\n"
	case "supersede":
		return "Usage: imprint supersede OLD_ID --claim NEW --scope tag,tag [--reason TEXT] [--text ORIG]\n"
	case "forget":
		return "Usage: imprint forget ID\n"
	case "list":
		return "Usage: imprint list [--status active|dormant|superseded] [--limit N]\n"
	case "get":
		return "Usage: imprint get ID\n"
	case "sweep":
		return "Usage: imprint sweep [--decay-days 90] [--decay-amount 0.05] [--dormant-threshold 0.3]\n"
	case "show":
		return "Usage: imprint show [--limit N] [--format table|json]\n"
	case "viz":
		return "Usage: imprint viz [--out PATH] [--format html|mermaid] [--include-archived]\n"
	case "mcp":
		return "Usage: imprint mcp\n\nSpeaks the Model Context Protocol on stdin/stdout against the selected vault.\n"
	case "init":
		return "Usage: imprint init [--force] [--docker] [--image ghcr.io/steamedbread2333/imprint:latest]\n\nWrites .cursor/mcp.json and .cursor/rules/imprint-memory.mdc in the current project.\n"
	case "export":
		return "Usage: imprint export\n"
	case "clear":
		return "Usage: imprint clear --confirm --yes\n\nIrreversible. Both flags are required.\n"
	default:
		return rootHelp
	}
}
