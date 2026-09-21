package imprint

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/sqlite"
)

func frozen(t *testing.T, dir string, at time.Time) *Vault {
	t.Helper()
	v, err := Open(OpenOptions{Dir: dir, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func day(offset int) time.Time {
	return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
}

func TestAddGetListForget(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.Add("Python function names must always be snake_case", []string{"python", "naming"}, "use snake_case", 0)
	if err != nil {
		t.Fatal(err)
	}
	if added.ID != "r-2026-09-11-001" {
		t.Fatalf("id = %s", added.ID)
	}
	if added.Confidence != 0.6 {
		t.Fatalf("confidence = %v", added.Confidence)
	}
	if filepath.Base(added.Path) != "vault.db" {
		t.Fatalf("path = %s", added.Path)
	}
	if _, err := os.Stat(added.Path); err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Claim != "Python function names must always be snake_case" {
		t.Fatalf("claim = %s", got.Claim)
	}
	if got.EvidenceLog[0].Text != "use snake_case" {
		t.Fatalf("evidence = %+v", got.EvidenceLog)
	}
	list, err := v.List("active", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != got.Claim {
		t.Fatalf("list = %+v", list)
	}
	second, err := v.Add("Use 4-space indents", []string{"python", "style"}, "tabs are forbidden", 0.85)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != "r-2026-09-11-002" {
		t.Fatalf("second id = %s", second.ID)
	}
	if _, err := v.Forget(added.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(added.ID); err == nil {
		t.Fatal("expected not found after forget")
	}
	still, err := v.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if still.Claim != "Use 4-space indents" {
		t.Fatalf("sibling lost after forget: %+v", still)
	}
}

func TestQueryScoreDoesNotBleed(t *testing.T) {
	rec := &Record{
		Claim:       "Prefer table-driven Go tests",
		EvidenceLog: []Evidence{{Text: "table-driven"}},
	}
	if s := queryScore(rec, "PascalCase"); s != 0 {
		t.Fatalf("score=%v", s)
	}
}

func TestFindScopeAndQuery(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.Add("Python function names must always be snake_case", []string{"python", "naming"}, "use snake_case", 0.8); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Go exported names use PascalCase", []string{"go", "naming"}, "PascalCase", 0.7); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Prefer table-driven Go tests", []string{"go", "testing"}, "table-driven", 0.6); err != nil {
		t.Fatal(err)
	}

	hits, err := v.Find([]string{"python", "naming"}, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "r-2026-09-11-001" {
		t.Fatalf("python naming hits = %+v", hits)
	}

	hits, err = v.Find([]string{"go"}, "PascalCase", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Title, "PascalCase") {
		t.Fatalf("go PascalCase hits = %+v", hits)
	}

	hits, err = v.Find(nil, "table-driven", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "r-2026-09-11-003" {
		t.Fatalf("query hits = %+v", hits)
	}
}

func TestFindSkipsLowConfidenceAndSuperseded(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	weak, err := v.Add("Maybe use tabs", []string{"python"}, "maybe", 0.25)
	if err != nil {
		t.Fatal(err)
	}
	old, err := v.Add("Use camelCase", []string{"python", "naming"}, "camel", 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Supersede(old.ID, "Use snake_case", []string{"python", "naming"}, "user correction", "snake_case"); err != nil {
		t.Fatal(err)
	}
	hits, err := v.Find([]string{"python"}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.ID == weak.ID || h.ID == old.ID {
			t.Fatalf("should not recall %s: %+v", h.ID, hits)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestReinforceCapsAndCount(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.Add("Always write tests", []string{"go", "testing"}, "always tests", 0.85)
	if err != nil {
		t.Fatal(err)
	}
	r1, err := v.Reinforce(added.ID, "user confirmed again")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Confidence != 0.875 || r1.ReinforcementCount != 1 {
		t.Fatalf("first reinforce = %+v", r1)
	}
	r2, err := v.Reinforce(added.ID, "said it once more")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Confidence != 0.89375 || r2.ReinforcementCount != 2 {
		t.Fatalf("capped reinforce = %+v", r2)
	}
}

func TestSupersedeArchivesAndLinks(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	old, err := v.Add("Backend Python uses snake_case", []string{"python", "naming"}, "snake_case", 0.7)
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Supersede(old.ID, "All JS/Python functions must use snake_case", []string{"javascript", "python", "naming"}, "extended to frontend", "frontend JS should use snake_case too")
	if err != nil {
		t.Fatal(err)
	}
	archived, err := v.Get(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != StatusSuperseded {
		t.Fatalf("status = %s", archived.Status)
	}
	fresh, err := v.Get(res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Supersedes) != 1 || fresh.Supersedes[0] != old.ID {
		t.Fatalf("supersedes = %v", fresh.Supersedes)
	}
}

func TestSweepDecayAndArchive(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(-100))
	added, err := v.Add("Old unused preference", []string{"general"}, "old", 0.32)
	if err != nil {
		t.Fatal(err)
	}
	v.now = func() time.Time { return day(0) }
	fresh, err := v.Add("Recent preference", []string{"general"}, "new", 0.7)
	if err != nil {
		t.Fatal(err)
	}
	res, err := v.Sweep(90, 0.05, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decayed != 1 || res.Archived != 1 {
		t.Fatalf("sweep = %+v", res)
	}
	old, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != StatusDormant || old.Confidence != 0.27 {
		t.Fatalf("old = status %s conf %v", old.Status, old.Confidence)
	}
	still, err := v.Get(fresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if still.Status != StatusActive || still.Confidence != 0.7 {
		t.Fatalf("fresh mutated: %+v", still)
	}
}

func TestExportAndClear(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.Add("Keep LICENSE as MIT", []string{"git"}, "MIT", 0.9); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := v.ExportJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var recs []Record
	if err := json.Unmarshal(buf.Bytes(), &recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("export count = %d", len(recs))
	}
	if err := v.Clear(); err != nil {
		t.Fatal(err)
	}
	list, err := v.List("", 0)
	if err != nil || len(list) != 0 {
		t.Fatalf("cleared list = %+v err=%v", list, err)
	}
}

func TestJSONContracts(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.Add("Use gofmt", []string{"go"}, "gofmt", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(added)
	if !bytes.Contains(raw, []byte(`"id"`)) || !bytes.Contains(raw, []byte(`"confidence"`)) {
		t.Fatalf("add json: %s", raw)
	}
}

func TestResolveDir(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj", "nested")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	imprintDir := filepath.Join(root, "proj", ".imprint")
	if err := os.MkdirAll(imprintDir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveDir("", false, func(string) string { return "" }, func() (string, error) { return proj, nil }, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got != imprintDir {
		t.Fatalf("walk-up got %s want %s", got, imprintDir)
	}
}

func TestGraphQueryLocal(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.AddRecord("Documentation framing", []string{"docs"}, "text", 0.85, nil, nil, nil, nil, "否定式堆砌"); err != nil {
		t.Fatal(err)
	}
	g, err := v.Graph(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 1 || g.Nodes[0].QueryLocal != "否定式堆砌" {
		t.Fatalf("graph node = %+v", g.Nodes)
	}
}

func TestGraph(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	old, err := v.Add("Use camelCase", []string{"js", "naming"}, "camel", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Supersede(old.ID, "Use snake_case in JS", []string{"js", "naming"}, "changed mind", "snake"); err != nil {
		t.Fatal(err)
	}
	g, err := v.Graph(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 2 || len(g.Edges) < 1 {
		t.Fatalf("graph = %+v", g)
	}
	active, err := v.Graph(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active.Nodes) != 2 || len(active.Edges) != 1 {
		t.Fatalf("active graph: nodes=%d edges=%d", len(active.Nodes), len(active.Edges))
	}
}

func TestForgetStripsInboundAndGetBacklinks(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	a, err := v.AddRecord("Keep related A", []string{"demo"}, "a", 0.6, nil, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.AddRecord("Keep related B", []string{"demo"}, "b", 0.6, nil, []string{a.ID}, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ReferencedBy) != 1 || got.ReferencedBy[0].ID != b.ID {
		t.Fatalf("backlinks = %+v", got.ReferencedBy)
	}
	if _, err := v.Forget(a.ID); err != nil {
		t.Fatal(err)
	}
	still, err := v.Get(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range still.Related {
		if id == a.ID {
			t.Fatalf("inbound related survived forget: %+v", still.Related)
		}
	}
}

func TestFindCJKAndIDF(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.Add("林小姐电话在前台登记", []string{"youti", "example"}, "林小姐电话", 0.6); err != nil {
		t.Fatal(err)
	}
	hits, err := v.Find(nil, "林小姐", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("cjk hits = %+v err=%v", hits, err)
	}
}

func TestListFilter(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.Add("Use gofmt", []string{"go", "style"}, "gofmt", 0.85); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Python function names must always be snake_case", []string{"python", "naming"}, "snake", 0.6); err != nil {
		t.Fatal(err)
	}
	items, err := v.ListFilter(ListFilter{Scope: []string{"go"}, MinConfidence: 0.85})
	if err != nil || len(items) != 1 || items[0].Title != "Use gofmt" {
		t.Fatalf("list filter = %+v err=%v", items, err)
	}
}

func TestAddWithSourcesAndSupersedeInherit(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.AddWithSources("Use Vitest", []string{"testing"}, "user said vitest", 0.85, []DocRef{
		{Path: "docs/testing.md", Heading: "Unit tests"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(added.ID)
	if err != nil || len(got.Sources) != 1 {
		t.Fatalf("sources = %+v err=%v", got.Sources, err)
	}
	replaced, err := v.Supersede(added.ID, "Use Vitest and Playwright", []string{"testing"}, "extended", "e2e too")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := v.Get(replaced.ID)
	if err != nil || len(fresh.Sources) != 1 {
		t.Fatalf("inherited sources = %+v err=%v", fresh.Sources, err)
	}
}

func TestFindMergedQueryLocal(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.AddRecord("Documentation framing policy", []string{"docs"}, "user text", 0.85, nil, nil, nil, nil, "否定式堆砌 文档写作")
	if err != nil {
		t.Fatal(err)
	}
	hits, err := v.FindMerged([]string{"docs"}, "documentation framing", "否定式堆砌", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != added.ID {
		t.Fatalf("find merged = %+v", hits)
	}
	if hits[0].QueryLocal != "否定式堆砌 文档写作" {
		t.Fatalf("query_local on hit = %q", hits[0].QueryLocal)
	}
}

func TestSupersedeInheritsQueryLocal(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.AddRecord("Policy A", []string{"docs"}, "text", 0.85, nil, nil, nil, nil, "本地检索词")
	if err != nil {
		t.Fatal(err)
	}
	replaced, err := v.Supersede(added.ID, "Policy B", []string{"docs"}, "update", "text")
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(replaced.ID)
	if err != nil || got.QueryLocal != "本地检索词" {
		t.Fatalf("inherited query_local = %q err=%v", got.QueryLocal, err)
	}
}

func TestVaultDBCreated(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(OpenOptions{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sqlite.DBPath(dir)); err != nil {
		t.Fatal(err)
	}
}
