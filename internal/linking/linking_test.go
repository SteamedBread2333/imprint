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
