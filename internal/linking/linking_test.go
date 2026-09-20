package linking_test

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestEnrichChunkGetReferencedRules(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	added, err := v.AddWithSources("Rule A", []string{"demo"}, "text", 0.6, []imprint.DocRef{
		{Path: "docs/x.md", Heading: "Intro"},
	})
	if err != nil {
		t.Fatal(err)
	}
	chunk := index.Chunk{ID: "c1", Path: "docs/x.md", Heading: "Intro", Text: "hello", LineStart: 1, LineEnd: 2}
	out, err := linking.EnrichChunkGet(v, nil, chunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.ReferencedRules) != 1 || out.ReferencedRules[0].ID != added.ID {
		t.Fatalf("referenced = %+v", out.ReferencedRules)
	}
}

func TestEnrichFindDualQuery(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.AddRecord("Documentation framing", []string{"docs"}, "text", 0.85, nil, nil, nil, nil, "否定式堆砌")
	if err != nil {
		t.Fatal(err)
	}
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "c1", Path: "docs/writing.zh.md", Heading: "写作", Text: "切忌否定式堆砌的 disclaimer 段落", LineStart: 1, LineEnd: 3},
			{ID: "c2", Path: "docs/architecture.md", Heading: "Storage", Text: "documentation framing for retrieval", LineStart: 1, LineEnd: 3},
		},
	}
	out, hits, err := linking.EnrichFind(v, st, []string{"docs"}, "documentation framing", "否定式堆砌", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected rule hits")
	}
	if len(out.Documents) == 0 {
		t.Fatal("expected document hits from dual query")
	}
	seen := map[string]bool{}
	for _, d := range out.Documents {
		seen[d.ID] = true
	}
	if !seen["c1"] || !seen["c2"] {
		t.Fatalf("documents = %+v", out.Documents)
	}
}
