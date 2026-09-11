package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func testApp(t *testing.T, dir string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	errw := &bytes.Buffer{}
	app := &App{
		Stdout:  out,
		Stderr:  errw,
		Environ: func(string) string { return "" },
		Getwd:   func() (string, error) { return dir, nil },
		Home:    func() (string, error) { return dir, nil },
		Now:     func() time.Time { return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC) },
	}
	return app, out, errw
}

func TestCLIAddFindReinforceJSON(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	code := app.Run([]string{"--json", "--vault", dir, "add", "Python function names must always be snake_case", "--scope", "python,naming", "--text", "use snake_case"})
	if code != 0 {
		t.Fatalf("add exit %d stderr=%s stdout=%s", code, errw, out)
	}
	var added imprint.AddResult
	if err := json.Unmarshal(out.Bytes(), &added); err != nil {
		t.Fatal(err)
	}
	if added.ID != "r-2026-09-11-001" || added.Confidence != 0.6 || filepath.Base(added.Path) != "imprint-0001.md" {
		t.Fatalf("add result %+v", added)
	}

	out.Reset()
	code = app.Run([]string{"--json", "--vault", dir, "find", "--scope", "python,naming"})
	if code != 0 {
		t.Fatalf("find exit %d %s", code, errw)
	}
	var hits []imprint.FindHit
	if err := json.Unmarshal(out.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title == "" || hits[0].Score <= 0 {
		t.Fatalf("find %+v", hits)
	}

	out.Reset()
	code = app.Run([]string{"--json", "--vault", dir, "reinforce", added.ID, "--evidence", "user confirmed again"})
	if code != 0 {
		t.Fatalf("reinforce %d %s", code, errw)
	}
	var rf imprint.ReinforceResult
	if err := json.Unmarshal(out.Bytes(), &rf); err != nil {
		t.Fatal(err)
	}
	if rf.Confidence != 0.7 || rf.ReinforcementCount != 1 {
		t.Fatalf("reinforce %+v", rf)
	}

	out.Reset()
	code = app.Run([]string{"--json", "--vault", dir, "supersede", added.ID, "--claim", "All JS/Python functions must use snake_case", "--scope", "javascript,python,naming", "--reason", "extended to frontend"})
	if code != 0 {
		t.Fatalf("supersede %d %s", code, errw)
	}
	var sp imprint.SupersedeResult
	if err := json.Unmarshal(out.Bytes(), &sp); err != nil {
		t.Fatal(err)
	}
	if sp.SupersededOldID != added.ID || sp.ID == added.ID {
		t.Fatalf("supersede %+v", sp)
	}

	out.Reset()
	code = app.Run([]string{"--json", "--vault", dir, "list", "--status", "superseded"})
	if code != 0 {
		t.Fatal(errw.String())
	}
	var listed []imprint.ListItem
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != added.ID {
		t.Fatalf("list superseded %+v", listed)
	}
}

func TestCLIForgetGetSweepShowVizClear(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "--vault", dir, "add", "Use gofmt", "--scope", "go", "--text", "gofmt", "--confidence", "0.6"}); code != 0 {
		t.Fatal(errw.String())
	}
	var added imprint.AddResult
	_ = json.Unmarshal(out.Bytes(), &added)

	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "get", added.ID}); code != 0 {
		t.Fatal(errw.String())
	}
	var rec imprint.Record
	if err := json.Unmarshal(out.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Claim != "Use gofmt" {
		t.Fatalf("get %s", rec.Claim)
	}

	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "show", "--format", "json"}); code != 0 {
		t.Fatal(errw.String())
	}
	if !strings.Contains(out.String(), added.ID) {
		t.Fatalf("show %s", out)
	}

	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "sweep"}); code != 0 {
		t.Fatal(errw.String())
	}
	var sw imprint.SweepResult
	if err := json.Unmarshal(out.Bytes(), &sw); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	dash := filepath.Join(dir, "dashboard.html")
	if code := app.Run([]string{"--json", "--vault", dir, "viz", "--out", dash, "--include-archived"}); code != 0 {
		t.Fatal(errw.String())
	}
	var vz imprint.VizResult
	if err := json.Unmarshal(out.Bytes(), &vz); err != nil {
		t.Fatal(err)
	}
	if vz.RulesCount != 1 {
		t.Fatalf("viz %+v", vz)
	}
	if _, err := os.Stat(dash); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "clear"}); code != 1 {
		t.Fatalf("clear without confirm should fail, got %d %s", code, out)
	}
	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "clear", "--confirm", "--yes"}); code != 0 {
		t.Fatal(errw.String())
	}

	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "list"}); code != 0 {
		t.Fatal(errw.String())
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("expected empty list, got %s", out)
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	dir := t.TempDir()
	app, _, _ := testApp(t, dir)
	if code := app.Run([]string{"nope"}); code != 2 {
		t.Fatalf("exit %d", code)
	}
}

func TestCLIHelpMentionsShards(t *testing.T) {
	dir := t.TempDir()
	app, out, _ := testApp(t, dir)
	if code := app.Run([]string{"help"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "imprint-NNNN.md") || !strings.Contains(out.String(), "init") {
		t.Fatalf("help missing shard or init note:\n%s", out)
	}
}

func TestCLIInitWritesCursorFiles(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "init"}); code != 0 {
		t.Fatalf("init exit %d stderr=%s stdout=%s", code, errw, out)
	}
	mcpPath := filepath.Join(dir, ".cursor", "mcp.json")
	rulePath := filepath.Join(dir, ".cursor", "rules", "imprint-memory.mdc")
	raw, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"command": "imprint"`) || !strings.Contains(string(raw), "mcp") {
		t.Fatalf("mcp.json = %s", raw)
	}
	rule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rule), "alwaysApply") || !strings.Contains(string(rule), "imprint_find") {
		t.Fatalf("rule = %s", rule)
	}
	out.Reset()
	errw.Reset()
	if code := app.Run([]string{"init"}); code == 0 {
		t.Fatalf("expected refuse without --force, stdout=%s", out)
	}
	if code := app.Run([]string{"--json", "init", "--force", "--docker"}); code != 0 {
		t.Fatalf("force docker init %d %s %s", code, errw, out)
	}
	raw, err = os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"command": "docker"`) || !strings.Contains(string(raw), "ghcr.io") {
		t.Fatalf("docker mcp.json = %s", raw)
	}
}
