package cli

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/imprint-memory.mdc
var cursorRuleTemplate []byte

const defaultGHCRImage = "ghcr.io/steamedbread2333/imprint:latest"

type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type mcpConfig struct {
	MCPServers map[string]mcpServer `json:"mcpServers"`
}

func (a *App) cmdInit(g globals, args []string) int {
	fs := newFlags()
	force := fs.Bool("force", false)
	docker := fs.Bool("docker", false)
	image := fs.String("image", defaultGHCRImage)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("init"))
			return 0
		}
		return a.fail(g.json, err)
	}

	root, err := a.workdir()
	if err != nil {
		return a.fail(g.json, err)
	}
	vault := strings.TrimSpace(g.vault)
	if vault == "" {
		vault = "./memory"
	}

	mcpPath := filepath.Join(root, ".cursor", "mcp.json")
	rulePath := filepath.Join(root, ".cursor", "rules", "imprint-memory.mdc")

	server := mcpServer{
		Command: "imprint",
		Args:    []string{"--vault", vault, "mcp"},
		Env:     map[string]string{"IMPRINT_VAULT": vault},
	}
	if *docker {
		absVault := vault
		if !filepath.IsAbs(absVault) {
			absVault = filepath.Join(root, vault)
		}
		server = mcpServer{
			Command: "docker",
			Args: []string{
				"run", "-i", "--rm",
				"--volume", absVault + ":/memory",
				*image,
				"--vault", "/memory", "mcp",
			},
		}
	}

	if err := writeMCPConfig(mcpPath, server, *force); err != nil {
		return a.fail(g.json, err)
	}
	if err := writeCursorRule(rulePath, *force); err != nil {
		return a.fail(g.json, err)
	}

	written := []string{mcpPath, rulePath}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{
			"written": written,
			"command": server.Command,
			"args":    server.Args,
		}))
	}
	fmt.Fprintf(a.out(), "wrote %s\n", mcpPath)
	fmt.Fprintf(a.out(), "wrote %s\n", rulePath)
	fmt.Fprintln(a.out(), "Reload Cursor Settings → MCP, then confirm tools named imprint_find / imprint_add appear.")
	return 0
}

func (a *App) workdir() (string, error) {
	if a.Getwd != nil {
		return a.Getwd()
	}
	return os.Getwd()
}

func writeMCPConfig(path string, server mcpServer, force bool) error {
	cfg := mcpConfig{MCPServers: map[string]mcpServer{}}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			if !force {
				return fmt.Errorf("%s exists and is not valid JSON; pass --force to replace", path)
			}
			cfg = mcpConfig{MCPServers: map[string]mcpServer{}}
		}
		if cfg.MCPServers == nil {
			cfg.MCPServers = map[string]mcpServer{}
		}
		if _, exists := cfg.MCPServers["imprint"]; exists && !force {
			return fmt.Errorf("%s already has an imprint server; pass --force to overwrite that entry", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	cfg.MCPServers["imprint"] = server
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

func writeCursorRule(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists; pass --force to overwrite", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := bytes.TrimSpace(cursorRuleTemplate)
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
