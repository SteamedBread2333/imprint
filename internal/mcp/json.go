package mcp

import (
	"bytes"
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func jsonOK(v any) (*sdkmcp.CallToolResult, any, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return toolErr(err), nil, nil
	}
	text := buf.String()
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}, v, nil
}

func toolErr(err error) *sdkmcp.CallToolResult {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(imprint.ErrorBody{Error: err.Error()})
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: buf.String()}},
		IsError: true,
	}
}
