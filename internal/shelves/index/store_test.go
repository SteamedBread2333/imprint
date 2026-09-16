package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteSaveLoad(t *testing.T) {
	dir := t.TempDir()
	s := &Store{
		Fingerprint: "abc123",
		BuiltAt:     time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		FileCount:   2,
		Chunks: []Chunk{
			{ID: "c1", Path: "docs/a.md", Heading: "Intro", Text: "hello world", LineStart: 1, LineEnd: 3},
			{ID: "c2", Path: "docs/b.md", Heading: "API", Text: "plugin search", LineStart: 10, LineEnd: 12},
		},
	}
	if err := SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(DBPath(dir)); err != nil {
		t.Fatalf("index.db missing: %v", err)
	}
	got, err := LoadStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != s.Fingerprint || got.FileCount != 2 || len(got.Chunks) != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	c, ok := got.ChunkByID("c2")
	if !ok || c.Heading != "API" {
		t.Fatalf("chunk by id: %+v ok=%v", c, ok)
	}
	hits := got.SearchStore("plugin", 5)
	if len(hits) == 0 || hits[0].ID != "c2" {
		t.Fatalf("search: %+v", hits)
	}
}

func TestMigrateLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "index.json")
	raw := `{"fingerprint":"legacy","built_at":"2026-01-01T00:00:00Z","file_count":1,"chunks":[{"id":"x","path":"docs/x.md","heading":"H","text":"legacy text","line_start":1,"line_end":2}]}`
	if err := os.WriteFile(legacy, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOrEmpty(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != "legacy" || len(got.Chunks) != 1 {
		t.Fatalf("migrate: %+v", got)
	}
	if _, err := os.Stat(DBPath(dir)); err != nil {
		t.Fatal("sqlite not created after migrate")
	}
}
