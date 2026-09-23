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
	if added.ID != "r-2026-09-11-001" || added.Confidence != 0.6 || filepath.Base(added.Path) != "vault.db" {
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
	if rf.Confidence != 0.6875 || rf.ReinforcementCount != 1 {
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

func TestCLIFindQueryLocal(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	code := app.Run([]string{
		"--json", "--vault", dir, "add",
		"Documentation framing policy",
		"--scope", "docs",
		"--text", "user text",
		"--query-local", "否定式堆砌 文档写作",
	})
	if code != 0 {
		t.Fatalf("add exit %d stderr=%s", code, errw)
	}
	out.Reset()
	code = app.Run([]string{
		"--json", "--vault", dir, "find",
		"--scope", "docs",
		"--query", "documentation framing",
		"--query-local", "否定式堆砌",
	})
	if code != 0 {
		t.Fatalf("find exit %d stderr=%s", code, errw)
	}
	var hits []imprint.FindHit
	if err := json.Unmarshal(out.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].QueryLocal != "否定式堆砌 文档写作" {
		t.Fatalf("find merged %+v", hits)
	}
}

func TestCLIForgetGetSweepShowClear(t *testing.T) {
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

func TestCLIHelpMentionsVaultDB(t *testing.T) {
	dir := t.TempDir()
	app, out, _ := testApp(t, dir)
	if code := app.Run([]string{"help"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "vault.db") || !strings.Contains(out.String(), "init") {
		t.Fatalf("help missing vault.db or init note:\n%s", out)
	}
}

func TestCLIInitWritesCursorRule(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "init", "--cursor"}); code != 0 {
		t.Fatalf("init exit %d stderr=%s stdout=%s", code, errw, out)
	}
	rulePath := filepath.Join(dir, ".cursor", "rules", "imprint-memory.mdc")
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "mcp.json")); !os.IsNotExist(err) {
		t.Fatal("init must not write mcp.json")
	}
	rule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rule), "alwaysApply") || !strings.Contains(string(rule), "imprint --json") {
		t.Fatalf("rule = %s", rule)
	}
	out.Reset()
	errw.Reset()
	if code := app.Run([]string{"init", "--cursor"}); code == 0 {
		t.Fatalf("expected refuse without --force, stdout=%s", out)
	}
	if code := app.Run([]string{"--json", "init", "--cursor", "--force"}); code != 0 {
		t.Fatalf("force init %d %s %s", code, errw, out)
	}
}

func TestCLIListFiltersJSON(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "--vault", dir, "add", "Use gofmt", "--scope", "go,style", "--text", "gofmt", "--confidence", "0.85"}); code != 0 {
		t.Fatalf("add %d %s", code, errw)
	}
	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "list", "--scope", "go", "--min-confidence", "0.85"}); code != 0 {
		t.Fatalf("list %d %s", code, errw)
	}
	var items []imprint.ListItem
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("list = %+v", items)
	}
}

func TestCLIVersion(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"version"}); code != 0 {
		t.Fatalf("version exit %d stderr=%s", code, errw)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "imprint ") || strings.TrimPrefix(got, "imprint ") == "" {
		t.Fatalf("version %q", got)
	}
}

func TestCLIExportJSONL(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--vault", dir, "add", "Use gofmt", "--scope", "go", "--text", "team policy"}); code != 0 {
		t.Fatalf("add exit %d: %s", code, errw)
	}
	out.Reset()
	if code := app.Run([]string{"--vault", dir, "export", "--format", "jsonl"}); code != 0 {
		t.Fatalf("export exit %d: %s", code, errw)
	}
	data, err := os.ReadFile(filepath.Join(dir, imprint.ExportDirName, "vault.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("jsonl lines = %d: %s", len(lines), data)
	}
	var rec imprint.Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Claim != "Use gofmt" {
		t.Fatalf("record = %+v", rec)
	}
}

func TestCLIReportJSON(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "--vault", dir, "add", "Prefer table tests", "--scope", "go", "--text", "team policy"}); code != 0 {
		t.Fatalf("add: %s", errw)
	}
	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "find", "--scope", "go", "--query", "table tests"}); code != 0 {
		t.Fatalf("find: %s", errw)
	}
	out.Reset()
	if code := app.Run([]string{"--json", "--vault", dir, "report", "--days", "30"}); code != 0 {
		t.Fatalf("report: %s", errw)
	}
	if !strings.Contains(out.String(), `"event_counts"`) || !strings.Contains(out.String(), `"recall_count"`) {
		t.Fatalf("report = %s", out)
	}
}

// `imprint add` in human mode must surface the similar advisory list when
// the embed sidecar reports polarity-conflicting paraphrases. Without this
// the user has to pass --json to discover the link — see Issue 4.
//
// Without an embedder (the testApp default) lexical Jaccard does not surface
// short claims as advisory, so we assert the human output's clean shape
// here. The embed-enabled path is covered by TestSimilarAdvisoryRendersIdScoreAndHint.
func TestCLIAddHumanOutputCleanShapeWithoutEmbed(t *testing.T) {
	dir := t.TempDir()
	app, out, _ := testApp(t, dir)
	if code := app.Run([]string{"--vault", dir, "add", "Always wrap errors with %w", "--scope", "go", "--text", "wrap"}); code != 0 {
		t.Fatalf("first add exit %d", code)
	}
	if !strings.Contains(out.String(), "added") {
		t.Fatalf("expected 'added' in human output, got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "similar") {
		t.Fatalf("no embedder → no advisory expected, got:\n%s", out.String())
	}
}
