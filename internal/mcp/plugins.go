package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/plugin"
)

func registerPluginTools(server *sdkmcp.Server, cfgPath string) {
	cfg, err := plugin.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "imprint-mcp: plugin config %s: %v\n", cfgPath, err)
		return
	}
	fmt.Fprintf(os.Stderr, "imprint-mcp: plugin config %s\n", cfgPath)
	routes, skipped := plugin.EnabledTools(cfg)
	if len(routes) == 0 {
		fmt.Fprintf(os.Stderr, "imprint-mcp: no plugin tools registered\n")
	}
	for _, msg := range skipped {
		fmt.Fprintf(os.Stderr, "imprint-mcp: %s\n", msg)
	}
	for _, route := range routes {
		if err := addPluginTool(server, route); err != nil {
			fmt.Fprintf(os.Stderr, "imprint-mcp: plugin %q tool %q skipped: %v\n",
				route.PluginID, route.Registered, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "imprint-mcp: registered plugin tool %q (%s)\n",
			route.Registered, route.PluginID)
	}
}

func addPluginTool(server *sdkmcp.Server, route plugin.ToolRoute) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	r := route
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        r.Registered,
		Description: r.Tool.Description,
		InputSchema: pluginToolInputSchema(r.Tool.InputSchema),
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, args json.RawMessage) (*sdkmcp.CallToolResult, any, error) {
		raw, err := plugin.CallTool(r, args)
		if err != nil {
			return toolErr(err), nil, nil
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return toolErr(err), nil, nil
		}
		return jsonOK(v)
	})
	return nil
}

// pluginToolInputSchema ensures MCP SDK gets a JSON Schema with type "object".
// json.RawMessage handlers do not infer a valid object schema on their own.
func pluginToolInputSchema(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	if typ, _ := raw["type"].(string); typ == "object" {
		return raw
	}
	out := make(map[string]any, len(raw)+1)
	for k, v := range raw {
		out[k] = v
	}
	out["type"] = "object"
	return out
}
