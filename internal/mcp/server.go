package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

const protocolVersion = "2024-11-05"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Serve speaks MCP JSON-RPC on in/out against vault.
func Serve(v *imprint.Vault, in io.Reader, out io.Writer) error {
	s := &server{vault: v, out: out, framing: "detect"}
	r := bufio.NewReader(in)
	for {
		body, framing, err := readMessage(r, s.framing)
		if s.framing == "detect" && framing != "" {
			s.framing = framing
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if herr := s.handle(body); herr != nil {
				return herr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

type server struct {
	vault   *imprint.Vault
	out     io.Writer
	framing string
}

func (s *server) handle(body []byte) error {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return s.write(rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
	}
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil
	}
	switch req.Method {
	case "initialize":
		return s.write(rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "imprint", "version": imprint.Version},
			},
		})
	case "ping":
		return s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
	case "tools/list":
		return s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": toolList()}})
	case "tools/call":
		var call toolCall
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &call); err != nil {
				return s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
			}
		}
		res := s.call(call.Name, call.Arguments)
		return s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
	default:
		return s.write(rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		})
	}
}

func (s *server) write(resp rpcResponse) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	switch s.framing {
	case "ndjson":
		_, err = s.out.Write(append(payload, '\n'))
		return err
	default:
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
		if _, err := io.WriteString(s.out, header); err != nil {
			return err
		}
		_, err = s.out.Write(payload)
		return err
	}
}

func readMessage(r *bufio.Reader, framing string) (body []byte, used string, err error) {
	if framing == "ndjson" {
		line, err := r.ReadBytes('\n')
		return line, "ndjson", err
	}
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, framing, err
		}
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if err := r.UnreadByte(); err != nil {
			return nil, framing, err
		}
		break
	}
	peek, err := r.Peek(1)
	if err != nil {
		return nil, framing, err
	}
	if framing != "lsp" && peek[0] == '{' {
		line, err := r.ReadBytes('\n')
		return line, "ndjson", err
	}
	headers := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, "lsp", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if ok {
			headers[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	n, err := strconv.Atoi(headers["content-length"])
	if err != nil {
		return nil, "lsp", fmt.Errorf("missing Content-Length")
	}
	body = make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, "lsp", err
	}
	return body, "lsp", nil
}

func toolResult(v any, err error) callResult {
	if err != nil {
		return callResult{
			Content: []textContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		}
	}
	raw, mErr := json.Marshal(v)
	if mErr != nil {
		return callResult{
			Content: []textContent{{Type: "text", Text: mErr.Error()}},
			IsError: true,
		}
	}
	return callResult{Content: []textContent{{Type: "text", Text: string(raw)}}}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func asFloat(v any, def float64) float64 {
	if v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return def
		}
		return f
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return def
		}
		return f
	default:
		return def
	}
}

func asInt(v any, def int) int {
	if v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, err := strconv.Atoi(t)
		if err != nil {
			return def
		}
		return n
	default:
		return def
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(t)
		return b
	default:
		return false
	}
}

func asStrings(v any) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case string:
		parts := strings.Split(t, ",")
		var out []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	case []any:
		var out []string
		for _, x := range t {
			s := asString(x)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func (s *server) call(name string, args map[string]any) callResult {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "imprint_add":
		res, err := s.vault.Add(asString(args["claim"]), asStrings(args["scope"]), asString(args["text"]), asFloat(args["confidence"], 0))
		return toolResult(res, err)
	case "imprint_find":
		res, err := s.vault.Find(asStrings(args["scope"]), asString(args["query"]), asInt(args["top_k"], 0))
		return toolResult(res, err)
	case "imprint_reinforce":
		res, err := s.vault.Reinforce(asString(args["id"]), asString(args["evidence"]))
		return toolResult(res, err)
	case "imprint_supersede":
		res, err := s.vault.Supersede(
			asString(args["old_id"]),
			asString(args["new_claim"]),
			asStrings(args["new_scope"]),
			asString(args["reason"]),
			asString(args["original_text"]),
		)
		return toolResult(res, err)
	case "imprint_forget":
		res, err := s.vault.Forget(asString(args["id"]))
		return toolResult(res, err)
	case "imprint_list":
		res, err := s.vault.List(asString(args["status"]), asInt(args["limit"], 0))
		return toolResult(res, err)
	case "imprint_get":
		res, err := s.vault.Get(asString(args["id"]))
		return toolResult(res, err)
	case "imprint_sweep":
		res, err := s.vault.Sweep(asInt(args["decay_days"], 0), asFloat(args["decay_amount"], 0), asFloat(args["dormant_threshold"], 0))
		return toolResult(res, err)
	case "imprint_show":
		res, err := s.vault.Show(asInt(args["limit"], 0))
		return toolResult(res, err)
	case "imprint_viz":
		res, err := s.vault.Viz(asString(args["out"]), asString(args["format"]), asBool(args["include_archived"]))
		return toolResult(res, err)
	default:
		return callResult{
			Content: []textContent{{Type: "text", Text: "unknown tool: " + name}},
			IsError: true,
		}
	}
}

func strArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
}

func toolList() []toolDef {
	return []toolDef{
		{
			Name:        "imprint_add",
			Description: "Add a new imprint rule. Use only after gatekeeper classification ADD. path is the shard file (imprint-NNNN.md), not a per-rule filename.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"claim":      map[string]any{"type": "string", "description": "Imperative rule statement"},
					"scope":      strArraySchema(),
					"text":       map[string]any{"type": "string", "description": "User's original words"},
					"confidence": map[string]any{"type": "number", "description": "Default 0.6"},
				},
				"required": []string{"claim", "scope", "text"},
			},
		},
		{
			Name:        "imprint_find",
			Description: "Recall imprints by scope (preferred) and optional lexical query.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope": strArraySchema(),
					"query": map[string]any{"type": "string"},
					"top_k": map[string]any{"type": "integer", "default": 5},
				},
			},
		},
		{
			Name:        "imprint_reinforce",
			Description: "Strengthen an existing rule: confidence +0.1 (cap 0.95).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":       map[string]any{"type": "string"},
					"evidence": map[string]any{"type": "string"},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "imprint_supersede",
			Description: "Replace an old rule. Archives the old record and writes a new one.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"old_id":        map[string]any{"type": "string"},
					"new_claim":     map[string]any{"type": "string"},
					"new_scope":     strArraySchema(),
					"reason":        map[string]any{"type": "string"},
					"original_text": map[string]any{"type": "string"},
				},
				"required": []string{"old_id", "new_claim", "new_scope"},
			},
		},
		{
			Name:        "imprint_forget",
			Description: "Permanently delete a rule.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string"}},
				"required":   []string{"id"},
			},
		},
		{
			Name:        "imprint_list",
			Description: "Short listing of rules.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{"type": "string", "enum": []string{"active", "dormant", "superseded"}},
					"limit":  map[string]any{"type": "integer"},
				},
			},
		},
		{
			Name:        "imprint_get",
			Description: "Full record by id, including evidence_log.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": map[string]any{"type": "string"}},
				"required":   []string{"id"},
			},
		},
		{
			Name:        "imprint_sweep",
			Description: "Decay untouched rules and archive those below the dormant threshold.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"decay_days":        map[string]any{"type": "integer", "default": 90},
					"decay_amount":      map[string]any{"type": "number", "default": 0.05},
					"dormant_threshold": map[string]any{"type": "number", "default": 0.3},
				},
			},
		},
		{
			Name:        "imprint_show",
			Description: "User-friendly full listing of imprints.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit":  map[string]any{"type": "integer"},
					"format": map[string]any{"type": "string", "enum": []string{"table", "json"}},
				},
			},
		},
		{
			Name:        "imprint_viz",
			Description: "Generate an HTML dashboard (default) or mermaid graph of the vault.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"out":              map[string]any{"type": "string"},
					"format":           map[string]any{"type": "string", "enum": []string{"html", "mermaid"}},
					"include_archived": map[string]any{"type": "boolean"},
				},
			},
		},
	}
}
