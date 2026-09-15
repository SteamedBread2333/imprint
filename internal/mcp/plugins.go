package mcp

import (
	"context"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/plugin"
)

func registerPluginTools(server *sdkmcp.Server, cfgPath string) {
	cfg, err := plugin.Load(cfgPath)
	if err != nil {
		return
	}
	routes, err := plugin.EnabledTools(cfg)
	if err != nil {
		return
	}
	for _, route := range routes {
		r := route
		sdkmcp.AddTool(server, &sdkmcp.Tool{
			Name:        r.Registered,
			Description: r.Tool.Description,
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
	}
}
