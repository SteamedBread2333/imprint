package mcp

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestFindToolSchemaIncludesQueryLocal(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(dir)
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
	defer cs.Close()

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "find" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		props, _ := m["properties"].(map[string]any)
		if props["query_local"] == nil || props["full"] == nil {
			t.Fatalf("find schema missing query_local or full: %s", string(raw))
		}
		return
	}
	t.Fatal("find tool not found")
}

func TestAddToolSchemaIncludesSources(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(dir)
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
	defer cs.Close()

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "add" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		s := string(raw)
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		props, _ := m["properties"].(map[string]any)
		if props["sources"] == nil {
			t.Fatalf("add schema missing sources: %s", s)
		}
		if props["query_local"] == nil {
			t.Fatalf("add schema missing query_local: %s", s)
		}
		return
	}
	t.Fatal("add tool not found")
}

func TestWriteAndGetToolSchemaPrompts(t *testing.T) {
	cs := startTestServer(t, t.TempDir())
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*sdkmcp.Tool{}
	for _, tool := range tools.Tools {
		tcopy := tool
		byName[tool.Name] = tcopy
	}
	need := map[string][]string{
		"supersede": {"query_local", "sources"},
		"reinforce": {"query_local"},
	}
	for name, fields := range need {
		tool := byName[name]
		if tool == nil {
			t.Fatalf("missing tool %s", name)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		props, _ := m["properties"].(map[string]any)
		for _, f := range fields {
			if props[f] == nil {
				t.Fatalf("%s schema missing %s: %s", name, f, string(raw))
			}
		}
	}
	get := byName["get"]
	if get == nil {
		t.Fatal("missing get tool")
	}
	raw, err := json.Marshal(get.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	props, _ := schema["properties"].(map[string]any)
	for _, field := range []string{"include_evidence", "evidence_limit", "full"} {
		if props[field] == nil {
			t.Fatalf("get schema missing %s: %s", field, raw)
		}
	}
}

func TestPATHBinaryFindSchemaIncludesQueryLocal(t *testing.T) {
	path, err := exec.LookPath("imprint-mcp")
	if err != nil {
		t.Skip("imprint-mcp not on PATH")
	}
	cmd := exec.Command(path, "--vault", t.TempDir())
	cmd.Stderr = io.Discard
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "test"}, nil)
	cs, err := client.Connect(context.Background(), &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "find" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		props, _ := m["properties"].(map[string]any)
		if props["query_local"] == nil {
			t.Fatalf("PATH imprint-mcp find schema missing query_local: %s", string(raw))
		}
		return
	}
	t.Fatal("find tool not found")
}
