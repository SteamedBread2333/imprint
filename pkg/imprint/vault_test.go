package imprint

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func frozen(t *testing.T, dir string, at time.Time) *Vault {
	t.Helper()
	v, err := OpenWithNow(dir, func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func day(offset int) time.Time {
	return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
}

func TestRecordRoundTrip(t *testing.T) {
	now := day(0)
	rec := &Record{
		ID:            "r-2026-09-11-001",
		Claim:         "Python function names must always be snake_case",
		Scope:         []string{"python", "naming"},
		Confidence:    0.6,
		Status:        StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastTouchedAt: now,
		EvidenceLog:   []Evidence{{At: now, Kind: EvidenceOriginal, Text: "use snake_case"}},
		Body:          "Keep it boring.",
	}
	raw, err := MarshalRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.HasPrefix(s, "---\n") || !strings.Contains(s, "claim:") {
		t.Fatalf("frontmatter missing:\n%s", s)
	}
	got, err := UnmarshalRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != rec.ID || got.Claim != rec.Claim || got.Body != rec.Body {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if len(got.Scope) != 2 || got.Scope[0] != "python" {
		t.Fatalf("scope: %v", got.Scope)
	}

	second := *rec
	second.ID = "r-2026-09-11-002"
	second.Claim = "Use 4-space indents"
	second.Body = ""
	packed, err := MarshalRecords([]*Record{rec, &second})
	if err != nil {
		t.Fatal(err)
	}
	many, err := UnmarshalRecords(packed)
	if err != nil {
		t.Fatal(err)
	}
	if len(many) != 2 || many[0].ID != rec.ID || many[1].ID != second.ID {
		t.Fatalf("packed round trip: %+v", many)
	}
	if many[0].Body != rec.Body {
		t.Fatalf("body lost in pack: %q", many[0].Body)
	}
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
	if added.Path != second.Path {
		t.Fatalf("expected one shard, got %s and %s", added.Path, second.Path)
	}
	if filepath.Base(added.Path) != "imprint-0001.md" {
		t.Fatalf("shard name = %s", added.Path)
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
		t.Fatalf("score=%v tokens=%v hayTokens=%v", s, tokenize("PascalCase"), tokenize(rec.Claim+" "+evidenceText(rec)))
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
	if hits[0].Score <= 0 {
		t.Fatalf("score should be positive: %+v", hits[0])
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

	hits, err = v.Find([]string{"rust"}, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no rust hits, got %+v", hits)
	}
}

func TestFindSkipsLowConfidenceAndSuperseded(t *testing.T) {
	dir := t.TempDir()
	now := day(0)
	v := frozen(t, dir, now)
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
	added, err := v.Add("Always write tests", []string{"go", "testing"}, "always tests", 0.9)
	if err != nil {
		t.Fatal(err)
	}
	r1, err := v.Reinforce(added.ID, "user confirmed again")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Confidence != 0.95 || r1.ReinforcementCount != 1 {
		t.Fatalf("first reinforce = %+v", r1)
	}
	r2, err := v.Reinforce(added.ID, "said it once more")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Confidence != 0.95 || r2.ReinforcementCount != 2 {
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
	if res.SupersededOldID != old.ID {
		t.Fatalf("%+v", res)
	}
	archived, err := v.Get(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != StatusSuperseded {
		t.Fatalf("status = %s", archived.Status)
	}
	if !strings.Contains(archived.Path, "archive") {
		t.Fatalf("expected archive path, got %s", archived.Path)
	}
	fresh, err := v.Get(res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Supersedes) != 1 || fresh.Supersedes[0] != old.ID {
		t.Fatalf("supersedes = %v", fresh.Supersedes)
	}
	if _, err := os.Stat(filepath.Join(dir, old.ID+".md")); !os.IsNotExist(err) {
		t.Fatalf("legacy file still in active: %v", err)
	}
	active, err := readRecords(filepath.Join(dir, "imprint-0001.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range active {
		if rec.ID == old.ID {
			t.Fatalf("old id still in active shard")
		}
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
	if res.Decayed != 1 {
		t.Fatalf("decayed = %+v", res)
	}
	if res.Archived != 1 {
		t.Fatalf("archived = %+v", res)
	}
	old, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != StatusDormant {
		t.Fatalf("status = %s conf=%v", old.Status, old.Confidence)
	}
	if old.Confidence != 0.27 {
		t.Fatalf("confidence = %v", old.Confidence)
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
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("cleared list = %+v", list)
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
	if !bytes.Contains(raw, []byte(`"id"`)) || !bytes.Contains(raw, []byte(`"confidence"`)) || !bytes.Contains(raw, []byte(`"path"`)) {
		t.Fatalf("add json: %s", raw)
	}
	hits, _ := v.Find([]string{"go"}, "", 5)
	raw, _ = json.Marshal(hits)
	if !bytes.Contains(raw, []byte(`"title"`)) || !bytes.Contains(raw, []byte(`"score"`)) {
		t.Fatalf("find json: %s", raw)
	}
}

func TestResolveDir(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj", "nested")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	mem := filepath.Join(root, "proj", "memory")
	if err := os.MkdirAll(mem, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveDir("", false, func(string) string { return "" }, func() (string, error) { return proj, nil }, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got != mem {
		t.Fatalf("walk-up got %s want %s", got, mem)
	}
	got, err = ResolveDir("/tmp/custom", true, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/custom" {
		t.Fatalf("flag should win: %s", got)
	}
}

func TestVizHTMLAndMermaid(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	old, err := v.Add("Use camelCase", []string{"js", "naming"}, "camel", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Supersede(old.ID, "Use snake_case in JS", []string{"js", "naming"}, "changed mind", "snake"); err != nil {
		t.Fatal(err)
	}
	htmlRes, err := v.Viz("", "html", true)
	if err != nil {
		t.Fatal(err)
	}
	if htmlRes.RulesCount != 2 || htmlRes.Path == "" {
		t.Fatalf("%+v", htmlRes)
	}
	body, err := os.ReadFile(htmlRes.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("dandelionLayout")) || !bytes.Contains(body, []byte(old.ID)) {
		t.Fatalf("dashboard missing graph data")
	}
	if !bytes.Contains(body, []byte(`id="helpPanel"`)) || !bytes.Contains(body, []byte(`id="recipe"`)) || !bytes.Contains(body, []byte(`id="legend"`)) {
		t.Fatalf("dashboard missing help, filter recipe, or legend")
	}
	if !bytes.Contains(body, []byte(`class="palette"`)) || !bytes.Contains(body, []byte("paintLegend")) {
		t.Fatalf("dashboard missing scope colour palette")
	}
	if bytes.Contains(body, []byte("unpkg.com")) || bytes.Contains(body, []byte("cytoscape(")) {
		t.Fatal("dashboard still references CDN or cytoscape")
	}
	m, err := v.Viz("", "mermaid", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Mermaid, "graph LR") || !strings.Contains(m.Mermaid, "supersedes") {
		t.Fatalf("mermaid = %s", m.Mermaid)
	}
}

func TestShardRollover(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	first, err := v.Add("First packed claim must be long enough", []string{"go"}, "one", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	v.MaxShardLines = bytes.Count(raw, []byte("\n"))
	if v.MaxShardLines < 1 {
		v.MaxShardLines = 1
	}
	second, err := v.Add("Second packed claim goes to the next shard", []string{"go"}, "two", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if second.Path == first.Path {
		t.Fatalf("expected rollover, both in %s", first.Path)
	}
	if filepath.Base(second.Path) != "imprint-0002.md" {
		t.Fatalf("second shard = %s", second.Path)
	}
	if _, err := v.Get(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(second.ID); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyCompactOnOpen(t *testing.T) {
	dir := t.TempDir()
	now := day(0)
	rec := &Record{
		ID:            "r-2026-09-11-001",
		Claim:         "Legacy one-file rule",
		Scope:         []string{"go"},
		Confidence:    0.6,
		Status:        StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastTouchedAt: now,
		EvidenceLog:   []Evidence{{At: now, Kind: EvidenceOriginal, Text: "legacy"}},
	}
	legacy := filepath.Join(dir, rec.ID+".md")
	if err := os.MkdirAll(filepath.Join(dir, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := MarshalRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0o644); err != nil {
		t.Fatal(err)
	}
	v := frozen(t, dir, now)
	got, err := v.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got.Path) != "imprint-0001.md" {
		t.Fatalf("compacted path = %s", got.Path)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be gone: %v", err)
	}
}

func TestForgetStripsInboundAndGetBacklinks(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	a, err := v.AddRecord("Keep related A", []string{"demo"}, "a", 0.6, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.AddRecord("Keep related B", []string{"demo"}, "b", 0.6, nil, []string{a.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ReferencedBy) != 1 || got.ReferencedBy[0].ID != b.ID || got.ReferencedBy[0].Kind != "related" {
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
	raw, err := os.ReadFile(still.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), a.ID) {
		t.Fatalf("forgotten id still in shard:\n%s", raw)
	}
}

func TestSeeAlsoRoundTrip(t *testing.T) {
	now := day(0)
	rec := &Record{
		ID:            "r-2026-09-11-001",
		Claim:         "Claim with a relative",
		Scope:         []string{"go"},
		Confidence:    0.6,
		Status:        StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastTouchedAt: now,
		Related:       []string{"r-2026-09-11-002"},
		Body:          "Keep it boring.",
		EvidenceLog:   []Evidence{{At: now, Kind: EvidenceOriginal, Text: "orig"}},
	}
	raw, err := MarshalRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "[[r-2026-09-11-002]]") || !strings.Contains(s, seeAlsoMark) {
		t.Fatalf("missing see-also:\n%s", s)
	}
	got, err := UnmarshalRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "Keep it boring." {
		t.Fatalf("body = %q", got.Body)
	}
	if !strings.Contains(string(raw), "Keep it boring.") {
		t.Fatal("user body dropped from marshal")
	}
}

func TestFindCJKAndIDF(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.Add("林小姐电话在前台登记", []string{"youti", "example"}, "林小姐电话", 0.6); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("周六还有预约空档", []string{"youti", "example"}, "预约", 0.6); err != nil {
		t.Fatal(err)
	}
	hits, err := v.Find(nil, "林小姐", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Title, "林小姐") {
		t.Fatalf("cjk hits = %+v", hits)
	}

	if _, err := v.Add("Use gofmt on save", []string{"go", "style"}, "use gofmt", 0.6); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Go exported names use PascalCase", []string{"go", "naming"}, "PascalCase", 0.6); err != nil {
		t.Fatal(err)
	}
	hits, err = v.Find([]string{"go"}, "PascalCase", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 1 || !strings.Contains(hits[0].Title, "PascalCase") {
		t.Fatalf("idf hits = %+v", hits)
	}
}

func TestListFilterAndNotes(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	if _, err := v.Add("Use gofmt", []string{"go", "style"}, "gofmt", 0.9); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Python function names must always be snake_case", []string{"python", "naming"}, "snake", 0.6); err != nil {
		t.Fatal(err)
	}
	items, err := v.ListFilter(ListFilter{Scope: []string{"go"}, MinConfidence: 0.85})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Use gofmt" {
		t.Fatalf("list filter = %+v", items)
	}
	items, err = v.ListFilter(ListFilter{Query: "snake_case"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Title, "snake_case") {
		t.Fatalf("list query = %+v", items)
	}
	res, err := v.Viz("", "notes", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.RulesCount != 2 {
		t.Fatalf("notes count = %+v", res)
	}
	note := filepath.Join(dir, "notes", items[0].ID+".md")
	body, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "[[") && !strings.Contains(string(body), "(none)") {
		t.Fatalf("note = %s", body)
	}
}

func TestDashboardRefreshAndOffline(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.Add("First rule for the map", []string{"go"}, "one", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Viz("", "html", false); err != nil {
		t.Fatal(err)
	}
	dash := filepath.Join(dir, "dashboard.html")
	if _, err := v.Add("Second rule for the map", []string{"go"}, "two", 0.6); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dash)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("unpkg.com")) {
		t.Fatal("dashboard still loads graph code from CDN")
	}
	if !bytes.Contains(body, []byte("Second rule for the map")) {
		t.Fatalf("dashboard not refreshed after add")
	}
	if !bytes.Contains(body, []byte("dandelionLayout")) {
		t.Fatal("embedded dandelion graph missing")
	}
	_ = added
}
