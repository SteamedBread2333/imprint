package index

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
		t.Fatalf("shelves.db missing: %v", err)
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

func TestConcurrentSaveStoreProducesCompleteSnapshot(t *testing.T) {
	dir := t.TempDir()
	const writers = 8
	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id := fmt.Sprintf("c%d", i)
			errs <- SaveStore(dir, &Store{
				Fingerprint: fmt.Sprintf("fp-%d", i),
				BuiltAt:     time.Date(2026, 9, 21, 12, 0, i, 0, time.UTC),
				FileCount:   1,
				Chunks: []Chunk{{
					ID: id, Path: fmt.Sprintf("docs/%d.md", i), Heading: "H",
					Text: "complete snapshot", LineStart: 1, LineEnd: 1,
				}},
			})
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent SaveStore: %v", err)
		}
	}
	if t.Failed() {
		return
	}
	got, err := LoadStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileCount != 1 || len(got.Chunks) != 1 || got.Chunks[0].Text != "complete snapshot" {
		t.Fatalf("incomplete final snapshot: %+v", got)
	}
}
