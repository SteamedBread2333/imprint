package mcp

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestServeNDJSONTools(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.OpenWithNow(dir, func() time.Time {
		return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	in := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"imprint_add","arguments":{"claim":"Use snake_case","scope":["python","naming"],"text":"use snake_case"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"imprint_find","arguments":{"scope":["python","naming"]}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"imprint_list","arguments":{}}}`,
	}, "\n") + "\n")
	var out bytes.Buffer
	if err := Serve(v, in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses, got %d\n%s", len(lines), out.String())
	}
	var init rpcResponse
	if err := json.Unmarshal([]byte(lines[0]), &init); err != nil {
		t.Fatal(err)
	}
	if init.Error != nil {
		t.Fatalf("init error %+v", init.Error)
	}

	var listed rpcResponse
	if err := json.Unmarshal([]byte(lines[1]), &listed); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(listed.Result)
	if !bytes.Contains(raw, []byte("imprint_add")) || !bytes.Contains(raw, []byte("imprint_viz")) {
		t.Fatalf("tools missing: %s", raw)
	}

	var added rpcResponse
	if err := json.Unmarshal([]byte(lines[2]), &added); err != nil {
		t.Fatal(err)
	}
	var cr callResult
	b, _ := json.Marshal(added.Result)
	if err := json.Unmarshal(b, &cr); err != nil {
		t.Fatal(err)
	}
	if cr.IsError || !strings.Contains(cr.Content[0].Text, "r-2026-09-11-001") {
		t.Fatalf("add result %+v", cr)
	}

	var found rpcResponse
	if err := json.Unmarshal([]byte(lines[3]), &found); err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(found.Result)
	if err := json.Unmarshal(b, &cr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cr.Content[0].Text, "snake_case") {
		t.Fatalf("find %s", cr.Content[0].Text)
	}
}

func TestServeLSPFraming(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	msg := "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n" + string(payload)
	var out bytes.Buffer
	if err := Serve(v, strings.NewReader(msg), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Content-Length:") {
		t.Fatalf("expected LSP framing, got %q", out.String())
	}
	if !strings.Contains(out.String(), `"id":1`) {
		t.Fatalf("response %s", out.String())
	}
}
