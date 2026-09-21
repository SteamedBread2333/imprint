package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func startTestServer(t *testing.T, dir string) *sdkmcp.ClientSession {
	t.Helper()
	v, err := imprint.Open(imprint.OpenOptions{Dir: dir, Now: func() time.Time {
		return time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatal(err)
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "imprint", Version: "test"}, nil)
	registerTools(server, v, nil)

	ct, st := sdkmcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "test"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *sdkmcp.ClientSession, name string, args any) string {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	res, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: m})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("tool %s error: %v", name, res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatalf("tool %s: empty content", name)
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("tool %s: expected text content", name)
	}
	return tc.Text
}

func TestParseArgs(t *testing.T) {
	cfg, err := ParseArgs([]string{"--project", "/tmp/proj", "--vault", ".imprint"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project != "/tmp/proj" || cfg.Vault != ".imprint" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	cfg, err = ParseArgs([]string{"--vault", "/tmp/v", "--global"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Vault != "/tmp/v" || !cfg.Global {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if _, err := ParseArgs([]string{"find"}); err == nil {
		t.Fatal("expected error for unknown arg")
	}
}

func TestToolFindAddGet(t *testing.T) {
	dir := t.TempDir()
	cs := startTestServer(t, dir)

	empty := callTool(t, cs, "find", map[string]any{"scope": "go"})
	if !strings.Contains(empty, "[]") {
		t.Fatalf("expected empty find: %s", empty)
	}

	added := callTool(t, cs, "add", map[string]any{
		"claim": "Use tabs in Go",
		"scope": "go,style",
		"text":  "always tabs",
	})
	var addRes imprint.AddResult
	if err := json.Unmarshal([]byte(added), &addRes); err != nil {
		t.Fatal(err)
	}
	if addRes.ID == "" {
		t.Fatalf("missing id: %s", added)
	}

	got := callTool(t, cs, "get", map[string]any{"id": addRes.ID, "full": true})
	if !strings.Contains(got, "Use tabs in Go") {
		t.Fatalf("get: %s", got)
	}

	hits := callTool(t, cs, "find", map[string]any{"scope": "go", "query": "tabs"})
	if !strings.Contains(hits, addRes.ID) {
		t.Fatalf("find: %s", hits)
	}
}

func TestToolAddQueryLocal(t *testing.T) {
	dir := t.TempDir()
	cs := startTestServer(t, dir)
	added := callTool(t, cs, "add", map[string]any{
		"claim":       "Documentation framing",
		"scope":       "docs",
		"text":        "user said",
		"query_local": "否定式堆砌 文档写作",
	})
	var addRes imprint.AddResult
	if err := json.Unmarshal([]byte(added), &addRes); err != nil {
		t.Fatal(err)
	}
	got := callTool(t, cs, "get", map[string]any{"id": addRes.ID, "full": true})
	if !strings.Contains(got, "否定式堆砌 文档写作") {
		t.Fatalf("get missing query_local: %s", got)
	}
}

func TestToolGetFoldsAndBoundsEvidence(t *testing.T) {
	dir := t.TempDir()
	cs := startTestServer(t, dir)
	added := callTool(t, cs, "add", map[string]any{
		"claim": "Keep evidence folded",
		"scope": "mcp",
		"text":  "initial private wording",
	})
	var addRes imprint.AddResult
	if err := json.Unmarshal([]byte(added), &addRes); err != nil {
		t.Fatal(err)
	}
	for _, evidence := range []string{"repeat one", "repeat two", "repeat three"} {
		callTool(t, cs, "reinforce", map[string]any{"id": addRes.ID, "evidence": evidence})
	}

	compact := callTool(t, cs, "get", map[string]any{"id": addRes.ID})
	if strings.Contains(compact, "initial private wording") || strings.Contains(compact, "repeat three") {
		t.Fatalf("default get leaked evidence: %s", compact)
	}
	if !strings.Contains(compact, `"evidence_count": 4`) {
		t.Fatalf("default get missing count: %s", compact)
	}

	withEvidence := callTool(t, cs, "get", map[string]any{
		"id": addRes.ID, "include_evidence": true, "evidence_limit": 2,
	})
	if !strings.Contains(withEvidence, "repeat three") || !strings.Contains(withEvidence, "repeat two") {
		t.Fatalf("bounded evidence missing newest entries: %s", withEvidence)
	}
	if strings.Contains(withEvidence, "repeat one") || strings.Contains(withEvidence, "initial private wording") {
		t.Fatalf("bounded evidence returned too many entries: %s", withEvidence)
	}
}

func TestToolsListed(t *testing.T) {
	dir := t.TempDir()
	cs := startTestServer(t, dir)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"find": true, "add": true, "reinforce": true, "supersede": true,
		"forget": true, "get": true, "list": true, "show": true, "sweep": true,
	}
	for _, tool := range tools.Tools {
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools: %v", want)
	}
}
